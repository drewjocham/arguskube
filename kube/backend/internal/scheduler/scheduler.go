// Package scheduler is an in-process periodic-job runner with retry
// + dead-letter semantics. It's the first cut at CR P2.13's
// "Cron/scheduler engine" — interval-based for now (Go duration
// strings), with a clean seam so a future PR can add real cron-
// expression parsing without touching the Run loop.
//
// What's deliberately NOT here yet:
//   * Persistence — jobs/runs live in memory; lose them on restart.
//     Persistent state lands when we wire this to the existing
//     workflows store, but that's a follow-up so reviewers can pick
//     apart the scheduling semantics first.
//   * Cron expressions — robfig/cron is a 30-line add, but parsing
//     adds surface area. The Schedule interface is in place to swap
//     IntervalSchedule for a CronSchedule without changes elsewhere.
//   * Webhook triggers (#3 in the CR's wish-list) — those are a
//     dispatch mechanism, not a schedule; goes on top of this.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Handler is the job's work function. It receives a context that
// honors both Scheduler shutdown and the job's per-run timeout.
// Returning a non-nil error counts toward the retry budget.
type Handler func(ctx context.Context) error

// Schedule decides when a job should fire next.
type Schedule interface {
	// Next returns the time at or after `from` when the job should run.
	Next(from time.Time) time.Time
}

// IntervalSchedule fires every Interval. The first run happens
// Interval after the scheduler starts; pass First to seed a specific
// initial fire time (zero means "Interval from now").
type IntervalSchedule struct {
	Interval time.Duration
	First    time.Time // optional; zero means "start + Interval"
}

// Next on IntervalSchedule returns from + Interval, or First on the
// very first call when First is non-zero.
func (s IntervalSchedule) Next(from time.Time) time.Time {
	if !s.First.IsZero() && from.Before(s.First) {
		return s.First
	}
	return from.Add(s.Interval)
}

// RetryPolicy describes the retry behavior for a failed job run.
// Zero values are fine: MaxAttempts 0 means "no retries", InitialDelay
// 0 means "retry immediately", and Multiplier 0 means "fixed delay".
type RetryPolicy struct {
	MaxAttempts  int           // total attempts INCLUDING the first; 1 = no retries
	InitialDelay time.Duration // delay before the second attempt
	MaxDelay     time.Duration // cap on the backoff
	Multiplier   float64       // 2.0 = exponential; 1.0 = fixed
}

// Job is the unit of work the scheduler dispatches.
type Job struct {
	ID       string
	Name     string
	Schedule Schedule
	Handler  Handler
	Retry    RetryPolicy
	Timeout  time.Duration // 0 = no per-run deadline
}

// runState is the live, mutable view of a single Job. Held under
// Scheduler.mu.
type runState struct {
	job      Job
	next     time.Time
	attempts int
	lastErr  error
	lastRun  time.Time
}

// JobResult captures the outcome of one attempt. Stored on the
// scheduler's history queue so the UI can surface it without
// inverting the dispatch loop.
type JobResult struct {
	JobID    string
	Started  time.Time
	Finished time.Time
	Attempt  int
	Err      error
}

// Scheduler runs registered jobs on their schedules. Construct one
// per process; calling Start more than once is an error.
type Scheduler struct {
	logger *slog.Logger

	mu      sync.Mutex
	jobs    map[string]*runState
	history []JobResult // bounded ring
	dead    []JobResult // jobs that exhausted Retry.MaxAttempts

	started atomic.Bool

	// tick is the dispatch granularity. Defaults to 1s; tests override
	// to 1ms so they don't sleep a second per iteration.
	tick time.Duration

	// historyMax caps the rolling history. Defaults to 200.
	historyMax int
}

// New constructs a Scheduler. Pass nil logger for slog.Default().
func New(logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		logger:     logger.With("component", "scheduler"),
		jobs:       make(map[string]*runState),
		tick:       time.Second,
		historyMax: 200,
	}
}

// SetTick overrides the dispatch granularity. Intended for tests so
// they don't wait a full second per check. Must be called before
// Start.
func (s *Scheduler) SetTick(d time.Duration) {
	if d <= 0 {
		d = time.Second
	}
	s.tick = d
}

// ErrAlreadyRegistered is returned when Register is called twice for
// the same Job.ID.
var ErrAlreadyRegistered = errors.New("scheduler: job already registered")

// Register adds a job. Safe to call before or after Start. The first
// schedule fire is computed against the registration time.
func (s *Scheduler) Register(j Job) error {
	if j.ID == "" {
		return errors.New("scheduler: job ID is required")
	}
	if j.Schedule == nil {
		return errors.New("scheduler: job Schedule is required")
	}
	if j.Handler == nil {
		return errors.New("scheduler: job Handler is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[j.ID]; ok {
		return ErrAlreadyRegistered
	}
	s.jobs[j.ID] = &runState{
		job:  j,
		next: j.Schedule.Next(time.Now()),
	}
	return nil
}

// Unregister removes a job. Idempotent.
func (s *Scheduler) Unregister(id string) {
	s.mu.Lock()
	delete(s.jobs, id)
	s.mu.Unlock()
}

// Start blocks running the dispatch loop until ctx is cancelled.
// Returns the ctx error on shutdown.
func (s *Scheduler) Start(ctx context.Context) error {
	if !s.started.CompareAndSwap(false, true) {
		return errors.New("scheduler: already started")
	}

	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case now := <-ticker.C:
			s.dispatchDue(ctx, now)
		}
	}
}

// dispatchDue fires every job whose next-run is <= now. Runs each
// job in its own goroutine so a slow handler doesn't block siblings.
func (s *Scheduler) dispatchDue(ctx context.Context, now time.Time) {
	s.mu.Lock()
	due := make([]*runState, 0, len(s.jobs))
	for _, st := range s.jobs {
		if !st.next.After(now) {
			due = append(due, st)
			// Optimistically schedule the next run *now*, before we
			// release the lock. If the handler fails and a retry is
			// granted, the retry path overwrites this.
			st.next = st.job.Schedule.Next(now)
		}
	}
	s.mu.Unlock()

	for _, st := range due {
		go s.runOne(ctx, st)
	}
}

func (s *Scheduler) runOne(parent context.Context, st *runState) {
	for {
		runCtx, cancel := s.contextFor(parent, st.job)
		started := time.Now()
		err := st.job.Handler(runCtx)
		cancel()
		finished := time.Now()

		s.mu.Lock()
		st.attempts++
		attempt := st.attempts
		st.lastRun = started
		st.lastErr = err
		s.appendHistoryLocked(JobResult{
			JobID: st.job.ID, Started: started, Finished: finished,
			Attempt: attempt, Err: err,
		})
		s.mu.Unlock()

		if err == nil {
			s.mu.Lock()
			st.attempts = 0
			s.mu.Unlock()
			s.logger.Debug("job ok", "id", st.job.ID, "attempt", attempt, "took", finished.Sub(started))
			return
		}

		if attempt >= maxAttempts(st.job.Retry) {
			s.mu.Lock()
			s.dead = append(s.dead, JobResult{
				JobID: st.job.ID, Started: started, Finished: finished,
				Attempt: attempt, Err: err,
			})
			st.attempts = 0
			s.mu.Unlock()
			s.logger.Warn("job exhausted retries → dead letter",
				"id", st.job.ID, "attempts", attempt, "lastErr", err)
			return
		}

		delay := backoff(st.job.Retry, attempt)
		s.logger.Warn("job failed, will retry",
			"id", st.job.ID, "attempt", attempt, "delay", delay, "err", err)

		select {
		case <-parent.Done():
			return
		case <-time.After(delay):
		}
	}
}

func (s *Scheduler) contextFor(parent context.Context, j Job) (context.Context, context.CancelFunc) {
	if j.Timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, j.Timeout)
}

// History returns a copy of the rolling result log, newest last.
func (s *Scheduler) History() []JobResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]JobResult, len(s.history))
	copy(out, s.history)
	return out
}

// DeadLetter returns a copy of jobs that have exhausted their retry
// budget since startup.
func (s *Scheduler) DeadLetter() []JobResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]JobResult, len(s.dead))
	copy(out, s.dead)
	return out
}

func (s *Scheduler) appendHistoryLocked(r JobResult) {
	s.history = append(s.history, r)
	if len(s.history) > s.historyMax {
		s.history = s.history[len(s.history)-s.historyMax:]
	}
}

func maxAttempts(p RetryPolicy) int {
	if p.MaxAttempts <= 0 {
		return 1
	}
	return p.MaxAttempts
}

func backoff(p RetryPolicy, attempt int) time.Duration {
	if p.InitialDelay <= 0 {
		return 0
	}
	d := float64(p.InitialDelay)
	m := p.Multiplier
	if m <= 0 {
		m = 1
	}
	for i := 1; i < attempt; i++ {
		d *= m
	}
	out := time.Duration(d)
	if p.MaxDelay > 0 && out > p.MaxDelay {
		out = p.MaxDelay
	}
	return out
}

// String returns a human-readable name for a Job (UI helper).
func (j Job) String() string {
	if j.Name != "" {
		return fmt.Sprintf("%s (%s)", j.Name, j.ID)
	}
	return j.ID
}
