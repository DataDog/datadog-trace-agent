package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	log "github.com/cihub/seelog"

	"github.com/DataDog/raclette/model"
)

// StatsWriter implements a Writer and writes to the Datadog API spans
type StatsWriter struct {
	in          chan model.StatsBucket
	statsBuffer []model.StatsBucket
	endpoint    string

	// exit channels
	exit      chan bool
	exitGroup *sync.WaitGroup
}

// NewStatsWriter returns a new Writer
func NewStatsWriter(endp string, exit chan bool, exitGroup *sync.WaitGroup) *StatsWriter {
	return &StatsWriter{
		endpoint:  endp,
		exit:      exit,
		exitGroup: exitGroup,
	}
}

// Init initalizes the span buffer and the input channel of spans
func (w *StatsWriter) Init(in chan model.StatsBucket) {
	w.in = in

	// NOTE: should this be unbounded?
	w.statsBuffer = []model.StatsBucket{}
}

// Start runs the writer by consuming spans in a buffer and periodically
// flushing to the API
func (w *StatsWriter) Start() {
	// will shutdown as the input channel is closed
	go func() {
		for s := range w.in {
			w.statsBuffer = append(w.statsBuffer, s)
		}
	}()

	go w.periodicFlush()

	log.Info("StatsWriter started")
}

func (w *StatsWriter) periodicFlush() {
	ticker := time.Tick(3 * time.Second)
	for {
		select {
		case <-ticker:
			w.Flush()
		case <-w.exit:
			log.Info("StatsWriter asked to exit. Flushing and exiting")
			// FIXME, make sure w.in is closed before to make sure we received all spans
			w.Flush()
			return
		}
	}
}

// Flush the span buffer by writing to the API its contents
func (w *StatsWriter) Flush() {
	stats := w.statsBuffer
	if len(stats) == 0 {
		log.Info("Nothing to flush")
		return
	}
	w.statsBuffer = []model.StatsBucket{}
	log.Infof("StatsWriter flush to the API, %d stats buckets", len(stats))

	payload := model.StatsPayload{
		// FIXME, this should go in a config file
		APIKey: "424242",
		Stats:  stats,
	}

	// FIXME, this should go in a config file
	url := w.endpoint + "/stats"

	jsonStr, err := json.Marshal(payload)
	if err != nil {
		log.Errorf("Error marshalling: %s", err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		log.Errorf("Error creating request: %s", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("Error posting request: %s", err)
		return
	}
	defer resp.Body.Close()
}
