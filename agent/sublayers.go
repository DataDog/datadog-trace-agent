package main

import (
	"bytes"

	"github.com/DataDog/datadog-trace-agent/model"
)

const (
	metricPrefixType    = "_sublayers.duration.by_type.sublayer_type:"
	metricPrefixService = "_sublayers.duration.by_service.sublayer_service:"
)

// SublayerTagger is a dumb worker that adds sublayer stats to traces
type SublayerTagger struct {
	in  chan model.Trace
	out chan model.Trace
}

// NewSublayerTagger inits a new SublayerTagger
func NewSublayerTagger(in chan model.Trace) *SublayerTagger {
	return &SublayerTagger{
		in:  in,
		out: make(chan model.Trace),
	}
}

// Run starts tagging sublayers onto traces
func (st *SublayerTagger) Run() {
	for t := range st.in {
		st.out <- tagSublayers(t)
	}

	close(st.out)
}

func tagSublayers(t model.Trace) model.Trace {
	iter := model.NewTraceLevelIterator(t)
	root, err := iter.NextSpan()
	if err != nil {
		// no root, skip sublayers
		return t
	}

	ss := newSublayerSpan()
	ss.Add(root)

	for iter.NextLevel() == nil {
		for cur, err := iter.NextSpan(); err == nil; cur, err = iter.NextSpan() {
			ss.Add(cur)
		}
	}

	// account for sublayer statsx
	var mName bytes.Buffer
	byType, byService := ss.OutputStats()

	if root.Metrics == nil {
		root.Metrics = make(map[string]float64)
	}
	root.Metrics["_sublayers.span_count"] = float64(len(t))
	for k, v := range byType {
		mName.WriteString(metricPrefixType)
		mName.WriteString(k)
		root.Metrics[mName.String()] = float64(v)
		mName.Reset()
	}
	for k, v := range byService {
		mName.WriteString(metricPrefixService)
		mName.WriteString(k)
		root.Metrics[mName.String()] = float64(v)
		mName.Reset()
	}

	return t
}

type timeSpan struct {
	Name     string
	Start    int64
	Duration int64
}

func insertTS(sts []timeSpan, ts timeSpan) []timeSpan {
	// don't do anything with unnamed
	if ts.Name == "" {
		return sts
	}

	if len(sts) == 0 {
		sts = append(sts, ts)
	} else {
		// find the timeSpan that contains that one (we go down in the tree, so
		// it should exist) - if not just skip
		// FIXME does NOT work for parallel stuff
		for i, ots := range sts {
			// >======================< ots
			//          >======<        ts
			if ots.Start <= ts.Start && ots.Start+ots.Duration >= ts.Start+ts.Duration {
				// do not account for if same name
				if ots.Name == ts.Name {
					return sts
				}

				// transform into
				// >========<               [i]   ots
				//          >======<        [i+1] ts
				//                 >======< [i+2] newts
				newts := timeSpan{ots.Name, ts.Start + ts.Duration, ots.Start + ots.Duration - (ts.Start + ts.Duration)}
				ots.Duration = ts.Start - ots.Start
				sts = append(sts, timeSpan{}, timeSpan{})
				copy(sts[i+2:], sts[i:])
				sts[i] = ots
				sts[i+1] = ts
				sts[i+2] = newts
				break
			}
		}
	}
	return sts
}

type sublayerSpan struct {
	byType    []timeSpan
	byService []timeSpan
}

func newSublayerSpan() *sublayerSpan {
	return &sublayerSpan{
		byType:    []timeSpan{},
		byService: []timeSpan{},
	}
}

func (ss *sublayerSpan) Add(s *model.Span) {
	tsType := timeSpan{s.Type, s.Start, s.Duration}
	tsService := timeSpan{s.Service, s.Start, s.Duration}

	ss.byType = insertTS(ss.byType, tsType)
	ss.byService = insertTS(ss.byService, tsService)
}

func (ss *sublayerSpan) OutputStats() (map[string]float64, map[string]float64) {
	mType := make(map[string]float64)
	mService := make(map[string]float64)

	for _, ts := range ss.byType {
		mType[ts.Name] += float64(ts.Duration)
	}
	for _, ts := range ss.byService {
		mService[ts.Name] += float64(ts.Duration)
	}

	return mType, mService
}
