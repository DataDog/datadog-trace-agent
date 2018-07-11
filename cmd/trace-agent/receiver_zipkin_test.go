package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestZipkinReceiver(t *testing.T) {
	type test struct {
		status     int
		data       []byte
		traceCount int
		spanCount  int
		gzip       bool
		err        string
	}
	testSuite := map[string]test{
		"empty": {
			status: http.StatusOK,
			data:   []byte(`[]`),
		},
		"invalid-json": {
			status: http.StatusBadRequest,
			data:   []byte(`FOO BAR`),
			err:    "looking for beginning",
		},
	}
	testdata, err := ioutil.ReadFile("./testdata/zipkin_spans.json")
	if err == nil && len(testdata) > 0 {
		// add the file as a test, both compressed and uncompressed
		testSuite["zipkin-spans.json"] = test{
			status:     http.StatusOK,
			data:       testdata,
			traceCount: 1,
			spanCount:  8,
		}
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		_, err := w.Write(testdata)
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
		testSuite["zipkin-spans.json.gzip"] = test{
			status:     http.StatusOK,
			data:       buf.Bytes(),
			gzip:       true,
			traceCount: 1,
			spanCount:  8,
		}
	} else {
		t.Logf("skipping ./testdata/zipkin_spans.json: %v", err)
	}

	conf := NewTestReceiverConfig()
	receiver := NewTestReceiverFromConfig(conf)

	defaultMux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()

	go receiver.Run()
	defer func() {
		receiver.Stop()
		http.DefaultServeMux = defaultMux
	}()

	url := fmt.Sprintf("http://%s:%d/zipkin/v2/spans", conf.ReceiverHost, conf.ReceiverPort)

	for testname, tt := range testSuite {
		t.Run(testname, func(t *testing.T) {
			assert := assert.New(t)
			req, err := http.NewRequest("POST", url, bytes.NewReader(tt.data))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Add("Content-Type", "application/json")
			if tt.gzip {
				req.Header.Add("Content-Encoding", "gzip")
			}
			resp, err := http.DefaultClient.Do(req)
			if tt.err == "" {
				assert.NoError(err)
			} else {
				assert.Equal(tt.status, resp.StatusCode)
				return
			}
			assert.Equal(tt.status, resp.StatusCode)
			slurp, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			assert.NoError(err)
			assert.Equal(fmt.Sprintf("OK:%d:%d", tt.traceCount, tt.spanCount), string(slurp))
		})
	}
}
