package main

import (
	"bytes"
	"encoding/json"
	"expvar"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/stretchr/testify/assert"
)

type testServerHandler struct {
	t *testing.T
}

func (h *testServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/debug/vars":
		h.t.Logf("serving fake (static) info data for %s", r.URL.Path)
		_, err := w.Write([]byte(`{
"cmdline": ["./trace-agent"],
"config": {"Enabled":true,"HostName":"localhost.localdomain","DefaultEnv":"none","APIEndpoints":["https://trace.agent.datadoghq.com"],"APIEnabled":true,"APIPayloadBufferMaxSize":16777216,"BucketInterval":10000000000,"ExtraAggregators":[],"ExtraSampleRate":1,"MaxTPS":10,"ReceiverHost":"localhost","ReceiverPort":8126,"ConnectionLimit":2000,"ReceiverTimeout":0,"StatsdHost":"127.0.0.1","StatsdPort":8125,"LogLevel":"INFO","LogFilePath":"/var/log/datadog/trace-agent.log"},
"endpoint": {"TracesPayload":4,"TracesPayloadError":0,"TracesBytes":3245,"TracesCount":6,"TracesStats":60,"ServicesPayload":2,"ServicesPayloadError":0,"ServicesBytes":346},
"memstats": {"Alloc":773552,"TotalAlloc":773552,"Sys":3346432,"Lookups":6,"Mallocs":7231,"Frees":561,"HeapAlloc":773552,"HeapSys":1572864,"HeapIdle":49152,"HeapInuse":1523712,"HeapReleased":0,"HeapObjects":6670,"StackInuse":524288,"StackSys":524288,"MSpanInuse":24480,"MSpanSys":32768,"MCacheInuse":4800,"MCacheSys":16384,"BuckHashSys":2675,"GCSys":131072,"OtherSys":1066381,"NextGC":4194304,"LastGC":0,"PauseTotalNs":0,"PauseNs":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"PauseEnd":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"NumGC":0,"GCCPUFraction":0,"EnableGC":true,"DebugGC":false,"BySize":[{"Size":0,"Mallocs":0,"Frees":0},{"Size":8,"Mallocs":126,"Frees":0},{"Size":16,"Mallocs":825,"Frees":0},{"Size":32,"Mallocs":4208,"Frees":0},{"Size":48,"Mallocs":345,"Frees":0},{"Size":64,"Mallocs":262,"Frees":0},{"Size":80,"Mallocs":93,"Frees":0},{"Size":96,"Mallocs":70,"Frees":0},{"Size":112,"Mallocs":97,"Frees":0},{"Size":128,"Mallocs":24,"Frees":0},{"Size":144,"Mallocs":25,"Frees":0},{"Size":160,"Mallocs":57,"Frees":0},{"Size":176,"Mallocs":128,"Frees":0},{"Size":192,"Mallocs":13,"Frees":0},{"Size":208,"Mallocs":77,"Frees":0},{"Size":224,"Mallocs":3,"Frees":0},{"Size":240,"Mallocs":2,"Frees":0},{"Size":256,"Mallocs":17,"Frees":0},{"Size":288,"Mallocs":64,"Frees":0},{"Size":320,"Mallocs":12,"Frees":0},{"Size":352,"Mallocs":20,"Frees":0},{"Size":384,"Mallocs":1,"Frees":0},{"Size":416,"Mallocs":59,"Frees":0},{"Size":448,"Mallocs":0,"Frees":0},{"Size":480,"Mallocs":3,"Frees":0},{"Size":512,"Mallocs":2,"Frees":0},{"Size":576,"Mallocs":17,"Frees":0},{"Size":640,"Mallocs":6,"Frees":0},{"Size":704,"Mallocs":10,"Frees":0},{"Size":768,"Mallocs":0,"Frees":0},{"Size":896,"Mallocs":11,"Frees":0},{"Size":1024,"Mallocs":11,"Frees":0},{"Size":1152,"Mallocs":12,"Frees":0},{"Size":1280,"Mallocs":2,"Frees":0},{"Size":1408,"Mallocs":2,"Frees":0},{"Size":1536,"Mallocs":0,"Frees":0},{"Size":1664,"Mallocs":10,"Frees":0},{"Size":2048,"Mallocs":17,"Frees":0},{"Size":2304,"Mallocs":7,"Frees":0},{"Size":2560,"Mallocs":1,"Frees":0},{"Size":2816,"Mallocs":1,"Frees":0},{"Size":3072,"Mallocs":1,"Frees":0},{"Size":3328,"Mallocs":7,"Frees":0},{"Size":4096,"Mallocs":4,"Frees":0},{"Size":4608,"Mallocs":1,"Frees":0},{"Size":5376,"Mallocs":6,"Frees":0},{"Size":6144,"Mallocs":4,"Frees":0},{"Size":6400,"Mallocs":0,"Frees":0},{"Size":6656,"Mallocs":1,"Frees":0},{"Size":6912,"Mallocs":0,"Frees":0},{"Size":8192,"Mallocs":0,"Frees":0},{"Size":8448,"Mallocs":0,"Frees":0},{"Size":8704,"Mallocs":1,"Frees":0},{"Size":9472,"Mallocs":0,"Frees":0},{"Size":10496,"Mallocs":0,"Frees":0},{"Size":12288,"Mallocs":1,"Frees":0},{"Size":13568,"Mallocs":0,"Frees":0},{"Size":14080,"Mallocs":0,"Frees":0},{"Size":16384,"Mallocs":0,"Frees":0},{"Size":16640,"Mallocs":0,"Frees":0},{"Size":17664,"Mallocs":1,"Frees":0}]},
"pid": 38149,
"receiver": {"TracesBytes":10000,"ServicesBytes":1000,"SpansReceived":360,"TracesReceived":240,"SpansDropped":0,"TracesDropped":0},
"presampler": {"Rate":1.0},
"uptime": 15,
"version": {"BuildDate": "2017-02-01T14:28:10+0100", "GitBranch": "ufoot/statusinfo", "GitCommit": "396a217", "GoVersion": "go version go1.7 darwin/amd64", "Version": "0.99.0"}
}`))
		if err != nil {
			h.t.Errorf("error serving %s: %v", r.URL.Path, err)
		}
	default:
		h.t.Logf("answering 404 for %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func testServer(t *testing.T) *httptest.Server {
	server := httptest.NewServer(&testServerHandler{t: t})
	t.Logf("test server (serving fake yet valid data) listening on %s", server.URL)
	return server
}

type testServerWarningHandler struct {
	t *testing.T
}

func (h *testServerWarningHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/debug/vars":
		h.t.Logf("serving fake (static) info data for %s", r.URL.Path)
		_, err := w.Write([]byte(`{
"cmdline": ["./trace-agent"],
"config": {"Enabled":true,"HostName":"localhost.localdomain","DefaultEnv":"none","APIEndpoints":["https://trace.agent.datadoghq.com"],"APIEnabled":true,"APIPayloadBufferMaxSize":16777216,"BucketInterval":10000000000,"ExtraAggregators":[],"ExtraSampleRate":1,"MaxTPS":10,"ReceiverHost":"localhost","ReceiverPort":8126,"ConnectionLimit":2000,"ReceiverTimeout":0,"StatsdHost":"127.0.0.1","StatsdPort":8125,"LogLevel":"INFO","LogFilePath":"/var/log/datadog/trace-agent.log"},
"endpoint": {"TracesPayload":4,"TracesPayloadError":3,"TracesBytes":3245,"TracesCount":6,"TracesStats":60,"ServicesPayload":2,"ServicesPayloadError":1,"ServicesBytes":346},
"memstats": {"Alloc":773552,"TotalAlloc":773552,"Sys":3346432,"Lookups":6,"Mallocs":7231,"Frees":561,"HeapAlloc":773552,"HeapSys":1572864,"HeapIdle":49152,"HeapInuse":1523712,"HeapReleased":0,"HeapObjects":6670,"StackInuse":524288,"StackSys":524288,"MSpanInuse":24480,"MSpanSys":32768,"MCacheInuse":4800,"MCacheSys":16384,"BuckHashSys":2675,"GCSys":131072,"OtherSys":1066381,"NextGC":4194304,"LastGC":0,"PauseTotalNs":0,"PauseNs":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"PauseEnd":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"NumGC":0,"GCCPUFraction":0,"EnableGC":true,"DebugGC":false,"BySize":[{"Size":0,"Mallocs":0,"Frees":0},{"Size":8,"Mallocs":126,"Frees":0},{"Size":16,"Mallocs":825,"Frees":0},{"Size":32,"Mallocs":4208,"Frees":0},{"Size":48,"Mallocs":345,"Frees":0},{"Size":64,"Mallocs":262,"Frees":0},{"Size":80,"Mallocs":93,"Frees":0},{"Size":96,"Mallocs":70,"Frees":0},{"Size":112,"Mallocs":97,"Frees":0},{"Size":128,"Mallocs":24,"Frees":0},{"Size":144,"Mallocs":25,"Frees":0},{"Size":160,"Mallocs":57,"Frees":0},{"Size":176,"Mallocs":128,"Frees":0},{"Size":192,"Mallocs":13,"Frees":0},{"Size":208,"Mallocs":77,"Frees":0},{"Size":224,"Mallocs":3,"Frees":0},{"Size":240,"Mallocs":2,"Frees":0},{"Size":256,"Mallocs":17,"Frees":0},{"Size":288,"Mallocs":64,"Frees":0},{"Size":320,"Mallocs":12,"Frees":0},{"Size":352,"Mallocs":20,"Frees":0},{"Size":384,"Mallocs":1,"Frees":0},{"Size":416,"Mallocs":59,"Frees":0},{"Size":448,"Mallocs":0,"Frees":0},{"Size":480,"Mallocs":3,"Frees":0},{"Size":512,"Mallocs":2,"Frees":0},{"Size":576,"Mallocs":17,"Frees":0},{"Size":640,"Mallocs":6,"Frees":0},{"Size":704,"Mallocs":10,"Frees":0},{"Size":768,"Mallocs":0,"Frees":0},{"Size":896,"Mallocs":11,"Frees":0},{"Size":1024,"Mallocs":11,"Frees":0},{"Size":1152,"Mallocs":12,"Frees":0},{"Size":1280,"Mallocs":2,"Frees":0},{"Size":1408,"Mallocs":2,"Frees":0},{"Size":1536,"Mallocs":0,"Frees":0},{"Size":1664,"Mallocs":10,"Frees":0},{"Size":2048,"Mallocs":17,"Frees":0},{"Size":2304,"Mallocs":7,"Frees":0},{"Size":2560,"Mallocs":1,"Frees":0},{"Size":2816,"Mallocs":1,"Frees":0},{"Size":3072,"Mallocs":1,"Frees":0},{"Size":3328,"Mallocs":7,"Frees":0},{"Size":4096,"Mallocs":4,"Frees":0},{"Size":4608,"Mallocs":1,"Frees":0},{"Size":5376,"Mallocs":6,"Frees":0},{"Size":6144,"Mallocs":4,"Frees":0},{"Size":6400,"Mallocs":0,"Frees":0},{"Size":6656,"Mallocs":1,"Frees":0},{"Size":6912,"Mallocs":0,"Frees":0},{"Size":8192,"Mallocs":0,"Frees":0},{"Size":8448,"Mallocs":0,"Frees":0},{"Size":8704,"Mallocs":1,"Frees":0},{"Size":9472,"Mallocs":0,"Frees":0},{"Size":10496,"Mallocs":0,"Frees":0},{"Size":12288,"Mallocs":1,"Frees":0},{"Size":13568,"Mallocs":0,"Frees":0},{"Size":14080,"Mallocs":0,"Frees":0},{"Size":16384,"Mallocs":0,"Frees":0},{"Size":16640,"Mallocs":0,"Frees":0},{"Size":17664,"Mallocs":1,"Frees":0}]},
"pid": 38149,
"receiver": {"TracesBytes":10000,"ServicesBytes":1000,"SpansReceived":360,"TracesReceived":240,"SpansDropped":10,"TracesDropped":5},
"presampler": {"Rate":0.421},
"uptime": 15,
"version": {"BuildDate": "2017-02-01T14:28:10+0100", "GitBranch": "ufoot/statusinfo", "GitCommit": "396a217", "GoVersion": "go version go1.7 darwin/amd64", "Version": "0.99.0"}
}`))
		if err != nil {
			h.t.Errorf("error serving %s: %v", r.URL.Path, err)
		}
	default:
		h.t.Logf("answering 404 for %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func testServerWarning(t *testing.T) *httptest.Server {
	server := httptest.NewServer(&testServerWarningHandler{t: t})
	t.Logf("test server (serving data containing worrying values) listening on %s", server.URL)
	return server
}

type testServerErrorHandler struct {
	t *testing.T
}

func (h *testServerErrorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	switch r.URL.Path {
	case "/debug/vars":
		h.t.Logf("serving fake (static) info data for %s", r.URL.Path)
		_, err := w.Write([]byte(`this is *NOT* a valid JSON, no way...`))
		if err != nil {
			h.t.Errorf("error serving %s: %v", r.URL.Path, err)
		}
	default:
		h.t.Logf("answering 404 for %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func testServerError(t *testing.T) *httptest.Server {
	server := httptest.NewServer(&testServerErrorHandler{t: t})
	t.Logf("test server (serving bad data to trigger errors) listening on %s", server.URL)
	return server
}

// run this at the beginning of each test, this is because we *really*
// need to have initInfo be called before doing anything
func testInit(t *testing.T) *config.AgentConfig {
	assert := assert.New(t)
	conf := config.NewDefaultAgentConfig()
	assert.NotNil(conf)

	conf.APIKeys = append(conf.APIKeys, "ooops")
	assert.NotNil(conf.APIKeys, "API Keys should be defined in upstream conf")
	err := initInfo(conf)
	assert.Nil(err)

	return conf
}

func TestInfo(t *testing.T) {
	assert := assert.New(t)
	conf := testInit(t)
	assert.NotNil(conf)

	server := testServer(t)
	assert.NotNil(server)
	defer server.Close()

	url, err := url.Parse(server.URL)
	assert.NotNil(url)
	assert.Nil(err)

	hostPort := strings.Split(url.Host, ":")
	assert.Equal(2, len(hostPort))
	port, err := strconv.Atoi(hostPort[1])
	assert.Nil(err)
	conf.ReceiverPort = port

	var buf bytes.Buffer
	err = Info(&buf, conf)
	assert.Nil(err)
	info := buf.String()

	t.Logf("Info:\n%s\n", info)

	lines := strings.Split(info, "\n")
	assert.Equal(21, len(lines))
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[0])
	assert.Regexp(regexp.MustCompile(`^Trace Agent \(v.*\)$`), lines[1])
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[2])
	assert.Equal(len(lines[1]), len(lines[0]))
	assert.Equal(len(lines[1]), len(lines[2]))
	assert.Equal("", lines[3])
	assert.Equal("  Pid: 38149", lines[4])
	assert.Equal("  Uptime: 15 seconds", lines[5])
	assert.Equal("  Mem alloc: 773552 bytes", lines[6])
	assert.Equal("", lines[7])
	assert.Equal("  Hostname: localhost.localdomain", lines[8])
	assert.Equal("  Receiver: localhost:8126", lines[9])
	assert.Equal("  API Endpoints: https://trace.agent.datadoghq.com", lines[10])
	assert.Equal("", lines[11])
	assert.Equal("  Bytes received (1 min): 11000", lines[12])
	assert.Equal("  Traces received (1 min): 240", lines[13])
	assert.Equal("  Spans received (1 min): 360", lines[14])
	assert.Equal("", lines[15])
	assert.Equal("  Bytes sent (1 min): 3591", lines[16])
	assert.Equal("  Traces sent (1 min): 6", lines[17])
	assert.Equal("  Stats sent (1 min): 60", lines[18])
	assert.Equal("", lines[19])
	assert.Equal("", lines[20])
}

func TestWarning(t *testing.T) {
	assert := assert.New(t)
	conf := testInit(t)
	assert.NotNil(conf)

	server := testServerWarning(t)
	assert.NotNil(server)
	defer server.Close()

	url, err := url.Parse(server.URL)
	assert.NotNil(url)
	assert.Nil(err)

	hostPort := strings.Split(url.Host, ":")
	assert.Equal(2, len(hostPort))
	port, err := strconv.Atoi(hostPort[1])
	assert.Nil(err)
	conf.ReceiverPort = port

	var buf bytes.Buffer
	err = Info(&buf, conf)
	assert.Nil(err)
	info := buf.String()

	t.Logf("Info:\n%s\n", info)

	lines := strings.Split(info, "\n")
	assert.Equal(26, len(lines))
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[0])
	assert.Regexp(regexp.MustCompile(`^Trace Agent \(v.*\)$`), lines[1])
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[2])
	assert.Equal(len(lines[1]), len(lines[0]))
	assert.Equal(len(lines[1]), len(lines[2]))
	assert.Equal("", lines[3])
	assert.Equal("  Pid: 38149", lines[4])
	assert.Equal("  Uptime: 15 seconds", lines[5])
	assert.Equal("  Mem alloc: 773552 bytes", lines[6])
	assert.Equal("", lines[7])
	assert.Equal("  Hostname: localhost.localdomain", lines[8])
	assert.Equal("  Receiver: localhost:8126", lines[9])
	assert.Equal("  API Endpoints: https://trace.agent.datadoghq.com", lines[10])
	assert.Equal("", lines[11])
	assert.Equal("  Bytes received (1 min): 11000", lines[12])
	assert.Equal("  Traces received (1 min): 240", lines[13])
	assert.Equal("  Spans received (1 min): 360", lines[14])
	assert.Equal("  WARNING: Traces dropped (1 min): 5", lines[15])
	assert.Equal("  WARNING: Spans dropped (1 min): 10", lines[16])
	assert.Equal("  WARNING: Pre-sampling traces: 42.1 %", lines[17])
	assert.Equal("", lines[18])
	assert.Equal("  Bytes sent (1 min): 3591", lines[19])
	assert.Equal("  Traces sent (1 min): 6", lines[20])
	assert.Equal("  Stats sent (1 min): 60", lines[21])
	assert.Equal("  WARNING: Traces API errors (1 min): 3/4", lines[22])
	assert.Equal("  WARNING: Services API errors (1 min): 1/2", lines[23])
	assert.Equal("", lines[24])
	assert.Equal("", lines[25])
}

func TestNotRunning(t *testing.T) {
	assert := assert.New(t)
	conf := testInit(t)
	assert.NotNil(conf)

	server := testServer(t)
	assert.NotNil(server)

	url, err := url.Parse(server.URL)
	assert.NotNil(url)
	assert.Nil(err)

	server.Close()

	hostPort := strings.Split(url.Host, ":")
	assert.Equal(2, len(hostPort))
	port, err := strconv.Atoi(hostPort[1])
	assert.Nil(err)
	conf.ReceiverPort = port

	var buf bytes.Buffer
	err = Info(&buf, conf)
	assert.NotNil(err)
	info := buf.String()

	t.Logf("Info:\n%s\n", info)

	lines := strings.Split(info, "\n")
	assert.Equal(7, len(lines))
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[0])
	assert.Regexp(regexp.MustCompile(`^Trace Agent \(v.*\)$`), lines[1])
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[2])
	assert.Equal(len(lines[1]), len(lines[0]))
	assert.Equal(len(lines[1]), len(lines[2]))
	assert.Equal("", lines[3])
	assert.Equal(fmt.Sprintf("  Not running (port %d)", port), lines[4])
	assert.Equal("", lines[5])
	assert.Equal("", lines[6])
}

func TestError(t *testing.T) {
	assert := assert.New(t)
	conf := testInit(t)
	assert.NotNil(conf)

	server := testServerError(t)
	assert.NotNil(server)
	defer server.Close()

	url, err := url.Parse(server.URL)
	assert.NotNil(url)
	assert.Nil(err)

	hostPort := strings.Split(url.Host, ":")
	assert.Equal(2, len(hostPort))
	port, err := strconv.Atoi(hostPort[1])
	assert.Nil(err)
	conf.ReceiverPort = port

	var buf bytes.Buffer
	err = Info(&buf, conf)
	assert.NotNil(err)
	info := buf.String()

	t.Logf("Info:\n%s\n", info)

	lines := strings.Split(info, "\n")
	assert.Equal(8, len(lines))
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[0])
	assert.Regexp(regexp.MustCompile(`^Trace Agent \(v.*\)$`), lines[1])
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[2])
	assert.Equal(len(lines[1]), len(lines[0]))
	assert.Equal(len(lines[1]), len(lines[2]))
	assert.Equal("", lines[3])
	assert.Regexp(regexp.MustCompile(`^  Error: .*$`), lines[4])
	assert.Equal(fmt.Sprintf("  URL: http://localhost:%d/debug/vars", port), lines[5])
	assert.Equal("", lines[6])
	assert.Equal("", lines[7])
}

func TestInfoReceiverStats(t *testing.T) {
	assert := assert.New(t)
	conf := testInit(t)
	assert.NotNil(conf)

	stats := receiverStats{
		SpansReceived:  1000,
		TracesReceived: 100,
		SpansDropped:   5,
		TracesDropped:  2,
	}
	// run this with -race flag
	done := make(chan struct{}, 4)
	for i := 0; i < 2; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				updateReceiverStats(stats)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 2; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				_ = publishReceiverStats()
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
	s := publishReceiverStats()
	switch s := s.(type) {
	case receiverStats:
		assert.Equal(int64(1000), s.SpansReceived)
		assert.Equal(int64(100), s.TracesReceived)
		assert.Equal(int64(5), s.SpansDropped)
		assert.Equal(int64(2), s.TracesDropped)
		buf, err := json.Marshal(&s)
		assert.NotNil(buf)
		assert.Nil(err)
		var statsCopy receiverStats
		err = json.Unmarshal(buf, &statsCopy)
		assert.Nil(err)
		assert.Equal(stats, statsCopy)
	default:
		t.Errorf("bad stats type: %v", s)
	}
	stats.SpansReceived++
	updateReceiverStats(stats)
	s = publishReceiverStats()
	switch s := s.(type) {
	case receiverStats:
		assert.Equal(int64(1001), s.SpansReceived)
	default:
		t.Errorf("bad stats type: %v", s)
	}
}

func TestInfoConfig(t *testing.T) {
	assert := assert.New(t)
	conf := testInit(t)
	assert.NotNil(conf)

	js := expvar.Get("config").String() // this is what expvar will call
	assert.NotEqual("", js)
	var confCopy config.AgentConfig
	err := json.Unmarshal([]byte(js), &confCopy)
	assert.Nil(err)
	assert.Nil(confCopy.APIKeys, "API Keys should *NEVER* be exported")
	conf.APIKeys = nil            // patch upstream source so that we can use equality testing
	assert.Equal(*conf, confCopy) // ensure all fields have been exported then parsed correctly
}
