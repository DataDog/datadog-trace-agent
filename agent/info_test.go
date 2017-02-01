package main

import (
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
"config": {"Enabled":true,"HostName":"localhost.localdomain","DefaultEnv":"none","APIEndpoints":["https://trace.agent.datadoghq.com"],"APIEnabled":true,"APIPayloadBufferMaxSize":16777216,"BucketInterval":10000000000,"ExtraAggregators":[],"ExtraSampleRate":1,"MaxTPS":10,"ReceiverHost":"localhost","ReceiverPort":7777,"ConnectionLimit":2000,"ReceiverTimeout":0,"StatsdHost":"localhost","StatsdPort":8125,"LogLevel":"INFO","LogFilePath":"/var/log/datadog/trace-agent.log"},
"memstats": {"Alloc":773552,"TotalAlloc":773552,"Sys":3346432,"Lookups":6,"Mallocs":7231,"Frees":561,"HeapAlloc":773552,"HeapSys":1572864,"HeapIdle":49152,"HeapInuse":1523712,"HeapReleased":0,"HeapObjects":6670,"StackInuse":524288,"StackSys":524288,"MSpanInuse":24480,"MSpanSys":32768,"MCacheInuse":4800,"MCacheSys":16384,"BuckHashSys":2675,"GCSys":131072,"OtherSys":1066381,"NextGC":4194304,"LastGC":0,"PauseTotalNs":0,"PauseNs":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"PauseEnd":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"NumGC":0,"GCCPUFraction":0,"EnableGC":true,"DebugGC":false,"BySize":[{"Size":0,"Mallocs":0,"Frees":0},{"Size":8,"Mallocs":126,"Frees":0},{"Size":16,"Mallocs":825,"Frees":0},{"Size":32,"Mallocs":4208,"Frees":0},{"Size":48,"Mallocs":345,"Frees":0},{"Size":64,"Mallocs":262,"Frees":0},{"Size":80,"Mallocs":93,"Frees":0},{"Size":96,"Mallocs":70,"Frees":0},{"Size":112,"Mallocs":97,"Frees":0},{"Size":128,"Mallocs":24,"Frees":0},{"Size":144,"Mallocs":25,"Frees":0},{"Size":160,"Mallocs":57,"Frees":0},{"Size":176,"Mallocs":128,"Frees":0},{"Size":192,"Mallocs":13,"Frees":0},{"Size":208,"Mallocs":77,"Frees":0},{"Size":224,"Mallocs":3,"Frees":0},{"Size":240,"Mallocs":2,"Frees":0},{"Size":256,"Mallocs":17,"Frees":0},{"Size":288,"Mallocs":64,"Frees":0},{"Size":320,"Mallocs":12,"Frees":0},{"Size":352,"Mallocs":20,"Frees":0},{"Size":384,"Mallocs":1,"Frees":0},{"Size":416,"Mallocs":59,"Frees":0},{"Size":448,"Mallocs":0,"Frees":0},{"Size":480,"Mallocs":3,"Frees":0},{"Size":512,"Mallocs":2,"Frees":0},{"Size":576,"Mallocs":17,"Frees":0},{"Size":640,"Mallocs":6,"Frees":0},{"Size":704,"Mallocs":10,"Frees":0},{"Size":768,"Mallocs":0,"Frees":0},{"Size":896,"Mallocs":11,"Frees":0},{"Size":1024,"Mallocs":11,"Frees":0},{"Size":1152,"Mallocs":12,"Frees":0},{"Size":1280,"Mallocs":2,"Frees":0},{"Size":1408,"Mallocs":2,"Frees":0},{"Size":1536,"Mallocs":0,"Frees":0},{"Size":1664,"Mallocs":10,"Frees":0},{"Size":2048,"Mallocs":17,"Frees":0},{"Size":2304,"Mallocs":7,"Frees":0},{"Size":2560,"Mallocs":1,"Frees":0},{"Size":2816,"Mallocs":1,"Frees":0},{"Size":3072,"Mallocs":1,"Frees":0},{"Size":3328,"Mallocs":7,"Frees":0},{"Size":4096,"Mallocs":4,"Frees":0},{"Size":4608,"Mallocs":1,"Frees":0},{"Size":5376,"Mallocs":6,"Frees":0},{"Size":6144,"Mallocs":4,"Frees":0},{"Size":6400,"Mallocs":0,"Frees":0},{"Size":6656,"Mallocs":1,"Frees":0},{"Size":6912,"Mallocs":0,"Frees":0},{"Size":8192,"Mallocs":0,"Frees":0},{"Size":8448,"Mallocs":0,"Frees":0},{"Size":8704,"Mallocs":1,"Frees":0},{"Size":9472,"Mallocs":0,"Frees":0},{"Size":10496,"Mallocs":0,"Frees":0},{"Size":12288,"Mallocs":1,"Frees":0},{"Size":13568,"Mallocs":0,"Frees":0},{"Size":14080,"Mallocs":0,"Frees":0},{"Size":16384,"Mallocs":0,"Frees":0},{"Size":16640,"Mallocs":0,"Frees":0},{"Size":17664,"Mallocs":1,"Frees":0}]},
"pid": 38149,
"receiver": {"SpansReceived":100,"TracesReceived":10,"SpansDropped":1,"TracesDropped":3},
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
	t.Logf("test server listening on %s", server.URL)
	return server
}

func TestInfo(t *testing.T) {
	assert := assert.New(t)
	conf := config.NewDefaultAgentConfig()
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

	info, running, err := Info(conf)
	assert.Nil(err)
	assert.Equal(true, running)

	t.Logf("Info:\n%s\n", info)

	lines := strings.Split(info, "\n")
	assert.Equal(27, len(lines))
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[0])
	assert.Regexp(regexp.MustCompile(`^Trace Agent \(v.*\)$`), lines[1])
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[2])
	assert.Equal(len(lines[1]), len(lines[0]))
	assert.Equal(len(lines[1]), len(lines[2]))
	assert.Equal("", lines[3])
	assert.Equal("  Version: 0.99.0", lines[4])
	assert.Equal("  Git hash: 396a217", lines[5])
	assert.Equal("  Git branch: ufoot/statusinfo", lines[6])
	assert.Equal("  Build date: 2017-02-01T14:28:10+0100", lines[7])
	assert.Equal("  Go Version: go version go1.7 darwin/amd64", lines[8])
	assert.Equal("", lines[9])
	assert.Regexp(regexp.MustCompile(`^  Command line: .*agent`), lines[10])
	assert.Equal("  Pid: 38149", lines[11])
	assert.Equal("  Uptime: 15", lines[12])
	assert.Equal("  Mem alloc: 773552", lines[13])
	assert.Equal("  Hostname: localhost.localdomain", lines[14])
	assert.Equal("  Receiver Host: localhost", lines[15])
	assert.Regexp(regexp.MustCompile(`^  Receiver port: \d+`), lines[16])
	assert.Equal("  Statsd Host: localhost", lines[17])
	assert.Equal("  Statsd port: 8125", lines[18])
	assert.Equal("  API Endpoints: https://trace.agent.datadoghq.com", lines[19])
	assert.Equal("", lines[20])
	assert.Equal("  Spans received (1 min): 100", lines[21])
	assert.Equal("  Traces received (1 min): 10", lines[22])
	assert.Equal("  Spans dropped (1 min): 1", lines[23])
	assert.Equal("  Traces dropped (1 min): 3", lines[24])
	assert.Equal("", lines[25])
	assert.Equal("", lines[26])
}

func TestNotRunning(t *testing.T) {
	assert := assert.New(t)
	conf := config.NewDefaultAgentConfig()
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

	info, running, err := Info(conf)
	assert.Nil(err)
	assert.Equal(false, running)

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

func TestInfoReceiverStats(t *testing.T) {
	assert := assert.New(t)
	stats := ReceiverStats{
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
	case ReceiverStats:
		assert.Equal(1000, s.SpansReceived)
		assert.Equal(100, s.TracesReceived)
		assert.Equal(5, s.SpansDropped)
		assert.Equal(2, s.TracesDropped)
		buf, err := json.Marshal(&s)
		assert.NotNil(buf)
		assert.Nil(err)
		var statsCopy ReceiverStats
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
	case ReceiverStats:
		assert.Equal(1001, s.SpansReceived)
	default:
		t.Errorf("bad stats type: %v", s)
	}
}

func TestInfoConfig(t *testing.T) {
	assert := assert.New(t)
	conf := config.NewDefaultAgentConfig()
	assert.NotNil(conf)
	conf.APIKeys = append(conf.APIKeys, "ooops")
	assert.NotNil(conf.APIKeys, "API Keys should be defined in upstream conf")

	err := initInfo(conf)
	assert.Nil(err)

	js := expvar.Get("config").String() // this is what expvar will call
	assert.NotEqual("", js)
	var confCopy config.AgentConfig
	err = json.Unmarshal([]byte(js), &confCopy)
	assert.Nil(err)
	assert.Nil(confCopy.APIKeys, "API Keys should *NEVER* be exported")
	conf.APIKeys = nil            // patch upstream source so that we can use equality testing
	assert.Equal(*conf, confCopy) // ensure all fields have been exported then parsed correctly
}
