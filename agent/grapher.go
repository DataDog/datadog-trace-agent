package main

import (
	"strings"
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
)

// Grapher builds a graph of all the components
type Grapher struct {
	inSpans    chan model.Span
	inPayloads chan model.AgentPayload // Trigger the flush of the grapher when stats + samples are received
	out        chan model.AgentPayload // Output the stats + samples + graph

	conf *config.AgentConfig

	graph map[string][]uint64

	mu sync.Mutex

	// exit channels used for synchronisation and sending stop signals
	exit      chan struct{}
	exitGroup *sync.WaitGroup
}

// Node is a node of the graph: only host + section for now
type Node struct {
	Host    string
	Section string
}

// Edge is an edge of the graph: relation between 2 nodes with a type
type Edge struct {
	From Node
	To   Node
	Type string
}

// Key returns a representation of the edge
func (e *Edge) Key() string {
	return strings.Join([]string{e.From.Host, e.From.Section, e.To.Host, e.To.Section, e.Type}, "|")
}

// NewGrapher creates a new empty grapher
func NewGrapher(
	inSpans chan model.Span, inPayloads chan model.AgentPayload, conf *config.AgentConfig, exit chan struct{}, exitGroup *sync.WaitGroup,
) *Grapher {

	return &Grapher{
		inSpans:    inSpans,
		inPayloads: inPayloads,
		out:        make(chan model.AgentPayload),

		conf: conf,

		graph: make(map[string][]uint64),

		exit:      exit,
		exitGroup: exitGroup,
	}
}

// Start runs the writer by consuming spans in a buffer and periodically
// flushing to the API
func (g *Grapher) Start() {
	g.exitGroup.Add(1)
	go g.run()

	log.Info("Grapher started")
}

// We rely on the concentrator ticker to flush periodically traces "aligning" on the buckets
// (it's not perfect, but we don't really care, traces of this stats bucket may arrive in the next flush)
func (g *Grapher) run() {
	for {
		select {
		case span := <-g.inSpans:
			g.AddSpan(span)
		case bucket := <-g.inPayloads:
			log.Info("Received a payload, initiating a graphing + flush")
			g.out <- g.FlushPayload(bucket)
		case <-g.exit:
			log.Info("Grapher exiting")
			g.exitGroup.Done()
			return
		}
	}
}

// AddSpan adds a span to the sampler internal momory
func (g *Grapher) AddSpan(s model.Span) {
	if s.Meta["in.host"] == "" && s.Meta["out.host"] == "" {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	edge := Edge{
		From: Node{Host: s.Meta["in.host"], Section: s.Meta["in.section"]},
		To:   Node{Host: s.Meta["out.host"], Section: s.Meta["out.section"]},
		Type: s.Type,
	}
	key := edge.Key()
	if _, ok := g.graph[key]; ok {
		g.graph[key] = append(g.graph[key], s.SpanID)
	} else {
		g.graph[key] = []uint64{s.SpanID}
	}
}

// FlushPayload adds the graph to the payload received
func (g *Grapher) FlushPayload(ap model.AgentPayload) model.AgentPayload {
	g.mu.Lock()
	defer g.mu.Unlock()

	ap.Graph = g.graph
	g.graph = make(map[string][]uint64)

	return ap
}
