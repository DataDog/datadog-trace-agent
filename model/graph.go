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

func lookupHost(ip string) (string, error) {
	hostname, err := net.LookupHost(ip)
	if err != nil {
		return "", err
	}

	// return the first host from the list
	if len(hostname) > 0 {
		return hostname[0], nil
	} else {
		return ip, nil
	}
}

// LookupHost tries to resolve Node's Host
func (e *Edge) LookupHosts() {

	// make sure both From and To has a Host filled
	if len(e.From.Host) < 1 || len(e.To.Host) < 1 {
		return
	}

	from, err := lookupHost(e.From.Host)
	if err == nil {
		e.From.Host = from
	}

	to, err := lookupHost(e.To.Host)
	if err == nil {
		e.To.Host = to
	}
}

// TODO: implement a real serialize/unserialize?
