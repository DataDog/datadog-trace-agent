package sampler

import (
	"sort"
	"testing"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
	"github.com/stretchr/testify/assert"
)

func TestSampler(t *testing.T) {
	assert := assert.New(t)

	conf := config.NewDefaultAgentConfig()
	conf.SamplerQuantiles = []float64{0, 0.25, 0.5, 0.75, 0.90, 0.95, 0.99, 1}
	sampler := NewResourceQuantileSampler(conf)

	type sampleResult struct {
		quantile float64
		value    int64
		samples  []uint64
	}

	testSpans := []model.Span{
		model.Span{TraceID: 100, SpanID: 10, Duration: 573496},
		model.Span{TraceID: 102, SpanID: 11, Duration: 992312},
		model.Span{TraceID: 108, SpanID: 12, Duration: 769540},
		model.Span{TraceID: 101, SpanID: 13, Duration: 26965},
		model.Span{TraceID: 100, SpanID: 14, Duration: 513323},
		model.Span{TraceID: 102, SpanID: 15, Duration: 34347},
		model.Span{TraceID: 109, SpanID: 16, Duration: 498798},
		model.Span{TraceID: 103, SpanID: 17, Duration: 19207},
		model.Span{TraceID: 101, SpanID: 18, Duration: 197884},
		model.Span{TraceID: 104, SpanID: 19, Duration: 151384},
		model.Span{TraceID: 104, SpanID: 20, Duration: 8937423},
		model.Span{TraceID: 104, SpanID: 21, Duration: 2342342},
		model.Span{TraceID: 103, SpanID: 22, Duration: 234923874},
		model.Span{TraceID: 105, SpanID: 23, Duration: 13434},
		model.Span{TraceID: 101, SpanID: 24, Duration: 12304982304},
		model.Span{TraceID: 103, SpanID: 25, Duration: 152342344},
		model.Span{TraceID: 104, SpanID: 26, Duration: 15234234},
		model.Span{TraceID: 103, SpanID: 27, Duration: 1523444},
		model.Span{TraceID: 102, SpanID: 28, Duration: 349944},
		model.Span{TraceID: 106, SpanID: 29, Duration: 29999934},
		model.Span{TraceID: 101, SpanID: 30, Duration: 34384993},
	}

	expectedResults := []sampleResult{
		sampleResult{quantile: 0, value: int64(13434), samples: []uint64{23}},
		// TODO[leo]: result different from numpy, investigate
		sampleResult{quantile: 0.25, value: int64(151384), samples: []uint64{19}},
		sampleResult{quantile: 0.5, value: int64(769540), samples: []uint64{12}},
		sampleResult{quantile: 0.75, value: int64(15234234), samples: []uint64{26}},
		sampleResult{quantile: 0.9, value: int64(152342344), samples: []uint64{25}},
		sampleResult{quantile: 0.95, value: int64(234923874), samples: []uint64{22}},
		sampleResult{quantile: 0.99, value: int64(12304982304), samples: []uint64{24}},
		sampleResult{quantile: 1, value: int64(12304982304), samples: []uint64{24}},
	}

	shouldChoose := []int{12, 13, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 30}

	for _, s := range testSpans {
		sampler.AddSpan(s)
	}

	// Now prepare distributions
	stats := model.NewStatsBucket(0, 1)
	tgs := model.NewTagsFromString("service:dogweb,resource:dash.list")
	d := model.NewDistribution(model.DURATION, tgs)
	for _, span := range testSpans {
		d.Add(span)
	}

	for _, r := range expectedResults {
		val, samples := d.Summary.Quantile(r.quantile)
		assert.Equal(r.value, val, "Expected value %d instead of %d for quantile %f", r.value, val, r.quantile)
		assert.Equal(r.samples, samples, "Expected samples %v instead of %v for quantile %f", r.samples, samples, r.quantile)
	}

	// Add one fake distribution for choosing
	stats.Distributions["whatever"] = d
	chosen := sampler.GetSamples(sampler.traceIDBySpanID, sampler.spansByTraceID, stats)

	// step1: chosen spans by distributions: 18, 12, 11
	chosenSID := make([]int, len(chosen))
	for i, s := range chosen {
		chosenSID[i] = int(s.SpanID)
	}
	sort.Ints(chosenSID)

	// Verify that are our samples are correct
	assert.Equal(shouldChoose, chosenSID)
}

/* small python helper to generate the test values above
import numpy as np
import re

R_DUR = re.compile(r'TraceID: (\d+),\s+SpanID: (\d+),\s+Duration: (\d+)')

vals = """
    testSpans := []model.Span{
        model.Span{TraceID: 100, SpanID: 10, Duration: 573496},
        model.Span{TraceID: 102, SpanID: 11, Duration: 992312},
        model.Span{TraceID: 108, SpanID: 12, Duration: 769540},
        model.Span{TraceID: 101, SpanID: 13, Duration: 26965},
        model.Span{TraceID: 100, SpanID: 14, Duration: 513323},
        model.Span{TraceID: 102, SpanID: 15, Duration: 34347},
        model.Span{TraceID: 109, SpanID: 16, Duration: 498798},
        model.Span{TraceID: 103, SpanID: 17, Duration: 19207},
        model.Span{TraceID: 101, SpanID: 18, Duration: 197884},
        model.Span{TraceID: 104, SpanID: 19, Duration: 151384},
        model.Span{TraceID: 104, SpanID: 20, Duration: 8937423},
        model.Span{TraceID: 104, SpanID: 21, Duration: 2342342},
        model.Span{TraceID: 103, SpanID: 22, Duration: 234923874},
        model.Span{TraceID: 105, SpanID: 23, Duration: 13434},
        model.Span{TraceID: 101, SpanID: 24, Duration: 12304982304},
        model.Span{TraceID: 103, SpanID: 25, Duration: 152342344},
        model.Span{TraceID: 104, SpanID: 26, Duration: 15234234},
        model.Span{TraceID: 103, SpanID: 27, Duration: 1523444},
        model.Span{TraceID: 102, SpanID: 28, Duration: 349944},
        model.Span{TraceID: 106, SpanID: 29, Duration: 29999934},
        model.Span{TraceID: 101, SpanID: 30, Duration: 34384993},
    }
"""
print vals

quantiles = [0, 0.25, 0.50, 0.75, 0.90, 0.95, 0.99, 1]
traces, spans, durations = [], [], []

for line in vals.splitlines():
    m = R_DUR.search(line)
    if not m:
        continue

    g = m.groups()
    assert len(g) == 3
    traces.append(int(g[0]))
    spans.append(int(g[1]))
    durations.append(int(g[2]))

expected_quantiles = []
straces = {}

print "expectedResults := []sampleResult{"
for q in quantiles:
    val = np.percentile(durations, q*100, interpolation='nearest')
    idx = durations.index(val)
    straces[traces[idx]] = True
    print "  sampleResult{{quantile: {0}, value: int64({1}), samples: []uint64{{{2}}}}},".format(q, val, spans[idx])
print "}"

chosen = []
for s, t in zip(spans, traces):
    if straces.get(t):
        chosen.append(str(s))
print "shouldChoose := []uint64{%s}" % ','.join(sorted(chosen))
*/
