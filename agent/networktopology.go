package main

import (
	"encoding/json"
	log "github.com/cihub/seelog"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/DataDog/raclette/model"
)

var ssOutputRegexp = regexp.MustCompile(`(?m)^ESTAB\s+\d+\s+\d+\s+(?P<localAddr>[^:]+):(?P<localPort>\d+)\s+(?P<remoteAddr>[^:]+):(?P<remotePort>\d+)(\s+)?$`)

// NetworkTopology generates meaningul resource for spans
type NetworkTopology struct {
	out chan []model.Edge

	Worker
}

// NewNetworkTopology creates a new NetworkTopology ready to be started
func NewNetworkTopology() *NetworkTopology {
	q := &NetworkTopology{
		out: make(chan []model.Edge),
	}
	q.Init()
	return q
}

// Start runs the NetworkTopology by quantizing spans from the channel
func (q *NetworkTopology) Start() {
	go func() {
		for range time.Tick(time.Second * 2) {
			edges, err := q.getTCPstats()
			if err != nil {
				log.Error(err)
				continue
			}
			q.out <- edges
		}
	}()

	log.Info("NetworkTopology started")
}

func buildEdge(from string, to string) (edge model.Edge, err error) {
	if err := json.NewDecoder(strings.NewReader(from)).Decode(&edge.From); err != nil {
		return edge, err
	}

	if err = json.NewDecoder(strings.NewReader(to)).Decode(&edge.To); err != nil {
		return edge, err
	}
	return edge, nil
}

func (q *NetworkTopology) getTCPstats() ([]model.Edge, error) {
	cmd := exec.Command("/bin/ss", "-rt4", "not", "src", "localhost", "and", "not", "dst", "localhost")
	stdout, err := cmd.Output()
	var from, to []byte
	var edges = make([]model.Edge, 0)

	// something went wrong, drop it like it's hot!
	if err != nil {
		return nil, err
	}

	// find all matching lines and expand them into json-like string
	for _, s := range ssOutputRegexp.FindAllSubmatchIndex(stdout, -1) {
		from = ssOutputRegexp.Expand([]byte{}, []byte(`{"Host": "$localAddr", "Section": "$localPort"}`), stdout, s)
		to = ssOutputRegexp.Expand([]byte{}, []byte(`{"Host": "$remoteAddr", "Section": "$remotePort"}`), stdout, s)

		edge, err := buildEdge(string(from), string(to))
		if err != nil {
			continue
		}

		edges = append(edges, edge)
	}

	log.Infof("NetworkTopology reported %d edges", len(edges))

	return edges, nil
}
