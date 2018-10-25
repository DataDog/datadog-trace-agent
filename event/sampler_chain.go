package event

import "github.com/DataDog/datadog-trace-agent/model"

// samplerChain is an APMEvent sampler that submits an event to a sequence of samplers, returning as soon as one of
// the samplers makes a decision.
type samplerChain struct {
	samplers []Sampler
	callback func(decision SamplingDecision)
}

// NewSamplerChain creates a new sampler chain from the provided samplers and with the specified decision callback.
func NewSamplerChain(samplers []Sampler, callback func(decision SamplingDecision)) Sampler {
	return &samplerChain{
		samplers: samplers,
		callback: callback,
	}
}

// Sample returns the first sampling decision that is not DecisionNone from calls to the underlying samplers, in order.
// If none of the samplers returns a decision different from DecisionNone, then DecisionNone is also returned here.
func (sc *samplerChain) Sample(event *model.APMEvent) SamplingDecision {
	result := DecisionNone

	for _, sampler := range sc.samplers {
		result = sampler.Sample(event)

		if result != DecisionNone {
			break
		}
	}

	if sc.callback != nil {
		sc.callback(result)
	}

	return result
}
