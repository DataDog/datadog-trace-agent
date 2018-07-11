package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

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
		log.Errorf("/zipkin/v2/spans: unsupported media type %q", v)
		HTTPFormatError([]string{tagZipkinHandler}, w)
		return
	}
	var zipkinSpans []*zipkin.SpanModel
	if err := json.NewDecoder(req.Body).Decode(&zipkinSpans); err != nil {
		log.Errorf("/zipkin/v2/spans: cannot decode traces payload: %v", err)
		HTTPDecodingError(err, []string{tagZipkinHandler}, w)
		return
	}

	spans := convertZipkinSpans(zipkinSpans)
	traces := model.TracesFromSpans(spans)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK:%d:%d", len(traces), len(spans))

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

// convertZipkinSpans returns a set of Datadog spans from a set of Zipkin spans.
func convertZipkinSpans(zipkinSpans []*zipkin.SpanModel) []model.Span {
	spans := make([]model.Span, len(zipkinSpans))
	for i, zspan := range zipkinSpans {
		span := model.Span{
			Name:     zspan.Name,
			Resource: zspan.Name,
			TraceID:  zspan.TraceID.Low,
			SpanID:   uint64(zspan.ID),
			Start:    zspan.Timestamp.UnixNano(),
			Duration: int64(zspan.Duration),
			Meta:     map[string]string{},
			Metrics:  map[string]float64{samplingPriorityKey: 2},
		}
		if zspan.ParentID != nil {
			span.ParentID = uint64(*zspan.ParentID)
		}
		if zspan.Err != nil {
			span.Error = 1
			span.Meta["error.msg"] = zspan.Err.Error()
		}
		for k, v := range zspan.Tags {
			switch k {
			case "service.name":
				span.Service = v
			case "resource.name":
				span.Resource = v
			case "span.type":
				span.Type = v
			case "sampling.priority":
				if n, err := strconv.Atoi(v); err == nil {
					span.Metrics[samplingPriorityKey] = float64(n)
				}
			default:
				span.Meta[k] = v
			}
		}
		if span.Type == "" {
			switch zspan.Kind {
			case zipkin.Producer, zipkin.Consumer:
				span.Type = "queue"
			case zipkin.Client:
				if hasAnyOfTags(&span, "sql.query") {
					span.Type = "sql"
				}
				if hasAnyOfTags(&span, "cassandra.query") {
					span.Type = "cassandra"
				}
				if hasAnyOfTags(&span, "http.path", "http.uri") {
					span.Type = "http"
				}
			case zipkin.Server:
				if hasAnyOfTags(&span, "http.path", "http.uri") {
					span.Type = "web"
				}
			}
		}
		if e := zspan.LocalEndpoint; e != nil {
			if e.ServiceName != "" && span.Service == "" {
				// if this is the local service, it should be fair to
				// use it as the span's service name as a fallback
				span.Service = e.ServiceName
			}
			if e.IPv4 != nil {
				span.Meta["in.host"] = e.IPv4.String()
			}
			if e.Port != 0 {
				span.Meta["in.port"] = strconv.Itoa(int(e.Port))
			}
		}
		if e := zspan.RemoteEndpoint; e != nil {
			if e.ServiceName != "" {
				span.Meta["out.service"] = e.ServiceName
			}
			if e.IPv4 != nil {
				span.Meta["out.host"] = e.IPv4.String()
			}
			if e.Port != 0 {
				span.Meta["out.port"] = strconv.Itoa(int(e.Port))
			}
		}
		spans[i] = span
	}
	return spans
}

// hasAnyOfTags reports whether the given span has any of the listed tags.
func hasAnyOfTags(span *model.Span, tags ...string) bool {
	for _, tag := range tags {
		if _, ok := span.Meta[tag]; ok {
			return true
		}
	}
	return false
}
