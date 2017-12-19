package info

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
	receiverStats       []TagStats    // only for the last minute
	endpointStats       EndpointStats // only for the last minute
	watchdogInfo        watchdog.Info
	samplerInfo         SamplerInfo
	prioritySamplerInfo SamplerInfo
	rateByService       map[string]float64
	preSamplerStats     sampler.PreSamplerStats
	start               = time.Now()
	once                sync.Once
	infoTmpl            *template.Template
	notRunningTmpl      *template.Template
	errorTmpl           *template.Template
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
  API Endpoint: {{.Status.Config.APIEndpoint}}{{ range $i, $ts := .Status.Receiver }}

  --- Receiver stats (1 min) ---

  -> tags: {{if $ts.Tags.Lang}}{{ $ts.Tags.Lang }}, {{ $ts.Tags.LangVersion }}, {{ $ts.Tags.Interpreter }}, {{ $ts.Tags.TracerVersion }}{{else}}None{{end}}

    Traces received: {{ $ts.Stats.TracesReceived }} ({{ $ts.Stats.TracesBytes }} bytes)
    Spans received: {{ $ts.Stats.SpansReceived }}
    Services received: {{ $ts.Stats.ServicesReceived }} ({{ $ts.Stats.ServicesBytes }} bytes)
    Total data received: {{ add $ts.Stats.TracesBytes $ts.Stats.ServicesBytes }} bytes{{if gt $ts.Stats.TracesDropped 0}}

    WARNING: Traces dropped: {{ $ts.Stats.TracesDropped }}
    {{end}}{{if gt $ts.Stats.SpansDropped 0}}WARNING: Spans dropped: {{ $ts.Stats.SpansDropped }}{{end}}

  ------------------------------{{end}}
{{ range $key, $value := .Status.RateByService }}
  Sample rate for '{{ $key }}': {{percent $value}} %{{ end }}{{if lt .Status.PreSampler.Rate 1.0}}

  WARNING: Pre-sampling traces: {{percent .Status.PreSampler.Rate}} %
{{end}}{{if .Status.PreSampler.Error}}  WARNING: Pre-sampler: {{.Status.PreSampler.Error}}
{{end}}

  Bytes sent (1 min): {{add .Status.Endpoint.TracesBytes .Status.Endpoint.ServicesBytes}}
  Traces sent (1 min): {{.Status.Endpoint.TracesCount}}
  Stats sent (1 min): {{.Status.Endpoint.TracesStats}}
{{if gt .Status.Endpoint.TracesPayloadError 0}}  WARNING: Traces API errors (1 min): {{.Status.Endpoint.TracesPayloadError}}/{{.Status.Endpoint.TracesPayload}}
{{end}}{{if gt .Status.Endpoint.ServicesPayloadError 0}}  WARNING: Services API errors (1 min): {{.Status.Endpoint.ServicesPayloadError}}/{{.Status.Endpoint.ServicesPayload}}
{{end}}
`
	notRunningTmplSrc = `{{.Banner}}
{{.Program}}
{{.Banner}}

  Not running (port {{.ReceiverPort}})

`
	errorTmplSrc = `{{.Banner}}
{{.Program}}
{{.Banner}}

  Error: {{.Error}}
  URL: {{.URL}}

`
)

// UpdateReceiverStats updates internal stats about the receiver
func UpdateReceiverStats(rs *ReceiverStats) {
	infoMu.Lock()
	defer infoMu.Unlock()
	rs.RLock()
	defer rs.RUnlock()

	s := make([]TagStats, 0, len(rs.Stats))
	for _, tagStats := range rs.Stats {
		if !tagStats.isEmpty() {
			s = append(s, *tagStats)
		}
	}

	receiverStats = s
}

func publishReceiverStats() interface{} {
	infoMu.RLock()
	defer infoMu.RUnlock()
	return receiverStats
}

// UpdateEndpointStats updates internal stats about API endpoints
func UpdateEndpointStats(es EndpointStats) {
	infoMu.Lock()
	defer infoMu.Unlock()
	endpointStats = es
}

func publishEndpointStats() interface{} {
	infoMu.RLock()
	defer infoMu.RUnlock()
	return endpointStats
}

// UpdateSamplerInfo updates internal stats about signature sampling
func UpdateSamplerInfo(ss SamplerInfo) {
	infoMu.Lock()
	defer infoMu.Unlock()

	samplerInfo = ss
}

func publishSamplerInfo() interface{} {
	infoMu.RLock()
	defer infoMu.RUnlock()
	return samplerInfo
}

// UpdatePrioritySamplerInfo updates internal stats about priority sampking
func UpdatePrioritySamplerInfo(ss SamplerInfo) {
	infoMu.Lock()
	defer infoMu.Unlock()

	prioritySamplerInfo = ss
}

func publishPrioritySamplerInfo() interface{} {
	infoMu.RLock()
	defer infoMu.RUnlock()
	return prioritySamplerInfo
}

// UpdateRateByService updates the RateByService map
func UpdateRateByService(rbs map[string]float64) {
	infoMu.Lock()
	defer infoMu.Unlock()
	rateByService = rbs
}

func publishRateByService() interface{} {
	infoMu.RLock()
	defer infoMu.RUnlock()
	return rateByService
}

// UpdateWatchdogInfo updates internal stats about the watchdog
func UpdateWatchdogInfo(wi watchdog.Info) {
	infoMu.Lock()
	defer infoMu.Unlock()
	watchdogInfo = wi
}

func publishWatchdogInfo() interface{} {
	infoMu.RLock()
	defer infoMu.RUnlock()
	return watchdogInfo
}

// UpdatePreSampler updates internal stats about the pre-sampling
func UpdatePreSampler(ss sampler.PreSamplerStats) {
	infoMu.Lock()
	defer infoMu.Unlock()
	preSamplerStats = ss
}

func publishPreSamplerStats() interface{} {
	infoMu.RLock()
	defer infoMu.RUnlock()
	return preSamplerStats
}

func publishUptime() interface{} {
	return int(time.Since(start) / time.Second)
}

type infoString string

func (s infoString) String() string { return string(s) }

// InitInfo initializes the info structure. It should be called only once.
func InitInfo(conf *config.AgentConfig) error {
	var err error

	funcMap := template.FuncMap{
		"add": func(a, b int64) int64 {
			return a + b
		},
		"percent": func(v float64) string {
			return fmt.Sprintf("%02.1f", v*100)
		},
	}

	once.Do(func() {
		expvar.NewInt("pid").Set(int64(os.Getpid()))
		expvar.Publish("uptime", expvar.Func(publishUptime))
		expvar.Publish("version", expvar.Func(publishVersion))
		expvar.Publish("receiver", expvar.Func(publishReceiverStats))
		expvar.Publish("endpoint", expvar.Func(publishEndpointStats))
		expvar.Publish("sampler", expvar.Func(publishSamplerInfo))
		expvar.Publish("prioritysampler", expvar.Func(publishPrioritySamplerInfo))
		expvar.Publish("ratebyservice", expvar.Func(publishRateByService))
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

		notRunningTmpl, err = template.New("infoNotRunning").Parse(notRunningTmplSrc)
		if err != nil {
			return
		}

		errorTmpl, err = template.New("infoError").Parse(errorTmplSrc)
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
	Version       infoVersion             `json:"version"`
	Receiver      []TagStats              `json:"receiver"`
	RateByService map[string]float64      `json:"ratebyservice"`
	Endpoint      EndpointStats           `json:"endpoint"`
	Watchdog      watchdog.Info           `json:"watchdog"`
	PreSampler    sampler.PreSamplerStats `json:"presampler"`
	Config        config.AgentConfig      `json:"config"`
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
		_ = notRunningTmpl.Execute(w, struct {
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
		_ = errorTmpl.Execute(w, struct {
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

	// remove the default service and env, it can be inferred from other
	// values so has little added-value and could be confusing for users.
	// Besides, if one still really wants it:
	// curl http://localhost:8126/degug/vars would show it.
	if info.RateByService != nil {
		delete(info.RateByService, "service:,env:")
	}

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
