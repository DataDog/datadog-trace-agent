package main

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func createTrace(serviceName string, operationName string, topLevel bool) processedTrace {
	ws := model.WeightedSpan{TopLevel: topLevel, Span: &model.Span{Service: serviceName, Name: operationName}}
	wt := model.WeightedTrace{&ws}
	return processedTrace{WeightedTrace: wt}
}

func TestTransactionSampler(t *testing.T) {
	assert := assert.New(t)

	config := make(map[string]map[string]float64)
	config["myService"] = make(map[string]float64)
	config["myService"]["myOperation"] = 1

	analyzed := make(chan *model.Span, 10)

	ts := newTransactionSampler(config, analyzed)
	ts.Add(createTrace("myService", "myOperation", true))
	ts.Add(createTrace("otherService", "myOperation", true))
	ts.Add(createTrace("myService", "otherOperation", true))
	ts.Add(createTrace("otherService", "otherOperation", true))
	ts.Add(createTrace("myService", "myOperation", false))
	close(analyzed)

	analyzedSpans := make([]*model.Span, 0)
	for s := range analyzed {
		analyzedSpans = append(analyzedSpans, s)
	}

	assert.True(ts.Enabled())
	assert.Len(analyzedSpans, 1)
}
