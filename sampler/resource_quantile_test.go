package sampler

import (
	"sort"
	"testing"
	"time"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
)

func TestSampler(t *testing.T) {
	assert := assert.New(t)

	conf := config.NewDefaultAgentConfig()
	conf.SamplerQuantiles = []float64{0, 0.25, 0.5, 0.95, 0.99, 1}
	conf.LatencyResolution = time.Nanosecond
	sampler := NewResourceQuantileSampler(conf)

	type sampleResult struct {
		quantile float64
		value    float64
		samples  []uint64
	}

	testTraces := []model.Trace{
		model.Trace{
			model.Span{TraceID: 100, SpanID: 10, Service: "x", Name: "y", Resource: "z", Duration: 573496},
			model.Span{TraceID: 100, SpanID: 14, Service: "x", Name: "y", Resource: "z", Duration: 513323},
		},
		model.Trace{
			model.Span{TraceID: 101, SpanID: 13, Service: "x", Name: "y", Resource: "z", Duration: 26965},
			model.Span{TraceID: 101, SpanID: 18, Service: "x", Name: "y", Resource: "z", Duration: 197884},
			model.Span{TraceID: 101, SpanID: 24, Service: "x", Name: "y", Resource: "z", Duration: 12304982304},
			model.Span{TraceID: 101, SpanID: 30, Service: "x", Name: "y", Resource: "z", Duration: 34384993},
		},
		model.Trace{
			model.Span{TraceID: 102, SpanID: 11, Service: "x", Name: "y", Resource: "z", Duration: 992312},
			model.Span{TraceID: 102, SpanID: 15, Service: "x", Name: "y", Resource: "z", Duration: 34347},
			model.Span{TraceID: 102, SpanID: 28, Service: "x", Name: "y", Resource: "z", Duration: 349944},
		},
		model.Trace{
			model.Span{TraceID: 103, SpanID: 17, Service: "x", Name: "y", Resource: "z", Duration: 19207},
			model.Span{TraceID: 103, SpanID: 22, Service: "x", Name: "y", Resource: "z", Duration: 234923874},
			model.Span{TraceID: 103, SpanID: 25, Service: "x", Name: "y", Resource: "z", Duration: 152342344},
			model.Span{TraceID: 103, SpanID: 27, Service: "x", Name: "y", Resource: "z", Duration: 1523444},
		},
		model.Trace{
			model.Span{TraceID: 104, SpanID: 19, Service: "x", Name: "y", Resource: "z", Duration: 151384},
			model.Span{TraceID: 104, SpanID: 20, Service: "x", Name: "y", Resource: "z", Duration: 8937423},
			model.Span{TraceID: 104, SpanID: 21, Service: "x", Name: "y", Resource: "z", Duration: 2342342},
			model.Span{TraceID: 104, SpanID: 26, Service: "x", Name: "y", Resource: "z", Duration: 15234234},
		},
		model.Trace{
			model.Span{TraceID: 105, SpanID: 23, Service: "x", Name: "y", Resource: "z", Duration: 13434},
		},
		model.Trace{
			model.Span{TraceID: 106, SpanID: 29, Service: "x", Name: "y", Resource: "z", Duration: 29999934},
		},
		model.Trace{
			model.Span{TraceID: 108, SpanID: 12, Service: "x", Name: "y", Resource: "z", Duration: 769540},
		},
		model.Trace{
			model.Span{TraceID: 109, SpanID: 16, Service: "x", Name: "y", Resource: "z", Duration: 498798},
		},
	}

	for _, t := range testTraces {
		sampler.AddTrace(t)
	}

	selected := sampler.Flush()

	/* Test results explanations:
	samples = [
		573496, 513323, 26965, 197884, 12304982304, 34384993,
		992312, 34347, 349944, 19207, 234923874, 152342344, 1523444,
		151384, 8937423, 2342342, 15234234, 13434, 29999934, 769540, 498798,
	]

	# find the quantiles
	qs = [0, 0.25, 0.5, 0.95, 0.99, 1]
	import numpy as np
	for q in qs:
		print "q:", q, " v:", np.percentile(samples, q*100, interpolation='nearest')

	>>>
	q: 0  v: 13434
		span: 23, selects trace 105

	FIXME[leo]: here we select span 19 for some reason??
	q: 0.25  v: 197884
		span: 18, selects trace 101
	q: 0.5  v: 769540
		span: 12, selects trace 108
	q: 0.95  v: 234923874
		span: 22, selects trace 103
	q: 0.99  v: 12304982304
		span: 24, selects trace 101
	q: 1  v: 12304982304
		span: 24, selects trace 101
	*/

	texp := []int{
		101,
		103,
		104,
		105,
		108,
	}
	sexp := []int{
		13, 18, 24, 30, // 101
		17, 22, 25, 27, // 103
		19, 20, 21, 26, // 104
		23, // 105
		12, // 108
	}

	var tgot []int
	var sgot []int
	for _, t := range selected {
		tgot = append(tgot, int(t[0].TraceID))
		for _, s := range t {
			sgot = append(sgot, int(s.SpanID))
		}
	}

	sort.Ints(tgot)
	sort.Ints(sgot)
	sort.Ints(texp)
	sort.Ints(sexp)

	assert.Equal(texp, tgot, "sampled the wrong traces")
	assert.Equal(sexp, sgot, "sampled the wrong spans")
}

type uintslice []uint64

func (s uintslice) Len() int           { return len(s) }
func (s uintslice) Less(i, j int) bool { return s[i] <= s[j] }
func (s uintslice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
