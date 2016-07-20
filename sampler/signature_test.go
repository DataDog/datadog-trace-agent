package sampler

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
)

func getTestSampler() *SignatureSampler {
	conf := config.NewDefaultAgentConfig()
	conf.SamplerTheta = 60
	conf.SamplerJitter = 0
	conf.SamplerSMin = 5
	sampler := NewSignatureSampler(conf)

	return sampler
}

func TestSignature(t *testing.T) {
	assert := assert.New(t)

	sampler := getTestSampler()

	testTraces := []model.Trace{
		// New, keep it
		model.Trace{
			model.Span{TraceID: 100, SpanID: 1001, Service: "x1", Name: "y1", Resource: "z1", Duration: 573496},
			model.Span{TraceID: 100, SpanID: 1002, ParentID: 1001, Service: "x1", Name: "y1", Resource: "z1", Duration: 513323},
		},
		// New, keep it
		model.Trace{
			model.Span{TraceID: 101, SpanID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 26965},
			model.Span{TraceID: 101, SpanID: 1012, ParentID: 1011, Service: "x1", Name: "y1", Resource: "z1", Duration: 197884},
			model.Span{TraceID: 101, SpanID: 1013, ParentID: 1012, Service: "x1", Name: "y1", Resource: "z1", Duration: 12304982304},
			model.Span{TraceID: 101, SpanID: 1014, ParentID: 1013, Service: "x2", Name: "y2", Resource: "z2", Duration: 34384993},
		},
		// Same as the previous one
		model.Trace{
			model.Span{TraceID: 102, SpanID: 1021, Service: "x1", Name: "y1", Resource: "z1", Duration: 992312},
			model.Span{TraceID: 102, SpanID: 1022, ParentID: 1021, Service: "x1", Name: "y1", Resource: "z1", Duration: 34347},
			model.Span{TraceID: 102, SpanID: 1023, ParentID: 1022, Service: "x2", Name: "y2", Resource: "z2", Duration: 349944},
		},
		// Different because it has an error
		model.Trace{
			model.Span{TraceID: 110, SpanID: 1101, Service: "x1", Name: "y1", Resource: "z1", Duration: 992312},
			model.Span{TraceID: 110, SpanID: 1102, ParentID: 1101, Service: "x1", Name: "y1", Resource: "z1", Error: 1, Duration: 34347},
			model.Span{TraceID: 110, SpanID: 1103, ParentID: 1101, Service: "x2", Name: "y2", Resource: "z2", Duration: 349944},
		},
		// New, keep it
		model.Trace{
			model.Span{TraceID: 103, SpanID: 1031, Service: "x1", Name: "y1", Resource: "z2", Duration: 19207},
			model.Span{TraceID: 103, SpanID: 1032, ParentID: 1031, Service: "x1", Name: "y1", Resource: "z1", Duration: 234923874},
			model.Span{TraceID: 103, SpanID: 1033, ParentID: 1032, Service: "x1", Name: "y1", Resource: "z1", Duration: 152342344},
		},
		// Same as previous one
		model.Trace{
			model.Span{TraceID: 104, SpanID: 1041, Service: "x1", Name: "y1", Resource: "z2", Duration: 151384},
			model.Span{TraceID: 104, SpanID: 1042, ParentID: 1041, Service: "x1", Name: "y1", Resource: "z2", Duration: 8937423},
			model.Span{TraceID: 104, SpanID: 1043, ParentID: 1041, Service: "x1", Name: "y1", Resource: "z3", Duration: 2342342},
		},
		model.Trace{
			model.Span{TraceID: 105, SpanID: 1051, Service: "x2", Name: "y2", Resource: "z2", Duration: 13434},
		},
	}

	for _, t := range testTraces {
		sampler.AddTrace(t)
	}

	selected := sampler.Flush()

	texp := []int{
		100,
		101,
		110,
		103,
		105,
	}

	var tgot []int
	for _, t := range selected {
		tgot = append(tgot, int(t[0].TraceID))
	}

	sort.Ints(tgot)
	sort.Ints(texp)

	assert.Equal(texp, tgot, "sampled the wrong traces")
}

func TestHardLimit(t *testing.T) {
	assert := assert.New(t)

	sampler := getTestSampler()

	signatureCount := 5 * int(sampler.tpsMax)
	traceCount := 4 * signatureCount

	for i := 1; i <= traceCount; i++ {
		t := model.Trace{
			model.Span{TraceID: uint64(i), SpanID: uint64(i * 100), Service: "s1", Name: "n1", Resource: string(rand.Intn(signatureCount)), Duration: 1234},
			model.Span{TraceID: uint64(i), SpanID: uint64(i*100 + 1), ParentID: uint64(i * 100), Service: "s1", Name: "n1", Resource: "whatever", Duration: 5678},
			model.Span{TraceID: uint64(i), SpanID: uint64(i*100 + 2), ParentID: uint64(i * 100), Service: "s2", Name: "n2", Resource: "whatever", Duration: 5678},
		}
		sampler.AddTrace(t)
	}

	selected := sampler.Flush()

	assert.True(len(selected) <= int(sampler.tpsMax), "We kept more traces than what the hard limit should allow")
}
