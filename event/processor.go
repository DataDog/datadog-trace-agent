package event

import (
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/sampler"
)

// Processor is responsible for all the logic surrounding extraction and sampling of APM events from processed traces.
type Processor struct {
	extractors    []Extractor
	maxEPSSampler eventSampler
}

// NewProcessor returns a new instance of Processor configured with the provided extractors and max eps limitation.
//
// Extractors will look at each span in the trace and decide whether it should be converted to an APM event or not. They
// will be tried in the provided order, with the first one returning an event stopping the chain.
//
// All extracted APM events are then submitted to sampling. This sampling is 2-fold:
// * A first sampling step is done based on the extraction sampling rate returned by an Extractor. If an Extractor
//   returns an event accompanied with a 0.1 extraction rate, then there's a 90% chance that this event will get
//   discarded.
// * A max events per second maxEPSSampler is applied to all non-PriorityUserKeep events that survived the first step
//   and will ensure that, in average, the total rate of events returned by the processor is not bigger than maxEPS.
func NewProcessor(extractors []Extractor, maxEPS float64) *Processor {
	return newProcessor(extractors, newMaxEPSSampler(maxEPS))
}

func newProcessor(extractors []Extractor, maxEPSSampler eventSampler) *Processor {
	return &Processor{
		extractors:    extractors,
		maxEPSSampler: maxEPSSampler,
	}
}

// Start starts the processor.
func (p *Processor) Start() {
	p.maxEPSSampler.Start()
}

// Stop stops the processor.
func (p *Processor) Stop() {
	p.maxEPSSampler.Stop()
}

// Process takes a processed trace, extracts events from it and samples them, returning a collection of
// sampled events along with the total count of extracted events.
func (p *Processor) Process(t model.ProcessedTrace) (events []*model.Event, numExtracted int64) {
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
		event, extractionRate, ok := p.extract(span, priority)
		if !ok {
			continue
		}

		sampled := p.extractionSample(event, extractionRate)
		if !sampled {
			continue
		}
		numExtracted++

		sampled, epsRate := p.maxEPSSample(event)
		if !sampled {
			continue
		}

		// This event got sampled, so add it to results
		events = append(events, event)
		// And set whatever rates had been set on the trace initially
		event.SetClientTraceSampleRate(clientSampleRate)
		event.SetPreSampleRate(preSampleRate)
		// As well as the rates of sampling done during this processing
		event.SetExtractionSampleRate(extractionRate)
		event.SetMaxEPSSampleRate(epsRate)
		if hasPriority {
			// Make sure to set the sampling priority on the event span so that this gets propagated to the backend.
			event.Span.SetSamplingPriority(priority)
		}
	}

	return
}

func (p *Processor) extract(span *model.WeightedSpan, priority model.SamplingPriority) (*model.Event, float64, bool) {
	for _, extractor := range p.extractors {
		if event, rate, ok := extractor.Extract(span, priority); ok {
			return event, rate, ok
		}
	}
	return nil, 0, false
}

func (p *Processor) extractionSample(event *model.Event, extractionRate float64) bool {
	return sampler.SampleByRate(event.Span.TraceID, extractionRate)
}

func (p *Processor) maxEPSSample(event *model.Event) (sampled bool, rate float64) {
	if event.Priority == model.PriorityUserKeep {
		return true, 1
	}
	return p.maxEPSSampler.Sample(event)
}

type eventSampler interface {
	Start()
	Sample(event *model.Event) (sampled bool, rate float64)
	Stop()
}
