package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

type dataFromAPI struct {
	urlPath   string
	urlParams map[string][]string
	header    http.Header
	body      string
}

func newTestServer(t *testing.T, data chan dataFromAPI) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Errorf("test server: cannot read request body: %v", err)
			return
		}
		defer r.Body.Close()

		data <- dataFromAPI{
			urlPath:   r.URL.Path,
			urlParams: r.URL.Query(),
			header:    r.Header,
			body:      string(body),
		}
		w.WriteHeader(http.StatusOK)
	}))
}

func newFailingTestServer(t *testing.T, status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
}

func newTestPayload(env string) model.AgentPayload {
	return model.AgentPayload{
		HostName: "test.host",
		Env:      env,
		Traces:   []model.Trace{model.Trace{fixtures.TestSpan()}},
		Stats:    []model.StatsBucket{fixtures.TestStatsBucket()},
	}
}

func TestWriterServices(t *testing.T) {
	assert := assert.New(t)
	// where we'll receive data
	data := make(chan dataFromAPI, 1)

	// make a real HTTP endpoint so we can test that too
	testAPI := newTestServer(t, data)
	defer testAPI.Close()

	conf := config.NewDefaultAgentConfig()
	conf.APIEndpoints = []string{testAPI.URL}
	conf.APIKeys = []string{"xxxxxxx"}

	w := NewWriter(conf)
	w.inServices = make(chan model.ServicesMetadata)

	go w.Run()

	// send services
	services := model.ServicesMetadata{
		"mcnulty": map[string]string{
			"app_type": "web",
		},
	}

	w.inServices <- services

receivingLoop:
	for {
		select {
		case received := <-data:
			assert.Equal("/api/v0.1/services", received.urlPath)
			assert.Equal(map[string][]string{
				"api_key": []string{"xxxxxxx"},
			}, received.urlParams)
			assert.Equal("application/json", received.header.Get("Content-Type"))
			assert.Equal("", received.header.Get("Content-Encoding"))
			assert.Equal(`{"mcnulty":{"app_type":"web"}}`, received.body)
			break receivingLoop
		case <-time.After(time.Second):
			t.Fatal("did not receive service data in time")
		}
	}
}

func TestWriterPayload(t *testing.T) {
	assert := assert.New(t)

	data := make(chan dataFromAPI, 1)

	server := newTestServer(t, data)
	defer server.Close()

	conf := config.NewDefaultAgentConfig()
	conf.APIEndpoints = []string{server.URL}
	conf.APIKeys = []string{"key"}

	w := NewWriter(conf)
	go w.Run()

	w.inPayloads <- newTestPayload("test")

receivingLoop:
	for {
		select {
		case received := <-data:
			assert.Equal("/api/v0.1/collector", received.urlPath)
			assert.Equal(map[string][]string{"api_key": []string{"key"}}, received.urlParams)
			assert.Equal("application/json", received.header.Get("Content-Type"))
			assert.Equal("gzip", received.header.Get("Content-Encoding"))
			// do not assert the body yet
			break receivingLoop
		case <-time.After(time.Second):
			t.Fatal("did not receive service data in time")
		}
	}

	w.Stop()

	assert.Equal(0, len(w.payloadBuffer))
}

func TestWriterPayloadErrors(t *testing.T) {
	assert := assert.New(t)

	data := make(chan dataFromAPI, 1)

	server := newTestServer(t, data)
	defer server.Close()

	failingServer400 := newFailingTestServer(t, http.StatusBadRequest)
	defer failingServer400.Close()

	failingServer500 := newFailingTestServer(t, http.StatusInternalServerError)
	defer failingServer500.Close()

	conf := config.NewDefaultAgentConfig()
	conf.APIEndpoints = []string{server.URL, failingServer400.URL, failingServer500.URL}
	conf.APIKeys = []string{"key", "key400", "key500"}

	w := NewWriter(conf)
	go w.Run()

	w.inPayloads <- newTestPayload("test")

receivingLoop:
	for {
		select {
		case received := <-data:
			assert.Equal("/api/v0.1/collector", received.urlPath)
			assert.Equal(map[string][]string{"api_key": []string{"key"}}, received.urlParams)
			assert.Equal("application/json", received.header.Get("Content-Type"))
			assert.Equal("gzip", received.header.Get("Content-Encoding"))
			// do not assert the body yet
			break receivingLoop
		case <-time.After(time.Second):
			t.Fatal("did not receive service data in time")
		}
	}

	w.Stop()

	// The payload for failingServer500 must have been kept in the buffer since it
	// could not be written. The payload for failingServer400 must not
	// have been kept since we do not retry on 4xx errors.
	assert.Equal(1, len(w.payloadBuffer))

	p0 := w.payloadBuffer[0]
	endpoint := p0.endpoint.(*APIEndpoint)
	assert.Equal(1, len(endpoint.apiKeys))
	assert.Equal("key500", endpoint.apiKeys[0])
}

func TestWriterBuffering(t *testing.T) {
	assert := assert.New(t)

	nbPayloads := 3
	payloads := make([]model.AgentPayload, nbPayloads)
	payloadSizes := make([]int, nbPayloads)
	for i := range payloads {
		payload := newTestPayload(fmt.Sprintf("p%d", i))
		payloads[i] = payload

		data, err := model.EncodeAgentPayload(payload)
		if err != nil {
			t.Fatalf("cannot encode test payload: %v", err)
		}
		payloadSizes[i] = len(data)
	}

	// Use a server that will reject all requests to make sure our
	// payloads are kept in the buffer.
	server := newFailingTestServer(t, http.StatusInternalServerError)
	defer server.Close()

	conf := config.NewDefaultAgentConfig()
	conf.APIEndpoints = []string{server.URL}
	conf.APIKeys = []string{"key"}
	conf.APIPayloadBufferMaxSize = payloadSizes[0] + payloadSizes[1]

	w := NewWriter(conf)
	// Make the chan unbuffered to block on write
	w.inPayloads = make(chan model.AgentPayload)
	go w.Run()

	for _, payload := range payloads {
		w.inPayloads <- payload
	}

	w.Stop()

	// Since the writer was created with a buffer just large enough for
	// the first two payloads, the third payload overflowed the buffer,
	// and the first and oldest payload (p0) was discarded.
	assert.Equal(2, len(w.payloadBuffer))
	assert.Equal("p1", w.payloadBuffer[0].payload.Env)
	assert.Equal("p2", w.payloadBuffer[1].payload.Env)
}

func TestWriterDisabledBuffering(t *testing.T) {
	assert := assert.New(t)

	server := newFailingTestServer(t, http.StatusInternalServerError)
	defer server.Close()

	conf := config.NewDefaultAgentConfig()
	conf.APIEndpoints = []string{server.URL}
	conf.APIKeys = []string{"key"}
	conf.APIPayloadBufferMaxSize = 0

	w := NewWriter(conf)
	// Make the chan unbuffered to block on write
	w.inPayloads = make(chan model.AgentPayload)
	go w.Run()

	w.inPayloads <- newTestPayload("test")

	w.Stop()

	// Since buffering is disabled, the payload should have been
	// dropped and the buffer should be empty.
	assert.Equal(0, len(w.payloadBuffer))
}
