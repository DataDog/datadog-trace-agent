package main

import (
	"encoding/json"
	"expvar" // automatically publish `/debug/vars` on HTTP port
	"fmt"
	"github.com/DataDog/datadog-trace-agent/config"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

var (
	infoMu             sync.RWMutex
	infoReceiverStats  receiverStats // only for the last minute
	infoStart          = time.Now()
	infoOnce           sync.Once
	infoTmpl           *template.Template
	infoNotRunningTmpl *template.Template
	infoErrorTmpl      *template.Template
)

const (
	infoTmplSrc = `{{.Banner}}
{{.Program}}
{{.Banner}}

  Command line:{{range .Status.CmdLine}} {{.}}{{end}}
  Pid: {{.Status.Pid}}
  Uptime: {{.Status.Uptime}}
  Mem alloc: {{.Status.MemStats.Alloc}}
  Hostname: {{.Status.Config.HostName}}
  Receiver host: {{.Status.Config.ReceiverHost}}
  Receiver port: {{.Status.Config.ReceiverPort}}
  Statsd host: {{.Status.Config.StatsdHost}}
  Statsd port: {{.Status.Config.StatsdPort}}
  API Endpoints:{{range .Status.Config.APIEndpoints}} {{.}}{{end}}

  Spans received (1 min): {{.Status.Receiver.SpansReceived}}
  Traces received (1 min): {{.Status.Receiver.TracesReceived}}
  Spans dropped (1 min): {{.Status.Receiver.SpansDropped}}
  Traces dropped (1 min): {{.Status.Receiver.TracesDropped}}

`
	infoNotRunningTmplSrc = `{{.Banner}}
{{.Program}}
{{.Banner}}

  Not running (port {{.ReceiverPort}})

`
	infoErrorTmplSrc = `{{.Banner}}
{{.Program}}
{{.Banner}}

  Error: {{.Error}}
  URL: {{.URL}}

`
)

func publishUptime() interface{} {
	return int(time.Since(infoStart) / time.Second)
}

func updateReceiverStats(rs receiverStats) {
	infoMu.Lock()
	infoReceiverStats = rs
	infoMu.Unlock()
}

func publishReceiverStats() interface{} {
	infoMu.RLock()
	rs := infoReceiverStats
	infoMu.RUnlock()
	return rs
}

type infoVersion struct {
	Version   string
	GitCommit string
	GitBranch string
	BuildDate string
	GoVersion string
}

func publishVersion() interface{} {
	return infoVersion{
		Version:   Version,
		GitCommit: GitCommit,
		GitBranch: GitBranch,
		BuildDate: BuildDate,
		GoVersion: GoVersion,
	}
}

type infoString string

func (s infoString) String() string { return string(s) }

// This should be called only once
func initInfo(conf *config.AgentConfig) error {
	var err error

	infoOnce.Do(func() {
		expvar.NewInt("pid").Set(int64(os.Getpid()))
		expvar.Publish("uptime", expvar.Func(publishUptime))
		expvar.Publish("version", expvar.Func(publishVersion))
		expvar.Publish("receiver", expvar.Func(publishReceiverStats))

		c := *conf
		c.APIKeys = nil // should not be exported by JSON, but just to make sure
		var buf []byte
		buf, err = json.Marshal(&c)
		if err != nil {
			return
		}

		// We keep a static copy of the config, already marshalled and stored
		// as a plain string. This saves the hassle of rebuilding it all the time
		// and avoids race issues as the source object is never used again.
		// Config is parsed at the beginning and never changed again, anyway.
		expvar.Publish("config", infoString(string(buf)))

		infoTmpl, err = template.New("info").Parse(infoTmplSrc)
		if err != nil {
			return
		}

		infoNotRunningTmpl, err = template.New("infoNotRunning").Parse(infoNotRunningTmplSrc)
		if err != nil {
			return
		}

		infoErrorTmpl, err = template.New("infoError").Parse(infoErrorTmplSrc)
		if err != nil {
			return
		}
	})

	return err
}

// StatusInfo is what we use to parse expvar response.
// It does not need to contain all the fields, only those we need
// to display when called with `-info` as JSON unmarshaller will
// automatically ignore extra fields.
type StatusInfo struct {
	CmdLine  []string `json:"cmdline"`
	Pid      int      `json:"pid"`
	Uptime   int      `json:"uptime"`
	MemStats struct {
		Alloc uint64
	} `json:"memstats"`
	Version  infoVersion        `json:"version"`
	Receiver receiverStats      `json:"receiver"`
	Config   config.AgentConfig `json:"config"`
}

func getProgramBanner(version string) (string, string) {
	program := fmt.Sprintf("Trace Agent (v %s)", version)
	banner := strings.Repeat("=", len(program))

	return program, banner
}

// Info writes a standard info message describing the running agent.
// This is not the current program, but an already running program,
// which we query with an HTTP request.
//
// If error is nil, means the program is running.
// If not, it displays a pretty-printed message anyway (for support)
//
// Typical output of 'trace-agent -info' when agent is running:
//
// ======================
// Trace Agent (v 0.99.0)
// ======================
//
//   Command line: ./trace-agent
//   Pid: 65365
//   Uptime: 3
//   Mem alloc: 779104
//   Hostname: localhost.localdomain
//   Receiver host: localhost
//   Receiver port: 7777
//   Statsd host: localhost
//   Statsd port: 8125
//   API Endpoints: https://trace.agent.datadoghq.com
//
//   Spans received (1 min): 0
//   Traces received (1 min): 0
//   Spans dropped (1 min): 0
//   Traces dropped (1 min): 0
//
// Typical output of 'trace-agent -info' when agent is not running:
//
// ======================
// Trace Agent (v 0.99.0)
// ======================
//
//   Not running (port 7777)
//
// Typical output of 'trace-agent -info' when something unexpected happened,
// for instance we're connecting to an HTTP server that serves an inadequate
// response, or there's a bug, or... :
//
// ======================
// Trace Agent (v 0.99.0)
// ======================
//
//   Error: json: cannot unmarshal number into Go value of type main.StatusInfo
//   URL: http://localhost:7777/debug/vars
//
func Info(w io.Writer, conf *config.AgentConfig) error {
	host := conf.ReceiverHost
	if host == "0.0.0.0" {
		host = "127.0.0.1" // [FIXME:christian] not fool-proof
	}
	url := "http://localhost:" + strconv.Itoa(conf.ReceiverPort) + "/debug/vars"
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		// OK, here, we can't even make an http call on the agent port,
		// so we can assume it's not even running, or at least, not with
		// these parameters. We display the port as a hint on where to
		// debug further, this is where the expvar JSON should come from.
		program, banner := getProgramBanner(Version)
		_ = infoNotRunningTmpl.Execute(w, struct {
			Banner       string
			Program      string
			ReceiverPort int
		}{
			Banner:       banner,
			Program:      program,
			ReceiverPort: conf.ReceiverPort,
		})
		return err
	}

	defer resp.Body.Close() // OK to defer, this is not on hot path

	var info StatusInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		program, banner := getProgramBanner(Version)
		_ = infoErrorTmpl.Execute(w, struct {
			Banner  string
			Program string
			Error   error
			URL     string
		}{
			Banner:  banner,
			Program: program,
			Error:   err,
			URL:     url,
		})
		return err
	}

	// display the remote program version, now that we know it
	program, banner := getProgramBanner(info.Version.Version)
	err = infoTmpl.Execute(w, struct {
		Banner  string
		Program string
		Status  *StatusInfo
	}{
		Banner:  banner,
		Program: program,
		Status:  &info,
	})
	return nil
}
