package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"stackstate-trace-agent/config"
	"stackstate-trace-agent/fixtures"
	"stackstate-trace-agent/info"
	"stackstate-trace-agent/model"
	"stackstate-trace-agent/sampler"
	"github.com/stretchr/testify/assert"
	"github.com/tinylib/msgp/msgp"
)

// Traces shouldn't come from more than 5 different sources
var langs = []string{"python", "ruby", "go", "java", "C#"}

// headerFields is a map used to decode the header metas
var headerFields = map[string]string{
	"lang":           "StackState-Meta-Lang",
	"lang_version":   "StackState-Meta-Lang-Version",
	"interpreter":    "StackState-Meta-Lang-Interpreter",
	"tracer_version": "StackState-Meta-Tracer-Version",
}

func NewTestReceiverFromConfig(conf *config.AgentConfig) *HTTPReceiver {
	dynConf := config.NewDynamicConfig()

	rawTraceChan := make(chan model.Trace, 5000)
	serviceChan := make(chan model.ServicesMetadata, 50)
	receiver := NewHTTPReceiver(conf, dynConf, rawTraceChan, serviceChan)

	return receiver
}

func NewTestReceiverConfig() *config.AgentConfig {
	conf := config.NewDefaultAgentConfig()
	conf.APIKey = "test"

	return conf
}

func TestReceiverRequestBodyLength(t *testing.T) {
	assert := assert.New(t)

	// save the global mux aside, we don't want to break other tests
	defaultMux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()

	conf := NewTestReceiverConfig()
	receiver := NewTestReceiverFromConfig(conf)
	receiver.maxRequestBodyLength = 2
	go receiver.Run()

	defer func() {
		receiver.Stop()
		http.DefaultServeMux = defaultMux
	}()

	url := fmt.Sprintf("http://%s:%d/v0.4/traces",
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
	conf := NewTestReceiverConfig()
	testCases := []struct {
		name        string
		r           *HTTPReceiver
		apiVersion  APIVersion
		contentType string
		traces      model.Trace
	}{
		{"v01 with empty content-type", NewTestReceiverFromConfig(conf), v01, "", model.Trace{fixtures.GetTestSpan()}},
		{"v01 with application/json", NewTestReceiverFromConfig(conf), v01, "application/json", model.Trace{fixtures.GetTestSpan()}},
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
	conf := NewTestReceiverConfig()
	testCases := []struct {
		name        string
		r           *HTTPReceiver
		apiVersion  APIVersion
		contentType string
		traces      []model.Trace
	}{
		{"v02 with empty content-type", NewTestReceiverFromConfig(conf), v02, "", fixtures.GetTestTrace(1, 1, false)},
		{"v03 with empty content-type", NewTestReceiverFromConfig(conf), v03, "", fixtures.GetTestTrace(1, 1, false)},
		{"v04 with empty content-type", NewTestReceiverFromConfig(conf), v04, "", fixtures.GetTestTrace(1, 1, false)},
		{"v02 with application/json", NewTestReceiverFromConfig(conf), v02, "application/json", fixtures.GetTestTrace(1, 1, false)},
		{"v03 with application/json", NewTestReceiverFromConfig(conf), v03, "application/json", fixtures.GetTestTrace(1, 1, false)},
		{"v04 with application/json", NewTestReceiverFromConfig(conf), v04, "application/json", fixtures.GetTestTrace(1, 1, false)},
		{"v02 with text/json", NewTestReceiverFromConfig(conf), v02, "text/json", fixtures.GetTestTrace(1, 1, false)},
		{"v03 with text/json", NewTestReceiverFromConfig(conf), v03, "text/json", fixtures.GetTestTrace(1, 1, false)},
		{"v04 with text/json", NewTestReceiverFromConfig(conf), v04, "text/json", fixtures.GetTestTrace(1, 1, false)},
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
	conf := NewTestReceiverConfig()
	testCases := []struct {
		name        string
		r           *HTTPReceiver
		apiVersion  APIVersion
		contentType string
		traces      model.Traces
	}{
		{"v01 with application/msgpack", NewTestReceiverFromConfig(conf), v01, "application/msgpack", fixtures.GetTestTrace(1, 1, false)},
		{"v02 with application/msgpack", NewTestReceiverFromConfig(conf), v02, "application/msgpack", fixtures.GetTestTrace(1, 1, false)},
		{"v03 with application/msgpack", NewTestReceiverFromConfig(conf), v03, "application/msgpack", fixtures.GetTestTrace(1, 1, false)},
		{"v04 with application/msgpack", NewTestReceiverFromConfig(conf), v04, "application/msgpack", fixtures.GetTestTrace(1, 1, false)},
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

				body, err := ioutil.ReadAll(resp.Body)
				assert.Nil(err)
				assert.Equal("OK\n", string(body))
			case v04:
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

				body, err := ioutil.ReadAll(resp.Body)
				assert.Nil(err)
				var tr traceResponse
				err = json.Unmarshal(body, &tr)
				assert.Nil(err, "the answer should be a valid JSON")
			}

			resp.Body.Close()
			server.Close()
		})
	}
}

func TestReceiverServiceJSONDecoder(t *testing.T) {
	// testing traces without content-type in agent endpoints, it should use JSON decoding
	assert := assert.New(t)
	conf := NewTestReceiverConfig()
	testCases := []struct {
		name        string
		r           *HTTPReceiver
		apiVersion  APIVersion
		contentType string
	}{
		{"v01 with empty content-type", NewTestReceiverFromConfig(conf), v01, ""},
		{"v02 with empty content-type", NewTestReceiverFromConfig(conf), v02, ""},
		{"v03 with empty content-type", NewTestReceiverFromConfig(conf), v03, ""},
		{"v04 with empty content-type", NewTestReceiverFromConfig(conf), v04, ""},
		{"v01 with application/json", NewTestReceiverFromConfig(conf), v01, "application/json"},
		{"v02 with application/json", NewTestReceiverFromConfig(conf), v02, "application/json"},
		{"v03 with application/json", NewTestReceiverFromConfig(conf), v03, "application/json"},
		{"v04 with application/json", NewTestReceiverFromConfig(conf), v04, "application/json"},
		{"v01 with text/json", NewTestReceiverFromConfig(conf), v01, "text/json"},
		{"v02 with text/json", NewTestReceiverFromConfig(conf), v02, "text/json"},
		{"v03 with text/json", NewTestReceiverFromConfig(conf), v03, "text/json"},
		{"v04 with text/json", NewTestReceiverFromConfig(conf), v04, "text/json"},
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
	conf := NewTestReceiverConfig()
	testCases := []struct {
		name        string
		r           *HTTPReceiver
		apiVersion  APIVersion
		contentType string
	}{
		{"v01 with application/msgpack", NewTestReceiverFromConfig(conf), v01, "application/msgpack"},
		{"v02 with application/msgpack", NewTestReceiverFromConfig(conf), v02, "application/msgpack"},
		{"v03 with application/msgpack", NewTestReceiverFromConfig(conf), v03, "application/msgpack"},
		{"v04 with application/msgpack", NewTestReceiverFromConfig(conf), v04, "application/msgpack"},
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

				body, err := ioutil.ReadAll(resp.Body)
				assert.Nil(err)
				assert.Equal("OK\n", string(body))
			case v04:
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

				body, err := ioutil.ReadAll(resp.Body)
				assert.Nil(err)
				assert.Equal("OK\n", string(body))
			}

			resp.Body.Close()
			server.Close()
		})
	}
}

func TestHandleTraces(t *testing.T) {
	assert := assert.New(t)

	// prepare the msgpack payload
	var buf bytes.Buffer
	msgp.Encode(&buf, fixtures.GetTestTrace(10, 10, true))

	// prepare the receiver
	conf := NewTestReceiverConfig()
	receiver := NewTestReceiverFromConfig(conf)

	// response recorder
	handler := http.HandlerFunc(receiver.httpHandleWithVersion(v04, receiver.handleTraces))

	for n := 0; n < 10; n++ {
		// consume the traces channel without doing anything
		select {
		case <-receiver.traces:
		default:
		}

		// forge the request
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v0.4/traces", bytes.NewReader(buf.Bytes()))
		req.Header.Set("Content-Type", "application/msgpack")

		// Add meta data to simulate data comming from multiple applications
		req.Header.Set("StackState-Meta-Lang", langs[n%len(langs)])

		handler.ServeHTTP(rr, req)
	}

	rs := receiver.stats
	assert.Equal(5, len(rs.Stats)) // We have a tagStats struct for each application

	// We test stats for each app
	for _, lang := range langs {
		ts, ok := rs.Stats[info.Tags{Lang: lang}]
		assert.True(ok)
		assert.Equal(int64(20), ts.TracesReceived)
		assert.Equal(int64(59222), ts.TracesBytes)
	}
	// make sure we have all our languages registered
	assert.Equal("C#|go|java|python|ruby", receiver.Languages())
}

// chunkedReader is a reader which forces partial reads, this is required
// to trigger some network related bugs, such as body not being read fully by server.
// Without this, all the data could be read/written at once, not triggering the issue.
type chunkedReader struct {
	reader io.Reader
}

func (sr *chunkedReader) Read(p []byte) (n int, err error) {
	size := 1024
	if size > len(p) {
		size = len(p)
	}
	buf := p[0:size]
	return sr.reader.Read(buf)
}

func TestReceiverPreSamplerCancel(t *testing.T) {
	assert := assert.New(t)

	var wg sync.WaitGroup
	var buf bytes.Buffer

	n := 100 // Payloads need to be big enough, else bug is not triggered
	msgp.Encode(&buf, fixtures.GetTestTrace(n, n, true))

	conf := NewTestReceiverConfig()
	conf.PreSampleRate = 0.000001 // Make sure we sample aggressively
	receiver := NewTestReceiverFromConfig(conf)

	server := httptest.NewServer(http.HandlerFunc(receiver.httpHandleWithVersion(v04, receiver.handleTraces)))

	defer server.Close()
	url := server.URL + "/v0.4/traces"

	// Make sure we use share clients, and they are reused.
	client := &http.Client{Transport: &http.Transport{
		MaxIdleConnsPerHost: 100,
	}}
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			for j := 0; j < 3; j++ {
				reader := &chunkedReader{reader: bytes.NewReader(buf.Bytes())}
				req, err := http.NewRequest("POST", url, reader)
				req.Header.Set("Content-Type", "application/msgpack")
				req.Header.Set(sampler.TraceCountHeader, strconv.Itoa(n))
				assert.Nil(err)

				resp, err := client.Do(req)
				assert.Nil(err)
				assert.NotNil(resp)
				if resp != nil {
					assert.Equal(http.StatusOK, resp.StatusCode)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkHandleTracesFromOneApp(b *testing.B) {
	// prepare the payload
	// msgpack payload
	var buf bytes.Buffer
	msgp.Encode(&buf, fixtures.GetTestTrace(1, 1, true))

	// prepare the receiver
	conf := NewTestReceiverConfig()
	receiver := NewTestReceiverFromConfig(conf)

	// response recorder
	handler := http.HandlerFunc(receiver.httpHandleWithVersion(v04, receiver.handleTraces))

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
		req, _ := http.NewRequest("POST", "/v0.4/traces", bytes.NewReader(buf.Bytes()))
		req.Header.Set("Content-Type", "application/msgpack")

		// Add meta data to simulate data comming from multiple applications
		for _, v := range headerFields {
			req.Header.Set(v, langs[n%len(langs)])
		}

		// trace only this execution
		b.StartTimer()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkHandleTracesFromMultipleApps(b *testing.B) {
	// prepare the payload
	// msgpack payload
	var buf bytes.Buffer
	msgp.Encode(&buf, fixtures.GetTestTrace(1, 1, true))

	// prepare the receiver
	conf := NewTestReceiverConfig()
	receiver := NewTestReceiverFromConfig(conf)

	// response recorder
	handler := http.HandlerFunc(receiver.httpHandleWithVersion(v04, receiver.handleTraces))

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
		req, _ := http.NewRequest("POST", "/v0.4/traces", bytes.NewReader(buf.Bytes()))
		req.Header.Set("Content-Type", "application/msgpack")

		// Add meta data to simulate data comming from multiple applications
		for _, v := range headerFields {
			req.Header.Set(v, langs[n%len(langs)])
		}

		// trace only this execution
		b.StartTimer()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkDecoderJSON(b *testing.B) {
	assert := assert.New(b)
	traces := fixtures.GetTestTrace(150, 66, true)

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
	err := msgp.Encode(&buf, fixtures.GetTestTrace(150, 66, true))
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
