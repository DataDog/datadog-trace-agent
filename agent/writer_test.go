package main

import (
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

func TestWriterServices(t *testing.T) {
	assert := assert.New(t)
	// where we'll receive data
	data := make(chan dataFromAPI, 1)

	// make a real HTTP endpoint so we can test that too
	testAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Logf("http test server problem when reading, %v", err)
			return
		}
		defer r.Body.Close()

		data <- dataFromAPI{
			urlPath:   r.URL.Path,
			urlParams: r.URL.Query(),
			header:    r.Header,
			body:      string(body),
		}
		w.WriteHeader(200)
	}))

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
	// where we'll receive data
	data := make(chan dataFromAPI, 1)

	// make a real HTTP endpoint so we can test that too
	testAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Logf("http test server problem when reading, %v", err)
			return
		}
		defer r.Body.Close()

		data <- dataFromAPI{
			urlPath:   r.URL.Path,
			urlParams: r.URL.Query(),
			header:    r.Header,
			body:      string(body),
		}
		w.WriteHeader(200)
	}))

	// buggy server
	testAPI2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
	}))

	defer testAPI.Close()
	defer testAPI2.Close()

	conf := config.NewDefaultAgentConfig()
	conf.APIEndpoints = []string{testAPI.URL, testAPI2.URL}
	conf.APIKeys = []string{"xxxxxxx", "yyyyyyyy"}

	w := NewWriter(conf)
	go w.Run()

	p := model.AgentPayload{
		HostName: "test.host",
		Traces:   []model.Trace{model.Trace{fixtures.TestSpan()}},
		Stats:    []model.StatsBucket{fixtures.TestStatsBucket()},
	}

	w.inPayloads <- p

receivingLoop:
	for {
		select {
		case received := <-data:
			assert.Equal("/api/v0.1/collector", received.urlPath)
			assert.Equal(map[string][]string{
				"api_key": []string{"xxxxxxx"},
			}, received.urlParams)
			assert.Equal("application/json", received.header.Get("Content-Type"))
			assert.Equal("gzip", received.header.Get("Content-Encoding"))
			// do not assert the body yet
			break receivingLoop
		case <-time.After(time.Second):
			t.Fatal("did not receive service data in time")
		}
	}
	time.Sleep(100 * time.Millisecond)

	// we should just have ignored the 400 error on the other backend
	assert.Equal(w.getPayloadBufferLen(), 0)
}
