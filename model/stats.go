package model

import (
	"errors"
	"fmt"

	log "github.com/cihub/seelog"
	"github.com/dgryski/go-gk"
)

// Strategy defines the streaming quantile algorithm we want
type Strategy string

// For now we just implement 2 strategies for streaming quantiles
//  * EXACT just keeps every value in memory
//  * GK uses the Greenwald-Khanna algorithm to compress the data and use a fixed amount of memory, with epsilon-precision
const (
	EXACT Strategy = "exact"
	GK             = "gk"
)

// QuantileSummary is the common interface for streaming quantiles
type QuantileSummary interface {
	Insert(float64)
	Query(float64) float64
}

// StatsBucket encloses stats for all traces between 2 time boundaries
// stats are referenced by group in the StatsByGroup map
type StatsBucket struct {
	Strategy     Strategy               `json:"strategy"`
	GkEps        float64                `json:"epsilon"`
	Start        float64                `json:"start"`
	End          float64                `json:"duration"`
	StatsByGroup map[string]*StatsGroup `json:"by_group"` // FIXME: should we use something else than a string to designate a group?
}

// NewStatsBucket opens a new bucket at this time and initializes it properly
func NewStatsBucket(st Strategy, eps float64) *StatsBucket {
	return &StatsBucket{
		StatsByGroup: make(map[string]*StatsGroup),
		Start:        Now(),
		Strategy:     st,
		GkEps:        eps,
	}
}

// HandleSpan adds the span to this bucket stats
func (b *StatsBucket) HandleSpan(s *Span) {
	// TODO: implement a real strategy for expanding groups, but for now we make just one per service and service/resource
	byService := fmt.Sprintf("service:%s", s.Service)
	b.handleGroupSpan(byService, s)

	byServiceResource := fmt.Sprintf("service:%s,resource:%s", s.Service, s.Resource)
	b.handleGroupSpan(byServiceResource, s)
}

func (b *StatsBucket) handleGroupSpan(g string, s *Span) {
	if sg, ok := b.StatsByGroup[g]; ok {
		sg.AddSpan(s)
	} else {
		sg := NewStatsGroup(g, b.Strategy, b.GkEps)
		sg.AddSpan(s)
		b.StatsByGroup[g] = sg
	}
}

// StatsGroup represents for a certain group (set of tags) all the values that were inserted (count, sum, quantiles)
type StatsGroup struct {
	Group     string  `json:"group"`
	Count     uint64  `json:"count"`
	TotalTime float64 `json:"total_time"`
	Errors    uint64  `json:"errors"`
	// FIXME: marshal summaries properly
	Summary QuantileSummary `json:"summary"`
}

// NewStatsGroup initialize a new group with proper parameters
func NewStatsGroup(g string, st Strategy, eps float64) *StatsGroup {
	var summary QuantileSummary
	switch st {
	case EXACT:
		summary = gk.NewExact()
	case GK:
		summary = gk.New(eps)
	default:
		log.Errorf("Summary strategy %s not implemented, panicking", st)
		panic(errors.New("Unknown strategy"))
	}
	return &StatsGroup{
		Group:   g,
		Summary: summary,
	}
}

// AddSpan adds a span to the group by incrementing and inserting it in structures
func (sg *StatsGroup) AddSpan(s *Span) {
	sg.Count++
	sg.TotalTime += s.Duration
	// FIXME: increment errors somehow?
	sg.Summary.Insert(s.Duration)
}
