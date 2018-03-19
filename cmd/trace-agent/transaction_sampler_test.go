package main

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func newRandomWeightedSpanWithService(service string) *model.WeightedSpan {
	span := fixtures.RandomSpan()
	span.Service = service

	return &model.WeightedSpan{
		Span:     span,
		Weight:   1.0,
		TopLevel: true,
	}
}

func newRandomWeightedSpan() *model.WeightedSpan {
	span := fixtures.RandomSpan()

	return &model.WeightedSpan{
		Span:     span,
		Weight:   1.0,
		TopLevel: true,
	}
}

func TestTransactionSamplerConfig(t *testing.T) {
	assert := assert.New(t)

	ts := newTestTransactionSampler()

	conf := make(chan *config.ServerConfig)
	go ts.Listen(conf)

	done := make(chan struct{})

	// watch the outbound channel for an expected sequence of spans
	go func() {
		countUntil := 3
		analyzedCount := 0
		expected := []string{"web", "web", "db"}

		for {
			select {
			case span := <-ts.analyzed:
				// spans should be analyzed only if the config says so
				assert.Equal(span.Service, expected[analyzedCount])
				analyzedCount++

				if analyzedCount == countUntil {
					done <- struct{}{}
				}
			default:
			}
		}
	}()

	// analyze only spans with service "web"
	conf <- &config.ServerConfig{
		AnalyzedServices: map[string]float64{
			"web": 1.0,
		},
	}

	// add a few spans that should not be analyzed
	for i := 0; i < 10; i++ {
		testTrace := processedTrace{
			Env:           "none",
			WeightedTrace: []*model.WeightedSpan{newRandomWeightedSpan()},
		}

		ts.Add(testTrace)
	}

	// now add some spans that should be
	testTrace2 := processedTrace{
		Env:           "none",
		WeightedTrace: []*model.WeightedSpan{newRandomWeightedSpanWithService("web")},
	}
	ts.Add(testTrace2)
	ts.Add(testTrace2)

	// flip the config to analyze the "db" service
	conf <- &config.ServerConfig{
		AnalyzedServices: map[string]float64{
			"db": 1.0,
		},
	}

	// shouldn't be analyzed
	ts.Add(testTrace2)

	testTrace3 := processedTrace{
		Env:           "none",
		WeightedTrace: []*model.WeightedSpan{newRandomWeightedSpanWithService("db")},
	}

	// should be analyzed
	ts.Add(testTrace3)

	<-done
}

func newTestTransactionSampler() *TransactionSampler {
	return &TransactionSampler{
		analyzed:              make(chan *model.Span),
		analyzedRateByService: make(map[string]float64),
	}
}
