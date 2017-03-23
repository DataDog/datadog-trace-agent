package model

// SublayerValue is just a span-metric placeholder for a given
// sublayer val
type SublayerValue struct {
	Metric string
	Tag    Tag
	Value  float64
}

// ComputeSublayers extracts sublayer values by type & service for a trace
func ComputeSublayers(t *Trace) []SublayerValue {
	iter := NewTraceLevelIterator(*t)
	root, err := iter.NextSpan()
	if err != nil {
		// no root, skip sublayers
		return []SublayerValue{}
	}

	ss := newSublayerSpans()
	ss.Add(root)

	for iter.NextLevel() == nil {
		for cur, err := iter.NextSpan(); err == nil; cur, err = iter.NextSpan() {
			ss.Add(cur)
		}
	}

	s := ss.OutputSublayers()
	s = append(s, SublayerValue{
		Metric: "_sublayers.span_count",
		Value:  float64(len(*t)),
	})

	return s
}

// SetSublayersOnSpan takes some sublayers and pins them on the given span.Metrics
func SetSublayersOnSpan(span *Span, values []SublayerValue) {
	if span.Metrics == nil {
		span.Metrics = make(map[string]float64, len(values))
	}

	for _, value := range values {
		name := value.Metric

		if value.Tag.Name != "" {
			name = name + "." + value.Tag.Name + ":" + value.Tag.Value
		}

		span.Metrics[name] = value.Value
	}
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

type sublayerSpans struct {
	byType    []timeSpan
	byService []timeSpan
}

func newSublayerSpans() *sublayerSpans {
	return &sublayerSpans{
		byType:    []timeSpan{},
		byService: []timeSpan{},
	}
}

func (ss *sublayerSpans) Add(s *Span) {
	tsType := timeSpan{s.Type, s.Start, s.Duration}
	tsService := timeSpan{s.Service, s.Start, s.Duration}

	ss.byType = insertTS(ss.byType, tsType)
	ss.byService = insertTS(ss.byService, tsService)
}

func (ss *sublayerSpans) OutputSublayers() []SublayerValue {
	mType := make(map[string]float64)
	mService := make(map[string]float64)

	for _, ts := range ss.byType {
		mType[ts.Name] += float64(ts.Duration)
	}
	for _, ts := range ss.byService {
		mService[ts.Name] += float64(ts.Duration)
	}

	sublayers := make([]SublayerValue, 0, len(mType)+len(mService)+1)
	for k, v := range mType {
		sublayers = append(sublayers, SublayerValue{
			Metric: "_sublayers.duration.by_type",
			Tag:    Tag{"sublayer_type", k},
			Value:  v,
		})
	}
	for k, v := range mService {
		sublayers = append(sublayers, SublayerValue{
			Metric: "_sublayers.duration.by_service",
			Tag:    Tag{"sublayer_service", k},
			Value:  v,
		})
	}
	return sublayers
}
