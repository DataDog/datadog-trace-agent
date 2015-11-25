package model

import (
	"strings"
)

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

// TODO: implement a real serialize/unserialize?
