package main

import (
	"encoding/json"
	"expvar" // automatically publish `/debug/vars` on HTTP port
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/sampler"
	"github.com/DataDog/datadog-trace-agent/watchdog"
)

var (
	infoMu              sync.RWMutex
	infoReceiverStats   Stats         // only for the last minute
	infoEndpointStats   endpointStats // only for the last minute
	infoWatchdogInfo    watchdog.Info
	infoSamplerInfo     samplerInfo
	infoPreSamplerStats sampler.PreSamplerStats
	infoStart           = time.Now()
	infoOnce            sync.Once
	infoTmpl            *template.Template
	infoNotRunningTmpl  *template.Template
	infoErrorTmpl       *template.Template
)

const (
	infoTmplSrc = `{{.Banner}}
{{.Program}}
{{.Banner}}

  Pid: {{.Status.Pid}}
  Uptime: {{.Status.Uptime}} seconds
  Mem alloc: {{.Status.MemStats.Alloc}} bytes

  Hostname: {{.Status.Config.HostName}}
  Receiver: {{.Status.Config.ReceiverHost}}:{{.Status.Config.ReceiverPort}}
  API Endpoint: {{.Status.Config.APIEndpoint}}

  Bytes received (1 min): {{add .Status.Receiver.TracesBytes .Status.Receiver.ServicesBytes}}
  Traces received (1 min): {{.Status.Receiver.TracesReceived}}
  Spans received (1 min): {{.Status.Receiver.SpansReceived}}
{{if gt .Status.Receiver.TracesDropped 0}}  WARNING: Traces dropped (1 min): {{.Status.Receiver.TracesDropped}}
{{end}}{{if gt .Status.Receiver.SpansDropped 0}}  WARNING: Spans dropped (1 min): {{.Status.Receiver.SpansDropped}}
{{end}}{{if lt .Status.PreSampler.Rate 1.0}}  WARNING: Pre-sampling traces: {{percent .Status.PreSampler.Rate}} %
{{end}}{{if .Status.PreSampler.Error}}  WARNING: Pre-sampler: {{.Status.PreSampler.Error}}
{{end}}
  Bytes sent (1 min): {{add .Status.Endpoint.TracesBytes .Status.Endpoint.ServicesBytes}}
  Traces sent (1 min): {{.Status.Endpoint.TracesCount}}
  Stats sent (1 min): {{.Status.Endpoint.TracesStats}}
{{if gt .Status.Endpoint.TracesPayloadError 0}}  WARNING: Traces API errors (1 min): {{.Status.Endpoint.TracesPayloadError}}/{{.Status.Endpoint.TracesPayload}}
{{end}}{{if gt .Status.Endpoint.ServicesPayloadError 0}}  WARNING: Services API errors (1 min): {{.Status.Endpoint.ServicesPayloadError}}/{{.Status.Endpoint.ServicesPayload}}
{{end}}
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

func updateReceiverStats(s Stats) {
	infoMu.Lock()
	infoReceiverStats = s
	infoMu.Unlock()
}

func publishReceiverStats() interface{} {
	infoMu.RLock()
	rs := infoReceiverStats
	infoMu.RUnlock()
	return rs
}

func updateEndpointStats(es endpointStats) {
	infoMu.Lock()
	infoEndpointStats = es
	infoMu.Unlock()
}

func publishEndpointStats() interface{} {
	infoMu.RLock()
	es := infoEndpointStats
	infoMu.RUnlock()
	return es
}

func updateSamplerInfo(ss samplerInfo) {
	infoMu.Lock()
	infoSamplerInfo = ss
	infoMu.Unlock()
}

func publishSamplerInfo() interface{} {
	infoMu.RLock()
	ss := infoSamplerInfo
	infoMu.RUnlock()
	return ss
}

func updateWatchdogInfo(wi watchdog.Info) {
	infoMu.Lock()
	infoWatchdogInfo = wi
	infoMu.Unlock()
}

func publishWatchdogInfo() interface{} {
	infoMu.RLock()
	wi := infoWatchdogInfo
	infoMu.RUnlock()
	return wi
}

func updatePreSampler(ss sampler.PreSamplerStats) {
	infoMu.Lock()
	infoPreSamplerStats = ss
	infoMu.Unlock()
}

func publishPreSamplerStats() interface{} {
	infoMu.RLock()
	ss := infoPreSamplerStats
	infoMu.RUnlock()
	return ss
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

	funcMap := template.FuncMap{
		"add": func(a, b int64) int64 {
			return a + b
		},
		"percent": func(v float64) string {
			return fmt.Sprintf("%02.1f", v*100)
		},
	}

	infoOnce.Do(func() {
		expvar.NewInt("pid").Set(int64(os.Getpid()))
		expvar.Publish("uptime", expvar.Func(publishUptime))
		expvar.Publish("version", expvar.Func(publishVersion))
		expvar.Publish("receiver", expvar.Func(publishReceiverStats))
		expvar.Publish("endpoint", expvar.Func(publishEndpointStats))
		expvar.Publish("sampler", expvar.Func(publishSamplerInfo))
		expvar.Publish("watchdog", expvar.Func(publishWatchdogInfo))
		expvar.Publish("presampler", expvar.Func(publishPreSamplerStats))

		c := *conf
		c.APIKey = "" // should not be exported by JSON, but just to make sure
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

		infoTmpl, err = template.New("info").Funcs(funcMap).Parse(infoTmplSrc)
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
	Version    infoVersion             `json:"version"`
	Receiver   Stats                   `json:"receiver"`
	Endpoint   endpointStats           `json:"endpoint"`
	Watchdog   watchdog.Info           `json:"watchdog"`
	PreSampler sampler.PreSamplerStats `json:"presampler"`
	Config     config.AgentConfig      `json:"config"`
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
// -----8<-------------------------------------------------------
// ======================
// Trace Agent (v 0.99.0)
// ======================
//
//   Pid: 38149
//   Uptime: 15 seconds
//   Mem alloc: 773552 bytes
//
//   Hostname: localhost.localdomain
//   Receiver: localhost:8126
//   API Endpoint: https://trace.agent.datadoghq.com
//
//   Bytes received (1 min): 10000
//   Traces received (1 min): 240
//   Spans received (1 min): 360
//   WARNING: Traces dropped (1 min): 5
//   WARNING: Spans dropped (1 min): 10
//   WARNING: Pre-sampling traces: 26.0 %
//   WARNING: Pre-sampler: raising pre-sampling rate from 2.9 % to 5.0 %
//
//   Bytes sent (1 min): 3245
//   Traces sent (1 min): 6
//   Stats sent (1 min): 60
//   WARNING: Traces API errors (1 min): 1/3
//   WARNING: Services API errors (1 min): 1/1
//
// -----8<-------------------------------------------------------
//
// The "WARNING:" lines are hidden if there's nothing dropped or no errors.
//
// Typical output of 'trace-agent -info' when agent is not running:
//
// -----8<-------------------------------------------------------
// ======================
// Trace Agent (v 0.99.0)
// ======================
//
//   Not running (port 8126)
//
// -----8<-------------------------------------------------------
//
// Typical output of 'trace-agent -info' when something unexpected happened,
// for instance we're connecting to an HTTP server that serves an inadequate
// response, or there's a bug, or... :
//
// -----8<-------------------------------------------------------
// ======================
// Trace Agent (v 0.99.0)
// ======================
//
//   Error: json: cannot unmarshal number into Go value of type main.StatusInfo
//   URL: http://localhost:8126/debug/vars
//
// -----8<-------------------------------------------------------
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
	if err != nil {
		return err
	}
	return nil
}
