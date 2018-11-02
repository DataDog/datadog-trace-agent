package event

import "github.com/DataDog/datadog-trace-agent/model"

// Processor is responsible for all the logic surrounding extraction and sampling of APM events from processed traces.
type Processor struct {
	extractors []Extractor
	samplers   []Sampler
}

// NewProcessor returns a new instance of Processor configured with the provided extractors and samplers.
//
// Extractors will look at each span in the trace and decide whether it should be converted to an APM event or not. They
// will be tried in the provided order, with the first non-neutral decision being the one applied.
//
// Samplers are applied, in order, to all extracted APM events, no matter which extractor extracted them. An event is
// sampled if all samplers decided to sample it. As soon as a sampler decided to drop an event, it is dropped.
// Samplers are assumed to be independent of each other so that the probability that an event survives sampling is
// given by P(sampledA ^ sampledB ^ ... ^ sampledZ) = P(sampledA) * P(sampledB) * ... * P(sampledZ).
func NewProcessor(extractors []Extractor, samplers []Sampler) *Processor {
	return &Processor{
		extractors: extractors,
		samplers:   samplers,
	}
}

// Start starts all underlying samplers.
func (p *Processor) Start() {
	for _, sampler := range p.samplers {
		sampler.Start()
	}
}

// Stop stops all underlying samplers.
func (p *Processor) Stop() {
	for _, sampler := range p.samplers {
		sampler.Stop()
	}
}

// Process takes a processed trace, extracts events from it, submits them to the samplers and returns a collection of
// sampled events along with the total count of extracted events.
func (p *Processor) Process(t model.ProcessedTrace) (events []*model.APMEvent, numExtracted int) {
	// Short-circuit if there are no extractors configured
	if len(p.extractors) == 0 {
		return nil, 0
	}

	priority, hasPriority := t.GetSamplingPriority()

	// If priority is not set on a trace, assume priority 0
	if !hasPriority {
		priority = 0
	}

	for _, span := range t.WeightedTrace {
		var extractedEvent *model.APMEvent

		// Loop through extractors until we find one that attempted to extract an event.
		for _, extractor := range p.extractors {
			extract, rate := extractor.Extract(span, priority)

			// If the extractor didn't know what to do with this span, try the next one
			if rate == UnknownRate {
				continue
			}

			if extract {
				extractedEvent = &model.APMEvent{Span: span.Span, TraceSampled: t.Sampled}
				extractedEvent.SetExtractionSampleRate(rate)
			}

			// If this extractor applied a valid sampling rate then that means it processed this span so don't try the
			// next ones.
			break
		}

		// If we didn't find any event in this span, try the next span
		if extractedEvent == nil {
			continue
		}

		numExtracted++

		// Otherwise, apply event samplers to the extracted event
		sampleDecision := true
		sampleRate := 1.0

		for _, sampler := range p.samplers {
			sampled, rate := sampler.Sample(extractedEvent)

			// If the sampler didn't know what do do with the event, try the next one.
			if rate == UnknownRate {
				continue
			}

			// If one of the samplers didn't sample this event, don't sample it.
			if !sampled {
				sampleDecision = false
				break
			}

			// Total sample rate is the multiplication of each sampler's rate as we assume samplers are independent
			sampleRate *= rate
		}

		// If the event survived global sampling then add it to the results.
		if sampleDecision {
			// And add the total sample rate to it
			extractedEvent.SetEventSamplerSampleRate(sampleRate)
			events = append(events, extractedEvent)
		}
	}

	return
}
