package event

import "github.com/DataDog/datadog-trace-agent/model"

// Processor is responsible for all the logic surrounding extraction and sampling of APM events from processed traces.
type Processor struct {
	extractors []Extractor
	samplers   []Sampler
}

// ProcessorParams is a struct containing extra parameters for event processing.
type ProcessorParams struct {
	ClientSampleRate float64
	PreSampleRate    float64
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

// Start starts the processor.
func (p *Processor) Start() {
	for _, sampler := range p.samplers {
		sampler.Start()
	}
}

// Stop stops the processor.
func (p *Processor) Stop() {
	for _, sampler := range p.samplers {
		sampler.Stop()
	}
}

// Process takes a processed trace, extracts events from it, submits them to the samplers and returns a collection of
// sampled events along with the total count of extracted events. Process also takes a ProcessorParams struct from
// which trace rates are extracted and set on every sampled event. An extraction callback can also be set which will
// be called for every extracted event.
func (p *Processor) Process(t model.ProcessedTrace, params ProcessorParams) (events []*model.APMEvent, numExtracted int64) {
	if len(p.extractors) == 0 {
		return
	}

	priority, hasPriority := t.GetSamplingPriority()

	if !hasPriority {
		priority = model.PriorityNone
	}

	for _, span := range t.WeightedTrace {
		var event *model.APMEvent

		for _, extractor := range p.extractors {
			extract, rate := extractor.Extract(span, priority)

			if rate == RateNone {
				// If the extractor did not make any extraction decision, try the next one
				continue
			}

			if extract {
				event = &model.APMEvent{Span: span.Span, TraceSampled: t.Sampled}
				event.SetExtractionSampleRate(rate)
			}

			// If this extractor applied a valid sampling rate then that means it processed this span so don't try the
			// next ones.
			break
		}

		if event == nil {
			// If we didn't find any event in this span, try the next span
			continue
		}

		numExtracted++

		// Otherwise, apply event samplers to the extracted event
		eventSampled := true
		sampleRate := 1.0

		for _, sampler := range p.samplers {
			sampled, rate := sampler.Sample(event)

			if rate == RateNone {
				// If the sampler didn't know what do do with the event, try the next one.
				continue
			}

			if !sampled {
				// If one of the samplers didn't sample this event, don't sample it.
				eventSampled = false
				break
			}

			// Total sample rate is the multiplication of each sampler's rate as we assume samplers are independent
			sampleRate *= rate
		}

		if eventSampled {
			events = append(events, event)

			// Add the total sample rate to it
			event.SetEventSampleRate(sampleRate)
			// Also add any trace sample rates to sampled events
			event.SetClientTraceSampleRate(params.ClientSampleRate)
			event.SetPreSampleRate(params.PreSampleRate)
		}
	}

	return
}
