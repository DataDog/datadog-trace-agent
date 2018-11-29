//go:generate msgp -marshal=false -o=span_msgp.go -tests=false
//msgp:ignore SpanOpts

package test

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/tinylib/msgp/msgp"
)

var (
	_ msgp.Encodable = (*TraceList)(nil)
	_ msgp.Encodable = (*Trace)(nil)
	_ msgp.Encodable = (*Span)(nil)
)

type (
	Trace     []*Span
	TraceList []Trace
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Span struct {
	SpanID   uint64             `msg:"span_id"`
	TraceID  uint64             `msg:"trace_id"`
	ParentID uint64             `msg:"parent_id"`
	Start    int64              `msg:"start"`
	Duration int64              `msg:"duration"`
	Meta     map[string]string  `msg:"meta,omitempty"`
	Metrics  map[string]float64 `msg:"metrics,omitempty"`
	Error    int32              `msg:"error"`
	Name     string             `msg:"name"`
	Service  string             `msg:"service"`
	Resource string             `msg:"resource"`
	Type     string             `msg:"type"`
}

type SpanOpts struct {
	Parent *Span

	Name     string
	Service  string
	Resource string
	Type     string
	Start    time.Time
	Duration time.Duration
	Tags     map[string]interface{} // strings or float64's
	Error    int32                  // 1 or 0
}

func NewSpan(opts *SpanOpts) *Span {
	if opts == nil {
		opts = &SpanOpts{}
	}
	var traceID, spanID, parentID uint64
	if opts.Parent != nil {
		parentID = opts.Parent.SpanID
		traceID = opts.Parent.TraceID
		spanID = rand.Uint64()
	} else {
		traceID = rand.Uint64()
		spanID = traceID
	}
	var start time.Time
	if !opts.Start.IsZero() {
		start = opts.Start
	} else {
		start = time.Now()
	}
	duration := 500 * time.Millisecond
	if opts.Duration != 0 {
		duration = opts.Duration
	}
	var name string
	if opts.Name != "" {
		name = opts.Name
	} else {
		name = randString(names)
	}
	var resource string
	if opts.Resource != "" {
		resource = opts.Resource
	} else {
		resource = randString(resources)
	}
	var typ string
	if opts.Type != "" {
		typ = opts.Type
	} else {
		typ = randString(types)
	}
	var service string
	if opts.Service != "" {
		service = opts.Service
	} else {
		service = randString(services)
	}

	span := &Span{
		TraceID:  traceID,
		SpanID:   spanID,
		ParentID: parentID,
		Start:    start.UnixNano(),
		Duration: int64(duration),
		Meta:     map[string]string{},
		Metrics:  map[string]float64{"_sampling_priority_v1": 2, "_sample_rate": 1},
		Error:    opts.Error,
		Name:     name,
		Service:  service,
		Resource: resource,
		Type:     typ,
	}

	for key, val := range opts.Tags {
		switch v := val.(type) {
		case float64:
			span.Metrics[key] = v
		case string:
			span.Meta[key] = v
		default:
			span.Meta[key] = fmt.Sprint(v)
		}
	}

	return span
}

var (
	names     = []string{"http.request", "sql.query", "redis.query", "file.save"}
	services  = []string{"mozarella", "marzano", "droppy", "tolstoy", "farina"}
	resources = []string{"/", "GET /index.html", "SELECT * FROM updates", "INSERT INTO crane", "DELETE db"}
	types     = []string{"web", "http", "db", "rpc"}
)

func randString(opts []string) string { return opts[rand.Intn(len(opts))] }
