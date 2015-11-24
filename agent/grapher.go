package main

import (
	"strings"
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
)

// Grapher builds a graph representation of all interating elements of the traced system
type Grapher struct {
	in  chan model.Span
	out chan map[string][]uint64

	conf *config.AgentConfig

	graph map[string][]uint64

	mu sync.Mutex

	Worker
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

// Key returns a serialized representation of the edge
func (e *Edge) Key() string {
	return strings.Join([]string{e.From.Host, e.From.Section, e.To.Host, e.To.Section, e.Type}, "|")
}

// NewGrapher creates a new empty grapher, ready to start
func NewGrapher(in chan model.Span, conf *config.AgentConfig) *Grapher {

	g := &Grapher{
		in:  in,
		out: make(chan map[string][]uint64),

		conf: conf,

		graph: make(map[string][]uint64),
	}
	g.Init()
	return g
}

// Start runs the Grapher which builds the graph with incoming spans and flushes it on demand
func (g *Grapher) Start() {
	go g.run()
	log.Info("Grapher started")
}

func (g *Grapher) run() {
	g.wg.Add(1)
	for {
		select {
		case span := <-g.in:
			if span.IsFlushMarker() {
				log.Debug("Grapher starts a flush")
				g.out <- g.Flush()
			} else {
				g.HandleSpan(span)
			}
		case <-g.exit:
			log.Info("Grapher exiting")
			close(g.out)
			g.wg.Done()
			return
		}
	}
}

// HandleSpan processes a span to extend our graph representation
func (g *Grapher) HandleSpan(s model.Span) {
	// If the span doesn't contain graph metadata, skip it
	if s.Meta["in.host"] == "" && s.Meta["out.host"] == "" {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Build the edge based on the span metadata
	// TODO: Hostname resolution
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

// Flush returns a graph representation and reset the Grapher state
func (g *Grapher) Flush() map[string][]uint64 {
	g.mu.Lock()
	graph := g.graph
	g.graph = make(map[string][]uint64)
	g.mu.Unlock()

	log.Debugf("Grapher flushes %d edges", len(graph))

	return graph
}
