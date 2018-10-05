package main

import (
	"bytes"
	"context"

	"encoding/json"
	"net"
	"net/http"
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func NewTestUDSReceiverFromConfig(conf *config.AgentConfig) *HTTPReceiver {
	dynConf := config.NewDynamicConfig()

	rawTraceChan := make(chan model.Trace, 5000)
	serviceChan := make(chan model.ServicesMetadata, 50)
	receiver := NewHTTPReceiver(conf, dynConf, rawTraceChan, serviceChan)

	return receiver
}

func NewTestUDSReceiverConfig() *config.AgentConfig {
	conf := config.New()
	conf.ReceiverUDSEnabled = true

	return conf
}

func TestUDSReceiverServiceSimpleJSON(t *testing.T) {
	assert := assert.New(t)
	conf := NewTestUDSReceiverConfig()
	receiver := NewTestUDSReceiverFromConfig(conf)
	receiver.Run()

	// send traces to that endpoint without a content-type
	data, err := json.Marshal(fixtures.GetTestTrace(1, 1, false))
	assert.Nil(err)
	req, err := http.NewRequest("POST", "http://unix/v0.4/traces", bytes.NewBuffer(data))
	assert.Nil(err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", conf.ReceiverUDSFile)
			},
		},
	}
	resp, err := client.Do(req)
	assert.Nil(err)

	// just perform a single request and check the status code
	assert.Equal(200, resp.StatusCode)

	select {
	case rt := <-receiver.traces:
		assert.Len(rt, 1)
		span := rt[0]
		assert.Equal(uint64(42), span.TraceID)
		assert.Equal(uint64(52), span.SpanID)
		assert.Equal("fennel_is_amazing", span.Service)
		assert.Equal("something_that_should_be_a_metric", span.Name)
		assert.Equal("NOT touched because it is going to be hashed", span.Resource)
		assert.Equal("192.168.0.1", span.Meta["http.host"])
		assert.Equal(41.99, span.Metrics["http.monitor"])
	default:
		t.Fatalf("no data received")
	}

	receiver.Stop()
}
