package main

import (
	"sync"
	"time"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/watchdog"
	log "github.com/cihub/seelog"
)

// ServiceMapper provides a cache layer over model.ServicesMetadata pipeline
// Used in conjunction with ServiceWriter: in-> ServiceMapper out-> ServiceWriter
type ServiceMapper struct {
	in    <-chan model.ServicesMetadata
	out   chan<- model.ServicesMetadata
	exit  chan bool
	done  sync.WaitGroup
	cache model.ServicesMetadata
}

// NewServiceMapper returns an instance of ServiceMapper with the provided channels
func NewServiceMapper(in <-chan model.ServicesMetadata, out chan<- model.ServicesMetadata) *ServiceMapper {
	return &ServiceMapper{
		in:    in,
		out:   out,
		exit:  make(chan bool),
		cache: make(model.ServicesMetadata),
	}
}

// Start runs the event loop in a non-blocking way
func (s *ServiceMapper) Start() {
	s.done.Add(1)

	go func() {
		defer watchdog.LogOnPanic()
		s.Run()
		s.done.Done()
	}()
}

// Stop gracefully terminates the event-loop
func (s *ServiceMapper) Stop() {
	close(s.exit)
	s.done.Wait()
}

// Run triggers the event-loop that consumes model.ServicesMeta
func (s *ServiceMapper) Run() {
	telemetryTicker := time.NewTicker(1 * time.Minute)
	defer telemetryTicker.Stop()

	for {
		select {
		case metadata := <-s.in:
			s.update(metadata)
		case <-telemetryTicker.C:
			log.Infof("total number of tracked services: %d", len(s.cache))
		case <-s.exit:
			return
		}
	}
}

func (s *ServiceMapper) update(metadata model.ServicesMetadata) {
	var changes model.ServicesMetadata

	for k, v := range metadata {
		if !s.shouldAdd(k, metadata) {
			continue
		}

		// We do this inside the for loop to avoid unecessary memory allocations.
		// After few method executions, the cache will be warmed up and this section be skipped altogether.
		if changes == nil {
			changes = make(model.ServicesMetadata)
		}

		changes[k] = v
	}

	if changes == nil {
		return
	}

	s.out <- changes
	s.cache.Merge(changes)
}

func (s *ServiceMapper) shouldAdd(service string, metadata model.ServicesMetadata) bool {
	cacheEntry, ok := s.cache[service]

	// No cache entry?
	if !ok {
		return true
	}

	// Cache entry came from service API?
	if _, ok = cacheEntry[model.ServiceApp]; ok {
		return false
	}

	// New metadata value came from service API?
	_, ok = metadata[service][model.ServiceApp]

	return ok
}
