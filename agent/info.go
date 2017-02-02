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
)

const (
	infoTmplSrc = `{{.Banner}}
{{.Program}}
{{.Banner}}

  Version: {{.Status.Version.Version}}
  Git hash: {{.Status.Version.GitCommit}}
  Git branch: {{.Status.Version.GitBranch}}
  Build date: {{.Status.Version.BuildDate}}
  Go Version: {{.Status.Version.GoVersion}}

  Command line: {{.CmdLine}}
  Pid: {{.Status.Pid}}
  Uptime: {{.Status.Uptime}}
  Mem alloc: {{.Status.MemStats.Alloc}}
  Hostname: {{.Status.Config.HostName}}
  Receiver host: {{.Status.Config.ReceiverHost}}
  Receiver port: {{.Status.Config.ReceiverPort}}
  Statsd host: {{.Status.Config.StatsdHost}}
  Statsd port: {{.Status.Config.StatsdPort}}
  API Endpoints: {{.APIEndpoints}}

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

// Info writes a standard info message describing the running agent.
// This is not the current program, but an already running program,
// which we query with an HTTP request.
//
// It returns an error if it could not generate a proper string.
// But no error does not mean the program we want to query is running.
// Eg:
// - if network port is unreachable with HTTP, write a pretty-printed
//   message, return false, and no error.
// - if we can successfully get JSON through HTTP, and parse it, write
//   a pretty-printed message, return true, and no error
// - if we can make an HTTP all, but get inconsitent data, write nothing,
//   return false, and an error.
func Info(w io.Writer, conf *config.AgentConfig) (bool, error) {
	program := fmt.Sprintf("Trace Agent (v %s)", Version)
	banner := strings.Repeat("=", len(program))

	host := conf.ReceiverHost
	if host == "0.0.0.0" {
		host = "127.0.0.1" // [FIXME:christian] not fool-proof
	}
	url := "http://localhost:" + strconv.Itoa(conf.ReceiverPort) + "/debug/vars"
	resp, err := http.Get(url)

	if err != nil {
		// OK, here, we can't even make an http call on the agent port,
		// so we can assume it's not even running, or at least, not with
		// these parameters. We display the port as a hint on where to
		// debug further, this is where the expvar JSON should come from.
		err = infoNotRunningTmpl.Execute(w, struct {
			Banner       string
			Program      string
			ReceiverPort int
		}{
			Banner:       banner,
			Program:      program,
			ReceiverPort: conf.ReceiverPort,
		})
		return false, err
	}

	defer resp.Body.Close() // OK to defer, this is not on hot path

	var info StatusInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return false, fmt.Errorf("unable to read response from Datadog Trace Agent on '%s'\nERROR: %v\n", url, err)
	}

	err = infoTmpl.Execute(w, struct {
		Banner       string
		Program      string
		CmdLine      string
		APIEndpoints string
		Status       *StatusInfo
	}{
		Banner:       banner,
		Program:      program,
		CmdLine:      strings.Join(info.CmdLine, " "), // [FIXME:christian] find a way to do this in text/template
		APIEndpoints: strings.Join(info.Config.APIEndpoints, ", "),
		Status:       &info,
	})
	return true, err
}
