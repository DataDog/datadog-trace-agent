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
	exit      chan bool
	exitGroup *sync.WaitGroup
}

const (
	SQLType                = "sql"
	SQLVariableReplacement = "?"
)

var sqlCommentName = regexp.MustCompile("^-- ([^\n]*)")
var sqlVariablesRegexp = regexp.MustCompile("('[^']+')|([0-9]+)")
var sqlalchemyVariablesRegexp = regexp.MustCompile("%\\(.+?\\)s")
var sqlListRegexp = regexp.MustCompile("('[^']+')|([0-9]+)")
var sqlListVariables = regexp.MustCompile("\\?[\\? ,]+\\?")

// NewQuantizer creates a new Quantizer
func NewQuantizer() *Quantizer {
	return &Quantizer{}
}

// Init initializes the Quantizer with input/output
func (q *Quantizer) Init(in chan model.Span, out chan model.Span, exit chan bool, exitGroup *sync.WaitGroup) {
	q.in = in
	q.out = out
	q.exit = exit
	q.exitGroup = exitGroup
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
		q.exitGroup.Done()
		close(q.in)
		return
	}()

	log.Info("Quantizer started")
}

// Quantize generates meaningul resource for a span, depending on its type
func (q *Quantizer) Quantize(span model.Span) model.Span {
	if span.Type == SQLType {
		return q.QuantizeSQL(span)
	} else {
		log.Debug("No quantization for this span")
		return span
	}
}

// QuantizeSQL generates resource for SQL spans
func (q *Quantizer) QuantizeSQL(span model.Span) model.Span {
	query, ok := span.Meta["query"]
	if !ok {
		log.Infof("`query` meta is missing in a SQL span, can't quantize it, SpanID: %d", span.SpanID)
		return span
	}

	resource := strings.TrimSpace(query)

	if strings.HasPrefix(resource, "--") {
		log.Infof("Quantize SQL command based on its comment, SpanID: %d", span.SpanID)
		resource = sqlCommentName.FindStringSubmatch(resource)[1]
		resource = strings.TrimSpace(resource)
	} else {
		log.Infof("Quantize SQL command with generic parsing, SpanID: %d", span.SpanID)
		// Remove variables
		resource = sqlVariablesRegexp.ReplaceAllString(resource, SQLVariableReplacement)
		resource = sqlalchemyVariablesRegexp.ReplaceAllString(resource, SQLVariableReplacement)

		// Deal with list of variables of arbitrary size
		resource = sqlListVariables.ReplaceAllString(resource, SQLVariableReplacement)
	}

	span.Resource = resource

	return span
}
