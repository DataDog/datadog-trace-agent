package main

import (
	"regexp"
	"strings"
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

// Quantizer generates meaningul resource for spans
type Quantizer struct {
	in        chan model.Span
	out       chan model.Span
	exit      chan struct{}
	exitGroup *sync.WaitGroup
}

const (
	sqlType                = "sql"
	sqlVariableReplacement = "?"
)

var sqlCommentName = regexp.MustCompile("^-- ([^\n]*)")
var sqlVariablesRegexp = regexp.MustCompile("('[^']+')|([0-9]+)")
var sqlalchemyVariablesRegexp = regexp.MustCompile("%\\(.+?\\)s")
var sqlListRegexp = regexp.MustCompile("('[^']+')|([0-9]+)")
var sqlListVariables = regexp.MustCompile("\\?[\\? ,]+\\?")

// NewQuantizer creates a new Quantizer
func NewQuantizer(inSpans chan model.Span, exit chan struct{}, exitGroup *sync.WaitGroup) (*Quantizer, chan model.Span) {
	q := Quantizer{
		in:        inSpans,
		out:       make(chan model.Span),
		exit:      exit,
		exitGroup: exitGroup,
	}
	return &q, q.out
}

// Start runs the Quantizer by quantizing spans from the channel
func (q *Quantizer) Start() {
	go func() {
		for span := range q.in {
			q.out <- q.Quantize(span)
		}
	}()

	q.exitGroup.Add(1)
	go func() {
		<-q.exit
		log.Info("Quantizer exiting")
		close(q.in)
		q.exitGroup.Done()
		return
	}()

	log.Info("Quantizer started")
}

// Quantize generates meaningul resource for a span, depending on its type
func (q *Quantizer) Quantize(span model.Span) model.Span {
	if span.Type == sqlType {
		return q.QuantizeSQL(span)
	}
	log.Debugf("No quantization for this span, Type: %s", span.Type)

	return span
}

// QuantizeSQL generates resource for SQL spans
func (q *Quantizer) QuantizeSQL(span model.Span) model.Span {
	query, ok := span.Meta["query"]
	if !ok {
		log.Debugf("`query` meta is missing in a SQL span, can't quantize it, SpanID: %d", span.SpanID)
		return span
	}

	resource := strings.TrimSpace(query)

	if strings.HasPrefix(resource, "--") {
		log.Debugf("Quantize SQL command based on its comment, SpanID: %d", span.SpanID)
		resource = sqlCommentName.FindStringSubmatch(resource)[1]
		resource = strings.TrimSpace(resource)
	} else {
		log.Debugf("Quantize SQL command with generic parsing, SpanID: %d", span.SpanID)
		// Remove variables
		resource = sqlVariablesRegexp.ReplaceAllString(resource, sqlVariableReplacement)
		resource = sqlalchemyVariablesRegexp.ReplaceAllString(resource, sqlVariableReplacement)

		// Deal with list of variables of arbitrary size
		resource = sqlListVariables.ReplaceAllString(resource, sqlVariableReplacement)
	}

	span.Resource = resource

	return span
}
