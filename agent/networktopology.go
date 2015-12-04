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

func (q *NetworkTopology) getTCPstats() ([]model.Edge, error) {
	cmd := exec.Command("/bin/ss", "-rt4", "not", "src", "localhost", "and", "not", "dst", "localhost")
	stdout, err := cmd.Output()
	var edges = make([]model.Edge, 0)

	// something went wrong, drop it like it's hot!
	if err != nil {
		return nil, err
	}

	groupNames := ssOutputRegexp.SubexpNames()[1:]
	for _, submatches := range ssOutputRegexp.FindAllStringSubmatch(string(stdout), -1) {
		e := model.Edge{Type: "tcp_network"}
		for i, s := range submatches[1:] {
			switch groupNames[i] {
			case "localAddr":
				e.From.Host = s
			case "localPort":
				e.From.Section = s
			case "remoteAddr":
				e.To.Host = s
			case "remotePort":
				e.To.Section = s
			}
		}
		edges = append(edges, e)
	}

	log.Infof("NetworkTopology reported %d edges", len(edges))

	return edges, nil
}
