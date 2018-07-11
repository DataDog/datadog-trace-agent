package main

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"

	"github.com/DataDog/datadog-trace-agent/info"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/model/zipkin"

	log "github.com/cihub/seelog"
)

// tagZipkinHandler is the metrics tag used for the Zipkin span handler.
const tagZipkinHandler = "handler:zipkin"

// handleZipkinSpans handles the endpoint accepting Zipkin spans.
func (r *HTTPReceiver) handleZipkinSpans(w http.ResponseWriter, req *http.Request) {
	switch v := req.Header.Get("Content-Type"); v {
	case "application/json", "text/json":
		// OK
	default:
		// unsupported Content-Type
		log.Errorf("/zipkin/v2/spans: unsupported media type %q", v)
		HTTPFormatError([]string{tagZipkinHandler}, w)
		return
	}
	var zipkinSpans []*zipkin.SpanModel
	reader := req.Body
	defer req.Body.Close()
	if enc := req.Header.Get("Content-Encoding"); enc != "" {
		// is the request body gzip encoded?
		if enc != "gzip" {
			log.Errorf("/zipkin/v2/spans: unsupported Content-Encoding: %s", enc)
			HTTPDecodingError(errors.New("unsupported Content-Encoding"), []string{tagZipkinHandler}, w)
			return
		}
		var err error
		reader, err = gzip.NewReader(reader)
		if err != nil {
			log.Errorf("/zipkin/v2/spans: error reading gzipped content")
			HTTPDecodingError(err, []string{tagZipkinHandler}, w)
			return
		}
		defer reader.Close()
	}
	if err := json.NewDecoder(reader).Decode(&zipkinSpans); err != nil {
		log.Errorf("/zipkin/v2/spans: cannot decode traces payload: %v", err)
		HTTPDecodingError(err, []string{tagZipkinHandler}, w)
		return
	}

	traces := tracesFromZipkinSpans(zipkinSpans)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK:%d:%d", len(traces), len(zipkinSpans))

	tags := info.Tags{
		Lang:          "unknown",
		LangVersion:   "unknown",
		Interpreter:   "unknown",
		TracerVersion: "zipkin.v2",
	}
	var size int64
	lr, ok := req.Body.(*model.LimitedReader)
	if ok {
		size = lr.Count
	}
	r.receiveTraces(traces, tags, size)
}

// tracesFromZipkinSpans creates Traces from a set of Zipkin spans.
func tracesFromZipkinSpans(zipkinSpans []*zipkin.SpanModel) model.Traces {
	// convert to Datadog spans
	spans := make([]model.Span, len(zipkinSpans))
	for i, zspan := range zipkinSpans {
		spans[i] = *zspan.Convert()
	}
	// group by TraceID
	traces := make(model.Traces, 0)
	byID := make(map[uint64][]*model.Span)
	seen := make(map[uint64]*model.Span)
	for _, s := range spans {
		if _, ok := seen[s.SpanID]; ok {
			// we have a duplicate SpanID, this is a case where the Zipkin server
			// normally merges spans together. As an example, this happens when a
			// client initiates a span that finishes on the server. Since Datadog
			// doesn't support such functionality, we'll keep the span and instead
			// generate a new SpanID for it to resolve the collision.
			s.SpanID = rand.Uint64()
		}
		seen[s.SpanID] = &s
		byID[s.TraceID] = append(byID[s.TraceID], &s)
	}
	for _, t := range byID {
		traces = append(traces, t)
	}
	return traces
}
