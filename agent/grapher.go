// +build ignore

package main

import (
	"sync"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/config"
	"github.com/DataDog/raclette/model"
)

// Grapher builds a graph representation of all interating elements of the traced system
type Grapher struct {
	in      chan model.Span
	inEdges chan []model.Edge

	// output channel send a list of Spans mapped by the Edge.Key()
	out chan map[string][]uint64

	conf *config.AgentConfig

	graph map[string][]uint64

	mu sync.Mutex

	Worker
}

// NewGrapher creates a new empty grapher, ready to start
func NewGrapher(in chan model.Span, inEdges chan []model.Edge, conf *config.AgentConfig) *Grapher {

	g := &Grapher{
		in:      in,
		inEdges: inEdges,
		out:     make(chan map[string][]uint64),

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
		case edges := <-g.inEdges:
			g.addEdges(edges)
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
	edge := model.Edge{
		From: model.Node{Host: s.Meta["in.host"], Section: s.Meta["in.section"]},
		To:   model.Node{Host: s.Meta["out.host"], Section: s.Meta["out.section"]},
		Type: s.Type,
	}

	// Expand Edge hosts
	edge.ExpandHosts()

	key := edge.Key()
	if _, ok := g.graph[key]; ok {
		g.graph[key] = append(g.graph[key], s.SpanID)
	} else {
		g.graph[key] = []uint64{s.SpanID}
	}
}

func (g *Grapher) addEdges(edges []model.Edge) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, edge := range edges {
		key := edge.Key()
		if _, ok := g.graph[key]; !ok {
			g.graph[key] = nil
		}
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
