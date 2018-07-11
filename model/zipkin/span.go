package zipkin

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/DataDog/datadog-trace-agent/model"
)

// unmarshal errors
var (
	ErrValidTraceIDRequired  = errors.New("valid traceId required")
	ErrValidIDRequired       = errors.New("valid span id required")
	ErrValidDurationRequired = errors.New("valid duration required")
)

// SpanContext holds the context of a Span.
type SpanContext struct {
	TraceID  TraceID `json:"traceId"`
	ID       ID      `json:"id"`
	ParentID *ID     `json:"parentId,omitempty"`
	Debug    bool    `json:"debug,omitempty"`
	Sampled  *bool   `json:"-"`
	Err      error   `json:"-"`
}

// SpanModel structure.
//
// If using this library to instrument your application you will not need to
// directly access or modify this representation. The SpanModel is exported for
// use cases involving 3rd party Go instrumentation libraries desiring to
// export data to a Zipkin server using the Zipkin V2 Span model.
type SpanModel struct {
	SpanContext
	Name           string            `json:"name,omitempty"`
	Kind           Kind              `json:"kind,omitempty"`
	Timestamp      time.Time         `json:"timestamp,omitempty"`
	Duration       time.Duration     `json:"duration,omitempty"`
	Shared         bool              `json:"shared,omitempty"`
	LocalEndpoint  *Endpoint         `json:"localEndpoint,omitempty"`
	RemoteEndpoint *Endpoint         `json:"remoteEndpoint,omitempty"`
	Annotations    []Annotation      `json:"annotations,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
}

// MarshalJSON exports our Model into the correct format for the Zipkin V2 API.
func (s SpanModel) MarshalJSON() ([]byte, error) {
	type Alias SpanModel

	var timestamp int64
	if !s.Timestamp.IsZero() {
		if s.Timestamp.Unix() < 1 {
			// Zipkin does not allow Timestamps before Unix epoch
			return nil, ErrValidTimestampRequired
		}
		timestamp = s.Timestamp.Round(time.Microsecond).UnixNano() / 1e3
	}

	if s.Duration < time.Microsecond {
		if s.Duration < 0 {
			// negative duration is not allowed and signals a timing logic error
			return nil, ErrValidDurationRequired
		} else if s.Duration > 0 {
			// sub microsecond durations are reported as 1 microsecond
			s.Duration = 1 * time.Microsecond
		}
	} else {
		// Duration will be rounded to nearest microsecond representation.
		//
		// NOTE: Duration.Round() is not available in Go 1.8 which we still support.
		// To handle microsecond resolution rounding we'll add 500 nanoseconds to
		// the duration. When truncated to microseconds in the call to marshal, it
		// will be naturally rounded. See TestSpanDurationRounding in span_test.go
		s.Duration += 500 * time.Nanosecond
	}

	if s.LocalEndpoint.Empty() {
		s.LocalEndpoint = nil
	}

	if s.RemoteEndpoint.Empty() {
		s.RemoteEndpoint = nil
	}

	return json.Marshal(&struct {
		Timestamp int64 `json:"timestamp,omitempty"`
		Duration  int64 `json:"duration,omitempty"`
		Alias
	}{
		Timestamp: timestamp,
		Duration:  s.Duration.Nanoseconds() / 1e3,
		Alias:     (Alias)(s),
	})
}

// UnmarshalJSON imports our Model from a Zipkin V2 API compatible span
// representation.
func (s *SpanModel) UnmarshalJSON(b []byte) error {
	type Alias SpanModel
	span := &struct {
		TimeStamp uint64 `json:"timestamp,omitempty"`
		Duration  uint64 `json:"duration,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(b, &span); err != nil {
		return err
	}
	if s.ID < 1 {
		return ErrValidIDRequired
	}
	if span.TimeStamp > 0 {
		s.Timestamp = time.Unix(0, int64(span.TimeStamp)*1e3)
	}
	s.Duration = time.Duration(span.Duration*1e3) * time.Nanosecond
	if s.LocalEndpoint.Empty() {
		s.LocalEndpoint = nil
	}

	if s.RemoteEndpoint.Empty() {
		s.RemoteEndpoint = nil
	}
	return nil
}

// Convert converts the Zipkin span to a Datadog span.
func (zspan *SpanModel) Convert() *model.Span {
	span := model.Span{
		Name:     zspan.Name,
		Resource: zspan.Name,
		TraceID:  zspan.TraceID.Low,
		SpanID:   uint64(zspan.ID),
		Start:    zspan.Timestamp.UnixNano(),
		Duration: int64(zspan.Duration),
		Meta:     map[string]string{},
		Metrics:  map[string]float64{"_sampling_priority_v1": 2},
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
				span.Metrics["_sampling_priority_v1"] = float64(n)
			}
		default:
			span.Meta[k] = v
		}
	}
	if span.Type == "" {
		switch zspan.Kind {
		case Producer, Consumer:
			span.Type = "queue"
		case Client:
			if hasAnyOfTags(&span, "sql.query") {
				span.Type = "sql"
			}
			if hasAnyOfTags(&span, "cassandra.query") {
				span.Type = "cassandra"
			}
			if hasAnyOfTags(&span, "http.path", "http.uri") {
				span.Type = "http"
			}
		case Server:
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
	return &span
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
