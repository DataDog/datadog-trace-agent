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
	seen := make(map[zipkin.ID]*zipkin.SpanModel, len(zipkinSpans))
	for _, zspan := range zipkinSpans {
		if dup, ok := seen[zspan.ID]; ok {
			// We have a duplicate ID, this is a case where the Zipkin server
			// normally merges spans together. As an example, this happens when a
			// client initiates a span that finishes on the server. Since Datadog
			// doesn't accept such behaviour, we'll keep the span and instead
			// generate a new ID for it to resolve the collision.
			//
			// This can however still prove problematic when the duplicate span comes
			// in as part of a subsequent payload, in which case we will not be able
			// to detect it. The best way to avoid this behaviour is to use the readily
			// available Zipkin setting analogue to:
			//
			// https://godoc.org/github.com/openzipkin/zipkin-go#WithSharedSpans
			//
			// This is enabled by default in zipkin-go.
			zspan.ID = zipkin.ID(rand.Uint64())
			// these spans generally have the same ParentID too, so let's assume that
			// the span which was created first will become the new parent.
			if dup.ParentID == zspan.ParentID && dup.ParentID != nil {
				if zspan.Timestamp.Before(dup.Timestamp) {
					dup.ParentID = &zspan.ID
				} else {
					zspan.ParentID = &dup.ID
				}
			}
			seen[dup.ID] = dup
		}
		seen[zspan.ID] = zspan
	}
	// group by TraceID
	traces := make(model.Traces, 0)
	byID := make(map[uint64][]*model.Span)
	for _, zs := range seen {
		s := zs.Convert()
		byID[s.TraceID] = append(byID[s.TraceID], s)
	}
	for _, t := range byID {
		traces = append(traces, t)
	}
	return traces
}
