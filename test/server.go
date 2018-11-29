package test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DataDog/datadog-trace-agent/internal/agent"
	"github.com/gogo/protobuf/proto"
	"github.com/tinylib/msgp/msgp"
)

const defaultBackendAddress = "localhost:8888"

// defaultChannelSize is the default size of the buffered channel
// receiving any payloads sent by the trace-agent to the backend.
const defaultChannelSize = 100

// Runner can start an agent instance using a custom configuration, send payloads
// to it and act as a fake backend. A runner is ready to use as is. To use it,
// first call Start, then RunAgent, and then Post to send payloads. Use the channel
// provided by Out to assert output.
type Runner struct {
	// Verbose will make the runner logs more verbosely, such as agent
	// starts and stops.
	Verbose bool

	// ChannelSize specifies the size of the buffered "out" channel
	// which receives any payloads sent by the trace-agent to the
	// fake backend.
	// Defaults to 100.
	ChannelSize int

	mu  sync.RWMutex // guards pid
	pid int          // agent pid, if running

	agentPort int              // agent port
	log       *safeBuffer      // agent log
	srv       http.Server      // fake backend
	out       chan interface{} // payload output
	started   uint64           // 0 if server is stopped
}

// Start starts the fake backend.
func (s *Runner) Start() error {
	return s.startBackend()
}

// Out returns a channel which will provide payloads received by the fake backend.
// They can be of type agent.TracePayload or agent.StatsPayload.
func (s *Runner) Out() <-chan interface{} { return s.out }

// Post posts the given list of traces to the trace agent. Before posting, agent must
// be started. You can start an agent using RunAgent.
func (s *Runner) Post(traceList agent.Traces) error {
	s.mu.RLock()
	if s.pid == 0 {
		defer s.mu.RUnlock()
		return errors.New("post: trace-agent not running")
	}
	s.mu.RUnlock()

	var buf bytes.Buffer
	if err := msgp.Encode(&buf, traceList); err != nil {
		return err
	}
	addr := fmt.Sprintf("http://localhost:%d/v0.3/traces", s.agentPort)
	req, err := http.NewRequest("POST", addr, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("X-Datadog-Trace-Count", strconv.Itoa(len(traceList)))
	req.Header.Set("Content-Type", "application/msgpack")
	req.Header.Set("Content-Length", strconv.Itoa(buf.Len()))

	_, err = http.DefaultClient.Do(req)
	// TODO: check response
	return err
}

// Shutdown shuts down the backend and stops any running agent.
func (s *Runner) Shutdown(wait time.Duration) error {
	defer close(s.out)
	defer atomic.StoreUint64(&s.started, 0)

	s.StopAgent()
	ctx, _ := context.WithTimeout(context.Background(), wait)
	return s.srv.Shutdown(ctx)
}

func (s *Runner) startBackend() error {
	if atomic.LoadUint64(&s.started) > 0 {
		// already running
		return nil
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v0.2/traces", s.handleTraces)
	mux.HandleFunc("/api/v0.2/stats", s.handleStats)
	mux.HandleFunc("/_health", s.handleHealth)

	size := defaultChannelSize
	if s.ChannelSize != 0 {
		size = s.ChannelSize
	}
	s.out = make(chan interface{}, size)
	s.log = newSafeBuffer()
	s.srv = http.Server{
		Addr:    defaultBackendAddress,
		Handler: mux,
	}
	go s.srv.ListenAndServe()
	atomic.StoreUint64(&s.started, 1)

	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-timeout:
			return errors.New("timeout out waiting for startup")
		default:
			resp, _ := http.Get(fmt.Sprintf("http://%s/_health", s.srv.Addr))
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

func (s *Runner) handleHealth(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Runner) handleStats(w http.ResponseWriter, req *http.Request) {
	var payload agent.StatsPayload
	if err := readJSONRequest(req, &payload); err != nil {
		log.Println("server: error reading stats: ", err)
	}
	s.out <- payload
}

func (s *Runner) handleTraces(w http.ResponseWriter, req *http.Request) {
	var payload agent.TracePayload
	if err := readProtoRequest(req, &payload); err != nil {
		log.Println("server: error reading traces: ", err)
	}
	s.out <- payload
}

func readJSONRequest(req *http.Request, v interface{}) error {
	r, err := readCloserFromRequest(req)
	if err != nil {
		return err
	}
	defer r.Close()
	return json.NewDecoder(r).Decode(v)
}

func readProtoRequest(req *http.Request, msg proto.Message) error {
	r, err := readCloserFromRequest(req)
	if err != nil {
		return err
	}
	slurp, err := ioutil.ReadAll(r)
	defer r.Close()
	if err != nil {
		return err
	}
	return proto.Unmarshal(slurp, msg)
}

func readCloserFromRequest(req *http.Request) (io.ReadCloser, error) {
	r := struct {
		io.Reader
		io.Closer
	}{
		Reader: req.Body,
		Closer: req.Body,
	}
	if req.Header.Get("Accept-Encoding") == "gzip" {
		gz, err := gzip.NewReader(req.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r.Reader = gz
	}
	return r, nil
}
