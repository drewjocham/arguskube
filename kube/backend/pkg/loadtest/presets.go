package loadtest

import "time"

// Preset is a named, editable starting point for a RunSpec. The
// frontend shows the description + "best for" guidance next to each
// option; once the user picks one, the resolved RunSpec fields are
// editable in the form before they hit Start.
//
// The five locked presets correspond to the audit-approved list:
// Smoke, Cold-start drain, Soak baseline, Linear ramp-up, Spike.
//
// Adding a sixth is one entry in PresetList — the frontend reflects
// the list verbatim, so the engine is the single source of truth.
type Preset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	WhenToUse   string `json:"whenToUse"`
	Spec        RunSpec `json:"spec"`
}

// PresetList returns the five locked presets. New instances each call
// so callers can mutate the returned Spec without contaminating other
// callers — important because the frontend uses these as form
// defaults that get edited in place.
func PresetList() []Preset {
	return []Preset{
		{
			ID:          "smoke",
			Name:        "Smoke",
			Description: "1,000 messages at constant 50/s; no Kubernetes scaling.",
			WhenToUse:   "Verify your broker connection and payload shape before a real run. Should complete in ~20 seconds.",
			Spec: RunSpec{
				Count: 1000,
				Ramp: Ramp{
					Kind:     RampConstant,
					Rate:     50,
					Duration: 30 * time.Second,
				},
				Workers: 10,
			},
		},
		{
			ID:          "cold-start",
			Name:        "Cold-start drain",
			Description: "Pre-publish 100,000 messages while consumer is scaled to 0, then scale to min 2 and time the drain.",
			WhenToUse:   "Use to size minReplicas and to measure end-to-end cold-start: how long until the queue empties after scale-up?",
			Spec: RunSpec{
				Count: 100_000,
				Ramp: Ramp{
					Kind: RampConstant,
					Rate: 500,
				},
				Workers: 100,
				Scale: ScalePlan{
					PreScaleToZero:   true,
					MinReplicas:      2,
					PreScaleTimeout:  2 * time.Minute,
					PostScaleTimeout: 10 * time.Minute,
				},
			},
		},
		{
			ID:          "soak",
			Name:        "Soak baseline",
			Description: "100,000 messages at constant 100/s over ~15 minutes; consumer running at steady state.",
			WhenToUse:   "Establishes a stable throughput baseline. Catches memory leaks and slow lag accumulation that don't show up in shorter tests.",
			Spec: RunSpec{
				Count: 100_000,
				Ramp: Ramp{
					Kind:     RampConstant,
					Rate:     100,
					Duration: 17 * time.Minute,
				},
				Workers: 50,
			},
		},
		{
			ID:          "linear-ramp",
			Name:        "Linear ramp-up",
			Description: "Rate climbs 1/s → 500/s over 10 minutes, then holds 500/s for 5 minutes.",
			WhenToUse:   "Find the throughput knee where ack latency climbs steeply — the rate at which your consumer can't keep up.",
			Spec: RunSpec{
				Count: 200_000,
				Ramp: Ramp{
					Kind:     RampLinear,
					Rate:     1,
					RampTo:   500,
					Duration: 10 * time.Minute,
				},
				Workers: 100,
			},
		},
		{
			ID:          "spike",
			Name:        "Spike",
			Description: "Six bursts of 10,000 messages each, with 30-second idle gaps between bursts.",
			WhenToUse:   "Test consumer autoscaler responsiveness to traffic spikes. Watch how quickly replicas come up and how high P99 climbs during each burst.",
			Spec: RunSpec{
				Count: 60_000,
				Ramp: Ramp{
					Kind:       RampSpike,
					SpikeCount: 6,
					SpikeSize:  10_000,
					SpikeIdle:  30 * time.Second,
				},
				Workers: 200,
			},
		},
	}
}
