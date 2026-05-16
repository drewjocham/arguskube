package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// waitFor is a small polling helper local to this test file. The
// project-wide testutil.WaitFor exists on a parallel review PR but
// hasn't merged yet, so we inline the same shape here rather than
// stack this PR on that one. Once both land, this can collapse to a
// single import.
func waitFor(t *testing.T, timeout, interval time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("waitFor: %s (timeout %s)", msg, timeout)
		}
		time.Sleep(interval)
	}
}

func TestRegisterRejectsMissingFields(t *testing.T) {
	t.Parallel()
	s := New(discardLogger())
	cases := []struct {
		name string
		job  Job
	}{
		{"no ID", Job{Schedule: IntervalSchedule{Interval: time.Second}, Handler: func(context.Context) error { return nil }}},
		{"no Schedule", Job{ID: "x", Handler: func(context.Context) error { return nil }}},
		{"no Handler", Job{ID: "x", Schedule: IntervalSchedule{Interval: time.Second}}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if err := s.Register(c.job); err == nil {
				t.Errorf("Register %s should fail", c.name)
			}
		})
	}
}

func TestRegisterDuplicateRejected(t *testing.T) {
	t.Parallel()
	s := New(discardLogger())
	j := Job{
		ID:       "dup",
		Schedule: IntervalSchedule{Interval: time.Second},
		Handler:  func(context.Context) error { return nil },
	}
	if err := s.Register(j); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := s.Register(j); !errors.Is(err, ErrAlreadyRegistered) {
		t.Errorf("expected ErrAlreadyRegistered; got %v", err)
	}
}

func TestSchedulerFiresJob(t *testing.T) {
	t.Parallel()
	s := New(discardLogger())
	s.SetTick(2 * time.Millisecond)

	var calls atomic.Int64
	if err := s.Register(Job{
		ID:       "fast",
		Schedule: IntervalSchedule{Interval: 5 * time.Millisecond},
		Handler:  func(context.Context) error { calls.Add(1); return nil },
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = s.Start(ctx) }()

	waitFor(t, time.Second, 2*time.Millisecond,
		func() bool { return calls.Load() >= 3 },
		"job should have run at least 3 times within 1s")
}

func TestSchedulerRetriesOnFailure(t *testing.T) {
	t.Parallel()
	s := New(discardLogger())
	s.SetTick(2 * time.Millisecond)

	var attempts atomic.Int64
	if err := s.Register(Job{
		ID:       "retry",
		Schedule: IntervalSchedule{Interval: time.Hour}, // far future; we test the retry, not the schedule
		Handler: func(context.Context) error {
			n := attempts.Add(1)
			if n < 3 {
				return errors.New("transient")
			}
			return nil
		},
		Retry: RetryPolicy{MaxAttempts: 5, InitialDelay: 1 * time.Millisecond, Multiplier: 1.0},
	}); err != nil {
		t.Fatal(err)
	}

	// Force the next-run to "now" so the dispatcher picks it up promptly.
	s.mu.Lock()
	s.jobs["retry"].next = time.Now()
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = s.Start(ctx) }()

	waitFor(t, time.Second, 5*time.Millisecond,
		func() bool { return attempts.Load() >= 3 },
		"job should have been retried until attempt #3 succeeded")
}

func TestSchedulerSendsToDeadLetter(t *testing.T) {
	t.Parallel()
	s := New(discardLogger())
	s.SetTick(2 * time.Millisecond)

	failing := errors.New("permanent")
	if err := s.Register(Job{
		ID:       "dead",
		Schedule: IntervalSchedule{Interval: time.Hour},
		Handler:  func(context.Context) error { return failing },
		Retry:    RetryPolicy{MaxAttempts: 2, InitialDelay: 1 * time.Millisecond},
	}); err != nil {
		t.Fatal(err)
	}

	s.mu.Lock()
	s.jobs["dead"].next = time.Now()
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = s.Start(ctx) }()

	waitFor(t, time.Second, 5*time.Millisecond,
		func() bool { return len(s.DeadLetter()) >= 1 },
		"job should land in the dead-letter queue after exhausting retries")

	dl := s.DeadLetter()
	if !errors.Is(dl[0].Err, failing) {
		t.Errorf("dead-letter error = %v, want %v", dl[0].Err, failing)
	}
}

func TestSchedulerStartTwiceErrors(t *testing.T) {
	t.Parallel()
	s := New(discardLogger())
	s.SetTick(2 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = s.Start(ctx) }()

	// Give Start a tick to mark itself started.
	time.Sleep(10 * time.Millisecond)
	if err := s.Start(context.Background()); err == nil {
		t.Error("second Start should fail")
	}
}

func TestSchedulerStopOnContextCancel(t *testing.T) {
	t.Parallel()
	s := New(discardLogger())
	s.SetTick(2 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Start(ctx) }()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled; got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduler did not exit on ctx cancel")
	}
}

func TestIntervalScheduleNext(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	sched := IntervalSchedule{Interval: 5 * time.Minute}
	got := sched.Next(now)
	want := now.Add(5 * time.Minute)
	if !got.Equal(want) {
		t.Errorf("Next = %v, want %v", got, want)
	}
}

func TestIntervalScheduleHonorsFirst(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	first := now.Add(30 * time.Second)
	sched := IntervalSchedule{Interval: 5 * time.Minute, First: first}
	if got := sched.Next(now); !got.Equal(first) {
		t.Errorf("Next on a fresh schedule should honor First; got %v want %v", got, first)
	}
	// After First has passed, behaves as normal interval.
	later := first.Add(10 * time.Second)
	if got := sched.Next(later); !got.Equal(later.Add(5 * time.Minute)) {
		t.Errorf("Next past First = %v, want %v", got, later.Add(5*time.Minute))
	}
}

func TestBackoffExponential(t *testing.T) {
	t.Parallel()
	p := RetryPolicy{InitialDelay: 100 * time.Millisecond, MaxDelay: time.Second, Multiplier: 2.0}
	got1 := backoff(p, 1)
	got2 := backoff(p, 2)
	got3 := backoff(p, 3)
	got4 := backoff(p, 10) // capped
	if got1 != 100*time.Millisecond {
		t.Errorf("attempt 1: %v, want 100ms", got1)
	}
	if got2 != 200*time.Millisecond {
		t.Errorf("attempt 2: %v, want 200ms", got2)
	}
	if got3 != 400*time.Millisecond {
		t.Errorf("attempt 3: %v, want 400ms", got3)
	}
	if got4 != time.Second {
		t.Errorf("attempt 10 should hit the 1s cap; got %v", got4)
	}
}
