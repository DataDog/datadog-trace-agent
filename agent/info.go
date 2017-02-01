package main

import (
	"encoding/json"
	"expvar" // automatically publish `/debug/vars` on HTTP port
	"fmt"
	"github.com/DataDog/datadog-trace-agent/config"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	infoMu            sync.RWMutex
	infoReceiverStats ReceiverStats // only for the last minute
	infoJSONConfig    string
	infoStart         time.Time
)

func init() {
	infoStart = time.Now()
}

func publishUptime() interface{} {
	return int(time.Now().Sub(infoStart) / time.Second)
}

func updateReceiverStats(rs ReceiverStats) {
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

type infoConfig struct{}

// String implements the expvar.Var interface
func (infoConfig) String() string {
	infoMu.RLock()
	c := infoJSONConfig
	infoMu.RUnlock()
	return c
}

func updateConf(conf *config.AgentConfig) error {
	infoMu.Lock()
	defer infoMu.Unlock()
	c := *conf
	c.APIKeys = nil // should not be exported by JSON, but just to make sure
	buf, err := json.Marshal(&c)
	if err != nil {
		return err
	}
	// We keep a static copy of the config, already marshalled and stored
	// as a plain string. This saves the hassle of rebuilding it all the time
	// and avoids race issues as the source object is never used again.
	// Config is parsed at the beginning and never changed again, anyway.
	infoJSONConfig = string(buf)
	expvar.Publish("config", infoConfig{})
	return nil
}

type infoVersion struct{}

// Below are types used to simply implement expvar.Var interface
// for config options. expvar.SetString does not make it easy to set
// a value within a map, and we need the 5 version-related fields
// to be in a same namespace (so in a Map).

// String implements the expvar.Var interface
func (infoVersion) String() string { return `"` + Version + `"` }

type infoGitCommit struct{}

// String implements the expvar.Var interface
func (infoGitCommit) String() string { return `"` + GitCommit + `"` }

type infoGitBranch struct{}

// String implements the expvar.Var interface
func (infoGitBranch) String() string { return `"` + GitBranch + `"` }

type infoBuildDate struct{}

// String implements the expvar.Var interface
func (infoBuildDate) String() string { return `"` + BuildDate + `"` }

type infoGoVersion struct{}

// String implements the expvar.Var interface
func (infoGoVersion) String() string { return `"` + GoVersion + `"` }

func init() {
	expvar.NewInt("pid").Set(int64(os.Getpid()))
	expvar.Publish("uptime", expvar.Func(publishUptime))
	version := expvar.NewMap("version")
	version.Set("Version", infoVersion{})
	version.Set("GitCommit", infoGitCommit{})
	version.Set("GitBranch", infoGitBranch{})
	version.Set("BuildDate", infoBuildDate{})
	version.Set("GoVersion", infoGoVersion{})
	expvar.Publish("receiver", expvar.Func(publishReceiverStats))
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
	Version struct {
		Version   string
		GitCommit string
		GitBranch string
		BuildDate string
		GoVersion string
	} `json:"version"`
	Receiver ReceiverStats      `json:"receiver"`
	Config   config.AgentConfig `json:"config"`
}

// Info returns a printable string, suitable for the `-info` option.
// It returns an error if it could not generate a proper string.
// But no error does not mean the program we want to query is running.
// Eg:
// - if network port is unreachable with HTTP, return a pretty-printed
//   message, false, and no error.
// - if we can successfully get JSON through HTTP, and parse it, return
//   a pretty-printed message, true, and no error
// - if we can make an HTTP all, but get inconsitent data, return no
//   message, false, and an error.
func Info(conf *config.AgentConfig) (string, bool, error) {
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
		return (banner + "\n" +
			program + "\n" +
			banner + "\n" +
			"\n" +
			"  Not running (port " + strconv.Itoa(conf.ReceiverPort) + ")\n" +
			"\n"), false, nil
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("unable to read response from Datadog Trace Agent on '%s'\nERROR: %v\n", url, err)
	}

	var info StatusInfo
	err = json.Unmarshal(body, &info)
	if err != nil {
		return "", false, fmt.Errorf("unable to parse response from Datadog Trace Agent on '%s'\nERROR: %v\n", url, err)
	}
	return (banner + "\n" +
		program + "\n" +
		banner + "\n" +
		"\n" +
		"  Version: " + info.Version.Version + "\n" +
		"  Git hash: " + info.Version.GitCommit + "\n" +
		"  Git branch: " + info.Version.GitBranch + "\n" +
		"  Build date: " + info.Version.BuildDate + "\n" +
		"  Go Version: " + info.Version.GoVersion + "\n" +
		"\n" +
		"  Command line: " + strings.Join(info.CmdLine, " ") + "\n" +
		"  Pid: " + strconv.Itoa(info.Pid) + "\n" +
		"  Uptime: " + strconv.Itoa(info.Uptime) + "\n" +
		"  Mem alloc: " + fmt.Sprintf("%d", info.MemStats.Alloc) + "\n" +
		"  Hostname: " + info.Config.HostName + "\n" +
		"  Receiver Host: " + info.Config.ReceiverHost + "\n" +
		"  Receiver port: " + strconv.Itoa(info.Config.ReceiverPort) + "\n" +
		"  Statsd Host: " + info.Config.StatsdHost + "\n" +
		"  Statsd port: " + strconv.Itoa(info.Config.StatsdPort) + "\n" +
		"  API Endpoints: " + strings.Join(info.Config.APIEndpoints, ", ") + "\n" +
		"\n" +
		"  Spans received (1 min): " + strconv.Itoa(int(info.Receiver.SpansReceived)) + "\n" +
		"  Traces received (1 min): " + strconv.Itoa(int(info.Receiver.TracesReceived)) + "\n" +
		"  Spans dropped (1 min): " + strconv.Itoa(int(info.Receiver.SpansDropped)) + "\n" +
		"  Traces dropped (1 min): " + strconv.Itoa(int(info.Receiver.TracesDropped)) + "\n" +
		"\n"), true, nil
}
