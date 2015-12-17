package model

import (
	"net"
	"os"
	"strings"
)

// Node is a node of the graph: only host + section for now
type Node struct {
	Host    string
	Section string
}

// Edge is an edge of the graph: relation between 2 nodes with a type
type Edge struct {
	From  Node
	To    Node
	Type  string
	OrgID int32
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
	}

	return ip, nil
}

// ExpandHosts is a method that lookup both From and To host.
// in case the From.Host is missing it's going to be filled with os.Hostname
func (e *Edge) ExpandHosts() {

	// we need at least To.Host to consider this Edge as a valid one
	// just return if it's missing - nothing here to be done
	if e.To.Host == "" {
		return
	}

	// fill the empty From.Host with os.Hostname
	if e.From.Host == "" {
		hostname, err := os.Hostname()
		if err != nil {
			e.From.Host = hostname
		} else {
			return
		}
	} else {
		from, err := lookupHost(e.From.Host)
		if err == nil {
			e.From.Host = from
		}
	}

	to, err := lookupHost(e.To.Host)
	if err == nil {
		e.To.Host = to
	}
}

// TODO: implement a real serialize/unserialize?
