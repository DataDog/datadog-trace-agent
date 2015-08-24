package main

import (
	"math/rand"

	"github.com/DataDog/raclette/model"
)

// generateTrace generate a trace for the given Service
// You can also pass some more parameters if you want to generate a trace
// in a given context (e.g. generate nested traces)
// * traceID will be used if != 0 or else generated
// * parentID will be used if != 0  or else generated
// * traces is a pointer to a slice of Spans where the trace we generate will be appended
// * minTs/maxTs are float64 timestamps, if != 0 they will be used as time boundaries for generated traces
//   this is something useful when you want to generatet "nested" traces
func generateTrace(s Service, traceID uint64, parentID uint64, traces *[]model.Span, minTs int64, maxTs int64) int64 {
	t := model.Span{
		TraceID:  traceID,
		SpanID:   uint64(rand.Int63()),
		ParentID: parentID,
		Service:  s.Name,
		Resource: s.ResourceMaker(),
		Type:     "custom",
		Duration: s.DurationMaker(),
	}
	t.Normalize()

	if t.Start < minTs {
		t.Start = minTs
	}
	if maxTs != 0 && t.Start+t.Duration > maxTs {
		t.Duration = maxTs - t.Start
	}

	//log.Printf("service %s, resource %s, duration %f, start %f, traceid %d, parentid %d, trace len %d, minTs %f, maxTs %f",
	//	s.Name, t.Resource, t.Duration, t.Start, traceID, parentID, len(*traces), minTs, maxTs)

	// for the next trace to start after this one
	maxGeneratedTs := t.Start + t.Duration
	if minTs == 0 {
		// except if this trace is the parent trace then the next one is at
		// start + some jitter
		maxGeneratedTs = t.Start
	}

	//log.Printf("maxgents %f", maxGeneratedTs)

	*traces = append(*traces, t)

	// replace that for subservices generation
	if maxTs == 0 {
		maxTs = t.Start + t.Duration
	}
	for _, subs := range s.SubServices {
		// subservices use the maxgen timestamp from last generation to keep them sequential in the timeline
		// ------
		//   s1
		//        -----------
		//			   s2
		genTs := generateTrace(subs, t.TraceID, t.SpanID, traces, maxGeneratedTs, maxTs)
		if genTs > maxGeneratedTs {
			maxGeneratedTs = genTs
		}
	}

	return maxGeneratedTs
}
