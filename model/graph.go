package model

import (
	"net"
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

// LookupHost tries to resolve Node's Host
func (n *Node) LookupHost() (string, error) {
	hostname, err := net.LookupHost(n.Host)
	if err != nil {
		return "", err
	}

	// return the first host from the list
	if len(hostname) > 0 {
		return hostname[0], nil
	} else {
		return n.Host, nil
	}
}

// TODO: implement a real serialize/unserialize?
