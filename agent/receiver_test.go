package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
	"github.com/tinylib/msgp/msgp"
)

func TestReceiverRequestBodyLength(t *testing.T) {
	assert := assert.New(t)

	conf := config.NewDefaultAgentConfig()
	conf.APIKeys = []string{"test"}

	// save the global mux aside, we don't want to break other tests
	defaultMux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()

	receiver := NewHTTPReceiver(conf)
	receiver.maxRequestBodyLength = 2
	go receiver.Run()

	defer func() {
		close(receiver.exit)
		// we need to wait more than on second (time for StoppableListener.Accept
		// to acknowledge the connection has been closed)
		time.Sleep(2 * time.Second)
		http.DefaultServeMux = defaultMux
	}()

	url := fmt.Sprintf("http://%s:%d/v0.3/traces",
		conf.ReceiverHost, conf.ReceiverPort)

	// Before going further, make sure receiver is started
	// since it's running in another goroutine
	for i := 0; i < 10; i++ {
		client := &http.Client{}

		body := bytes.NewBufferString("[]")
		req, err := http.NewRequest("POST", url, body)
		assert.Nil(err)

		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	testBody := func(expectedStatus int, bodyData string) {
		client := &http.Client{}

		body := bytes.NewBufferString(bodyData)
		req, err := http.NewRequest("POST", url, body)
		assert.Nil(err)

		resp, err := client.Do(req)
		assert.Nil(err)
		assert.Equal(expectedStatus, resp.StatusCode)
	}

	testBody(http.StatusOK, "[]")
	testBody(http.StatusRequestEntityTooLarge, " []")
}

func TestLegacyReceiver(t *testing.T) {
	// testing traces without content-type in agent endpoints, it should use JSON decoding
	assert := assert.New(t)
	config := config.NewDefaultAgentConfig()
	testCases := []struct {
		name        string
		r           *HTTPReceiver
		apiVersion  APIVersion
		contentType string
		traces      model.Trace
	}{
		{"v01 with empty content-type", NewHTTPReceiver(config), v01, "", model.Trace{fixtures.GetTestSpan()}},
		{"v01 with application/json", NewHTTPReceiver(config), v01, "application/json", model.Trace{fixtures.GetTestSpan()}},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(tc.name), func(t *testing.T) {
			// start testing server
			server := httptest.NewServer(
				http.HandlerFunc(tc.r.httpHandleWithVersion(tc.apiVersion, tc.r.handleTraces)),
			)

			// send traces to that endpoint without a content-type
			data, err := json.Marshal(tc.traces)
			assert.Nil(err)
			req, err := http.NewRequest("POST", server.URL, bytes.NewBuffer(data))
			assert.Nil(err)
			req.Header.Set("Content-Type", tc.contentType)

			client := &http.Client{}
			resp, err := client.Do(req)
			assert.Nil(err)
			assert.Equal(200, resp.StatusCode)

			// now we should be able to read the trace data
			select {
			case rt := <-tc.r.traces:
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

			resp.Body.Close()
			server.Close()
		})
	}
}

func TestReceiverJSONDecoder(t *testing.T) {
	// testing traces without content-type in agent endpoints, it should use JSON decoding
	assert := assert.New(t)
	config := config.NewDefaultAgentConfig()
	testCases := []struct {
		name        string
		r           *HTTPReceiver
		apiVersion  APIVersion
		contentType string
		traces      []model.Trace
	}{
		{"v02 with empty content-type", NewHTTPReceiver(config), v02, "", fixtures.GetTestTrace(1, 1)},
		{"v03 with empty content-type", NewHTTPReceiver(config), v03, "", fixtures.GetTestTrace(1, 1)},
		{"v02 with application/json", NewHTTPReceiver(config), v02, "application/json", fixtures.GetTestTrace(1, 1)},
		{"v03 with application/json", NewHTTPReceiver(config), v03, "application/json", fixtures.GetTestTrace(1, 1)},
		{"v02 with text/json", NewHTTPReceiver(config), v02, "text/json", fixtures.GetTestTrace(1, 1)},
		{"v03 with text/json", NewHTTPReceiver(config), v03, "text/json", fixtures.GetTestTrace(1, 1)},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(tc.name), func(t *testing.T) {
			// start testing server
			server := httptest.NewServer(
				http.HandlerFunc(tc.r.httpHandleWithVersion(tc.apiVersion, tc.r.handleTraces)),
			)

			// send traces to that endpoint without a content-type
			data, err := json.Marshal(tc.traces)
			assert.Nil(err)
			req, err := http.NewRequest("POST", server.URL, bytes.NewBuffer(data))
			assert.Nil(err)
			req.Header.Set("Content-Type", tc.contentType)

			client := &http.Client{}
			resp, err := client.Do(req)
			assert.Nil(err)
			assert.Equal(200, resp.StatusCode)

			// now we should be able to read the trace data
			select {
			case rt := <-tc.r.traces:
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

			resp.Body.Close()
			server.Close()
		})
	}
}

func TestReceiverMsgpackDecoder(t *testing.T) {
	// testing traces without content-type in agent endpoints, it should use Msgpack decoding
	// or it should raise a 415 Unsupported media type
	assert := assert.New(t)
	config := config.NewDefaultAgentConfig()
	testCases := []struct {
		name        string
		r           *HTTPReceiver
		apiVersion  APIVersion
		contentType string
		traces      model.Traces
	}{
		{"v01 with application/msgpack", NewHTTPReceiver(config), v01, "application/msgpack", fixtures.GetTestTrace(1, 1)},
		{"v02 with application/msgpack", NewHTTPReceiver(config), v02, "application/msgpack", fixtures.GetTestTrace(1, 1)},
		{"v03 with application/msgpack", NewHTTPReceiver(config), v03, "application/msgpack", fixtures.GetTestTrace(1, 1)},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(tc.name), func(t *testing.T) {
			// start testing server
			server := httptest.NewServer(
				http.HandlerFunc(tc.r.httpHandleWithVersion(tc.apiVersion, tc.r.handleTraces)),
			)

			// send traces to that endpoint using the msgpack content-type
			var buf bytes.Buffer
			err := msgp.Encode(&buf, tc.traces)
			assert.Nil(err)
			req, err := http.NewRequest("POST", server.URL, &buf)
			assert.Nil(err)
			req.Header.Set("Content-Type", tc.contentType)

			client := &http.Client{}
			resp, err := client.Do(req)
			assert.Nil(err)

			switch tc.apiVersion {
			case v01:
				assert.Equal(415, resp.StatusCode)
			case v02:
				assert.Equal(415, resp.StatusCode)
			case v03:
				assert.Equal(200, resp.StatusCode)

				// now we should be able to read the trace data
				select {
				case rt := <-tc.r.traces:
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
			}

			resp.Body.Close()
			server.Close()
		})
	}
}

func TestReceiverServiceJSONDecoder(t *testing.T) {
	// testing traces without content-type in agent endpoints, it should use JSON decoding
	assert := assert.New(t)
	config := config.NewDefaultAgentConfig()
	testCases := []struct {
		name        string
		r           *HTTPReceiver
		apiVersion  APIVersion
		contentType string
	}{
		{"v01 with empty content-type", NewHTTPReceiver(config), v01, ""},
		{"v02 with empty content-type", NewHTTPReceiver(config), v02, ""},
		{"v03 with empty content-type", NewHTTPReceiver(config), v03, ""},
		{"v01 with application/json", NewHTTPReceiver(config), v01, "application/json"},
		{"v02 with application/json", NewHTTPReceiver(config), v02, "application/json"},
		{"v03 with application/json", NewHTTPReceiver(config), v03, "application/json"},
		{"v01 with text/json", NewHTTPReceiver(config), v01, "text/json"},
		{"v02 with text/json", NewHTTPReceiver(config), v02, "text/json"},
		{"v03 with text/json", NewHTTPReceiver(config), v03, "text/json"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(tc.name), func(t *testing.T) {
			// start testing server
			server := httptest.NewServer(
				http.HandlerFunc(tc.r.httpHandleWithVersion(tc.apiVersion, tc.r.handleServices)),
			)

			// send service to that endpoint using the JSON content-type
			services := model.ServicesMetadata{
				"backend": map[string]string{
					"app":      "django",
					"app_type": "web",
				},
				"database": map[string]string{
					"app":      "postgres",
					"app_type": "db",
				},
			}

			data, err := json.Marshal(services)
			assert.Nil(err)
			req, err := http.NewRequest("POST", server.URL, bytes.NewBuffer(data))
			assert.Nil(err)
			req.Header.Set("Content-Type", tc.contentType)

			client := &http.Client{}
			resp, err := client.Do(req)
			assert.Nil(err)

			assert.Equal(200, resp.StatusCode)

			// now we should be able to read the trace data
			select {
			case rt := <-tc.r.services:
				assert.Len(rt, 2)
				assert.Equal(rt["backend"]["app"], "django")
				assert.Equal(rt["backend"]["app_type"], "web")
				assert.Equal(rt["database"]["app"], "postgres")
				assert.Equal(rt["database"]["app_type"], "db")
			default:
				t.Fatalf("no data received")
			}

			resp.Body.Close()
			server.Close()
		})
	}
}

func TestReceiverServiceMsgpackDecoder(t *testing.T) {
	// testing traces without content-type in agent endpoints, it should use Msgpack decoding
	// or it should raise a 415 Unsupported media type
	assert := assert.New(t)
	config := config.NewDefaultAgentConfig()
	testCases := []struct {
		name        string
		r           *HTTPReceiver
		apiVersion  APIVersion
		contentType string
	}{
		{"v01 with application/msgpack", NewHTTPReceiver(config), v01, "application/msgpack"},
		{"v02 with application/msgpack", NewHTTPReceiver(config), v02, "application/msgpack"},
		{"v03 with application/msgpack", NewHTTPReceiver(config), v03, "application/msgpack"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf(tc.name), func(t *testing.T) {
			// start testing server
			server := httptest.NewServer(
				http.HandlerFunc(tc.r.httpHandleWithVersion(tc.apiVersion, tc.r.handleServices)),
			)

			// send service to that endpoint using the JSON content-type
			services := model.ServicesMetadata{
				"backend": map[string]string{
					"app":      "django",
					"app_type": "web",
				},
				"database": map[string]string{
					"app":      "postgres",
					"app_type": "db",
				},
			}

			// send traces to that endpoint using the Msgpack content-type
			var buf bytes.Buffer
			err := msgp.Encode(&buf, services)
			assert.Nil(err)
			req, err := http.NewRequest("POST", server.URL, &buf)
			assert.Nil(err)
			req.Header.Set("Content-Type", tc.contentType)

			client := &http.Client{}
			resp, err := client.Do(req)
			assert.Nil(err)

			switch tc.apiVersion {
			case v01:
				assert.Equal(415, resp.StatusCode)
			case v02:
				assert.Equal(415, resp.StatusCode)
			case v03:
				assert.Equal(200, resp.StatusCode)

				// now we should be able to read the trace data
				select {
				case rt := <-tc.r.services:
					assert.Len(rt, 2)
					assert.Equal(rt["backend"]["app"], "django")
					assert.Equal(rt["backend"]["app_type"], "web")
					assert.Equal(rt["database"]["app"], "postgres")
					assert.Equal(rt["database"]["app_type"], "db")
				default:
					t.Fatalf("no data received")
				}
			}

			resp.Body.Close()
			server.Close()
		})
	}
}

func BenchmarkHandleTraces(b *testing.B) {
	// prepare the payload
	// msgpack payload
	var buf bytes.Buffer
	msgp.Encode(&buf, fixtures.GetTestTrace(1, 1))

	// prepare the receiver
	config := config.NewDefaultAgentConfig()
	receiver := NewHTTPReceiver(config)

	// response recorder
	handler := http.HandlerFunc(receiver.httpHandleWithVersion(v03, receiver.handleTraces))

	// benchmark
	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		// consume the traces channel without doing anything
		select {
		case <-receiver.traces:
		default:
		}

		// forge the request
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v0.3/traces", bytes.NewReader(buf.Bytes()))
		req.Header.Set("Content-Type", "application/msgpack")

		// trace only this execution
		b.StartTimer()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkDecoderJSON(b *testing.B) {
	assert := assert.New(b)
	traces := fixtures.GetTestTrace(150, 66)

	// json payload
	payload, err := json.Marshal(traces)
	assert.Nil(err)

	// benchmark
	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		reader := bytes.NewReader(payload)

		b.StartTimer()
		var spans model.Traces
		decoder := json.NewDecoder(reader)
		_ = decoder.Decode(&spans)
	}
}

func BenchmarkDecoderMsgpack(b *testing.B) {
	assert := assert.New(b)

	// msgpack payload
	var buf bytes.Buffer
	err := msgp.Encode(&buf, fixtures.GetTestTrace(150, 66))
	assert.Nil(err)

	// benchmark
	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		reader := bytes.NewReader(buf.Bytes())

		b.StartTimer()
		var traces model.Traces
		_ = msgp.Decode(reader, &traces)
	}
}
