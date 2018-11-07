package event

import "github.com/DataDog/datadog-trace-agent/model"

// Processor is responsible for all the logic surrounding extraction and sampling of APM events from processed traces.
type Processor struct {
	extractors []Extractor
	sampler    Sampler
}

// NewProcessor returns a new instance of Processor configured with the provided extractors and sampler.
//
// Extractors will look at each span in the trace and decide whether it should be converted to an APM event or not. They
// will be tried in the provided order, with the first non-neutral decision being the one applied.
//
// All extracted APM events are then submitted to the specified sampler (if any), no matter which extractor extracted
// them. Only those events that survived this sampling step are returned. If sampler is nil, all extracted events are
// assumed to be sampled and shall be returned.
func NewProcessor(extractors []Extractor, sampler Sampler) *Processor {
	return &Processor{
		extractors: extractors,
		sampler:    sampler,
	}
}

// Start starts the processor.
func (p *Processor) Start() {
	if p.sampler != nil {
		p.sampler.Start()
	}
}

// Stop stops the processor.
func (p *Processor) Stop() {
	if p.sampler != nil {
		p.sampler.Stop()
	}
}

// Process takes a processed trace, extracts events from it, submits them to the samplers and returns a collection of
// sampled events along with the total count of extracted events. Process also takes a ProcessorParams struct from
// which trace rates are extracted and set on every sampled event. An extraction callback can also be set which will
// be called for every extracted event.
func (p *Processor) Process(t model.ProcessedTrace) (events []*model.APMEvent, numExtracted int64) {
	if len(p.extractors) == 0 {
		return
	}

	priority, hasPriority := t.GetSamplingPriority()

	if !hasPriority {
		priority = model.PriorityNone
	}

	clientSampleRate := t.Root.GetClientSampleRate()
	preSampleRate := t.Root.GetPreSampleRate()

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

			// If this extractor applied a valid sampling Rate then that means it processed this span so don't try the
			// next ones.
			break
		}

		if event == nil {
			// If we didn't find any event in this span, try the next span
			continue
		}

		numExtracted++

		if p.sampler != nil {
			if sampled, _ := p.sampler.Sample(event); !sampled {
				// If we didn't sample this event, try the next span
				continue
			}
		}

		// Otherwise, this event got sampled, so add it to results
		events = append(events, event)
		// And set whatever rates had been set on the trace initially
		event.SetClientTraceSampleRate(clientSampleRate)
		event.SetPreSampleRate(preSampleRate)
	}

	return
}
