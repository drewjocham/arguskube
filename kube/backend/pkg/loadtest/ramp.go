package loadtest

import (
	"math"
	"time"
)

// rampPlanner converts a Ramp definition into a stream of "publish
// allowed" timestamps relative to t=0. The engine consumes that stream
// by sleeping until each timestamp, then handing one message to a
// worker. Keeping the planner pure (no broker calls, no time.Now in
// hot path) means we can test the timing math directly.
//
// The choice between "compute all timestamps up front" (this approach)
// vs "step a rate-limiter token bucket" is a tradeoff:
//
//   - Compute-up-front is simple and exact; spike + step are easier
//     to express; tests can verify the full schedule.
//   - Token-bucket is more memory-efficient at huge counts and can
//     respond to live rate changes.
//
// We pick compute-up-front. At the target ceiling (1M messages) the
// schedule is ~16 MB at 16 bytes/timestamp — well within budget for a
// desktop process, and the simplicity is worth it for a v1.
type rampPlanner struct {
	ramp  Ramp
	count int
}

func newRampPlanner(r Ramp, count int) *rampPlanner {
	return &rampPlanner{ramp: r, count: count}
}

// schedule returns the per-message offsets from t=0. For spike, count
// is determined by the ramp itself (SpikeCount * SpikeSize), so the
// caller's count argument is ignored. For non-spike, the schedule is
// capped at count messages even if Duration allows more.
func (p *rampPlanner) schedule() []time.Duration {
	switch p.ramp.Kind {
	case RampConstant:
		return p.scheduleConstant()
	case RampLinear:
		return p.scheduleLinear()
	case RampStep:
		return p.scheduleStep()
	case RampSpike:
		return p.scheduleSpike()
	}
	return nil
}

func (p *rampPlanner) scheduleConstant() []time.Duration {
	if p.ramp.Rate <= 0 || p.count <= 0 {
		return nil
	}
	gap := time.Duration(float64(time.Second) / p.ramp.Rate)
	// Cap at Duration if set.
	n := p.count
	if p.ramp.Duration > 0 {
		maxByDur := int(p.ramp.Duration / gap)
		if maxByDur < n {
			n = maxByDur
		}
	}
	out := make([]time.Duration, n)
	for i := range out {
		out[i] = time.Duration(i) * gap
	}
	return out
}

// scheduleLinear ramps rate from Rate→RampTo over Duration. Rate at
// time t is Rate + (RampTo-Rate) * (t/Duration). We integrate to find
// when message i fires:
//
//	cumulative(t) = Rate*t + 0.5*(RampTo-Rate)*t²/Duration
//	cumulative(t) == i  -> solve for t (quadratic)
//
// The quadratic root we want is the positive one. When Rate == RampTo
// we degenerate to constant rate and skip the math.
func (p *rampPlanner) scheduleLinear() []time.Duration {
	if p.ramp.Rate <= 0 || p.ramp.RampTo <= 0 || p.ramp.Duration <= 0 {
		return nil
	}
	if p.ramp.Rate == p.ramp.RampTo {
		// Equivalent to constant; reuse that schedule.
		return (&rampPlanner{
			ramp:  Ramp{Kind: RampConstant, Rate: p.ramp.Rate, Duration: p.ramp.Duration},
			count: p.count,
		}).scheduleConstant()
	}
	a := (p.ramp.RampTo - p.ramp.Rate) / (2 * p.ramp.Duration.Seconds())
	b := p.ramp.Rate
	dur := p.ramp.Duration
	out := make([]time.Duration, 0, p.count)
	for i := 1; i <= p.count; i++ {
		// Solve a*t² + b*t - i = 0 for t.
		// t = (-b + sqrt(b² + 4ai)) / 2a
		t := (-b + math.Sqrt(b*b+4*a*float64(i))) / (2 * a)
		offset := time.Duration(t * float64(time.Second))
		if offset > dur {
			break
		}
		out = append(out, offset)
	}
	return out
}

// scheduleStep keeps a constant rate within each step window then
// bumps the rate at every StepEvery boundary.
func (p *rampPlanner) scheduleStep() []time.Duration {
	if p.ramp.Rate <= 0 || p.ramp.StepBy <= 0 || p.ramp.StepEvery <= 0 || p.count <= 0 {
		return nil
	}
	out := make([]time.Duration, 0, p.count)
	rate := p.ramp.Rate
	cursor := time.Duration(0)
	stepDeadline := p.ramp.StepEvery
	for len(out) < p.count {
		if p.ramp.Duration > 0 && cursor > p.ramp.Duration {
			break
		}
		out = append(out, cursor)
		gap := time.Duration(float64(time.Second) / rate)
		cursor += gap
		if cursor >= stepDeadline {
			rate += p.ramp.StepBy
			stepDeadline += p.ramp.StepEvery
		}
	}
	return out
}

// scheduleSpike fires SpikeCount bursts of SpikeSize messages instantly,
// with SpikeIdle gaps between them.
func (p *rampPlanner) scheduleSpike() []time.Duration {
	if p.ramp.SpikeCount <= 0 || p.ramp.SpikeSize <= 0 {
		return nil
	}
	out := make([]time.Duration, 0, p.ramp.SpikeCount*p.ramp.SpikeSize)
	cursor := time.Duration(0)
	for b := 0; b < p.ramp.SpikeCount; b++ {
		for m := 0; m < p.ramp.SpikeSize; m++ {
			out = append(out, cursor)
		}
		cursor += p.ramp.SpikeIdle
	}
	return out
}
