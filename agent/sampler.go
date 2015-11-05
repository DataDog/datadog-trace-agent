package main

import (
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

// FIXME[leo]: do not hardcode it maybe?
var DefaultQuantiles = [...]float64{0, 0.25, 0.5, 0.75, 0.90, 0.95, 0.99, 1}

// Sampler chooses wich spans to write to the API
type Sampler struct {
	inSpans chan model.Span
	inStats chan model.StatsBucket  // Trigger the flush of the sampler when stats are received
	out     chan model.AgentPayload // Output the stats + samples

	// exit channels used for synchronisation and sending stop signals
	exit      chan struct{}
	exitGroup *sync.WaitGroup

	TraceIDBySpanID map[uint64]uint64
	SpansByTraceID  map[uint64][]model.Span
	mu              sync.Mutex
}

// NewSampler creates a new empty sampler
func NewSampler(inSpans chan model.Span, inStats chan model.StatsBucket, exit chan struct{}, exitGroup *sync.WaitGroup) *Sampler {

	return &Sampler{
		inSpans: inSpans,
		inStats: inStats,
		out:     make(chan model.AgentPayload),

		exit:      exit,
		exitGroup: exitGroup,

		TraceIDBySpanID: map[uint64]uint64{},
		SpansByTraceID:  map[uint64][]model.Span{},
	}
}

// Start runs the writer by consuming spans in a buffer and periodically
// flushing to the API
func (s *Sampler) Start() {
	s.exitGroup.Add(1)
	go s.run()

	log.Info("Sampler started")
}

// We rely on the concentrator ticker to flush periodically traces "aligning" on the buckets
// (it's not perfect, but we don't really care, traces of this stats bucket may arrive in the next flush)
func (s *Sampler) run() {
	for {
		select {
		case span := <-s.inSpans:
			s.AddSpan(span)
		case bucket := <-s.inStats:
			log.Info("Received a bucket from concentrator, initiating a sampling+flush")
			s.out <- s.FlushPayload(bucket)
		case <-s.exit:
			log.Info("Sampler exiting")
			s.exitGroup.Done()
			return
		}
	}
}

// AddSpan adds a span to the sampler internal momory
func (s *Sampler) AddSpan(span model.Span) {
	s.mu.Lock()
	s.TraceIDBySpanID[span.SpanID] = span.TraceID

	spans, ok := s.SpansByTraceID[span.TraceID]
	if !ok {
		spans = []model.Span{span}
	} else {
		spans = append(spans, span)
	}
	s.SpansByTraceID[span.TraceID] = spans
	s.mu.Unlock()
}

func (s *Sampler) FlushPayload(sb model.StatsBucket) model.AgentPayload {
	// Freeze sampler state
	s.mu.Lock()
	traceIDBySpanID := s.TraceIDBySpanID
	spansByTraceID := s.SpansByTraceID
	s.TraceIDBySpanID = map[uint64]uint64{}
	s.SpansByTraceID = map[uint64][]model.Span{}
	s.mu.Unlock()

	samples := s.GetSamples(traceIDBySpanID, spansByTraceID, sb)
	return model.AgentPayload{
		Stats: sb,
		Spans: samples,
	}
}

// GetSamples returns a list of representative spans to write
func (s *Sampler) GetSamples(traceIDBySpanID map[uint64]uint64, spansByTraceID map[uint64][]model.Span, sb model.StatsBucket) []model.Span {
	startTime := time.Now()
	spanIDs := make([]uint64, len(sb.Distributions)*len(DefaultQuantiles))

	// Look at the stats to find representative spans
	for _, d := range sb.Distributions {
		for _, q := range DefaultQuantiles {
			_, sIDs := d.Summary.Quantile(q)

			if len(sIDs) > 0 { // TODO: not sure this condition is required
				spanIDs = append(spanIDs, sIDs[0])
			}
		}
	}

	// Then find the trace IDs thanks to a spanID -> traceID map
	traceIDSet := make(map[uint64]struct{})
	var token struct{}
	for _, spanID := range spanIDs {
		// spanIDs is pre-allocated, so it may contain zeros
		if spanID != 0 {
			traceID, ok := traceIDBySpanID[spanID]
			if !ok {
				log.Errorf("SpanID reported by Quantiles not available in Sampler, SpanID=%d", spanID)
			} else {
				traceIDSet[traceID] = token
			}
		}
	}

	// Then get the traces (ie. set of spans) thanks to a traceID -> []spanID map
	spans := []model.Span{}
	for traceID := range traceIDSet {
		spans = append(spans, spansByTraceID[traceID]...)
	}

	Statsd.Count("trace_agent.sampler.trace.total", int64(len(spansByTraceID)), nil, 1)
	Statsd.Count("trace_agent.sampler.trace.kept", int64(len(traceIDSet)), nil, 1)
	Statsd.Count("trace_agent.sampler.span.total", int64(len(traceIDBySpanID)), nil, 1)
	Statsd.Count("trace_agent.sampler.span.kept", int64(len(spans)), nil, 1)

	execTime := time.Since(startTime)
	log.Infof("Sampled %d traces out of %d, %d spans out of %d, in %s",
		len(traceIDSet), len(spansByTraceID), len(spans), len(traceIDBySpanID), execTime)

	Statsd.Gauge("trace_agent.sampler.sample_duration", execTime.Seconds(), nil, 1)

	return spans
}
