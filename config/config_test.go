package config

import (
	"os"
	"strings"

	"github.com/stretchr/testify/assert"

	"testing"

	"github.com/go-ini/ini"
)

func TestGetStrArray(t *testing.T) {
	assert := assert.New(t)
	f, _ := ini.Load([]byte("[Main]\n\nports = 10,15,20,25"))
	conf := File{
		f,
		"some/path",
	}

	ports, err := conf.GetStrArray("Main", "ports", ',')
	assert.Nil(err)
	assert.Equal(ports, []string{"10", "15", "20", "25"})
}

func TestGetStrArrayWithCommas(t *testing.T) {
	assert := assert.New(t)
	f, _ := ini.Load([]byte("[trace.ignore]\n\nresource = \"x,y,z\", foobar"))
	conf := File{f, "some/path"}

	vals, err := conf.GetStrArray("trace.ignore", "resource", ',')
	assert.Nil(err)
	assert.Equal(vals, []string{"x,y,z", "foobar"})
}

func TestGetStrArrayWithEscapedSequences(t *testing.T) {
	assert := assert.New(t)
	f, _ := ini.Load([]byte("[trace.ignore]\n\nresource = \"foo\\.bar\", \"\"\""))
	conf := File{f, "some/path"}

	vals, err := conf.GetStrArray("trace.ignore", "resource", ',')
	assert.Nil(err)
	assert.Equal(vals, []string{`foo\.bar`, `"`})
}

func TestGetStrArrayEmpty(t *testing.T) {
	assert := assert.New(t)
	f, _ := ini.Load([]byte("[Main]\n\nports = "))
	conf := File{
		f,
		"some/path",
	}

	ports, err := conf.GetStrArray("Main", "ports", ',')
	assert.Nil(err)
	assert.Equal([]string{}, ports)
}

func TestDefaultConfig(t *testing.T) {
	assert := assert.New(t)
	agentConfig := NewDefaultAgentConfig()

	// assert that some sane defaults are set
	assert.Equal(agentConfig.ReceiverHost, "localhost")
	assert.Equal(agentConfig.ReceiverPort, 8126)

	assert.Equal(agentConfig.StatsdHost, "localhost")
	assert.Equal(agentConfig.StatsdPort, 8125)

	assert.Equal(agentConfig.LogLevel, "INFO")
}

func TestOnlyEnvConfig(t *testing.T) {
	// setting an API Key should be enough to generate valid config
	os.Setenv("DD_API_KEY", "apikey_from_env")

	agentConfig, _ := NewAgentConfig(nil, nil)
	assert.Equal(t, "apikey_from_env", agentConfig.APIKey)

	os.Setenv("DD_API_KEY", "")
}

func TestOnlyDDAgentConfig(t *testing.T) {
	assert := assert.New(t)

	// absent an override by legacy config, reading from dd-agent config should do the right thing
	ddAgentConf, _ := ini.Load([]byte(strings.Join([]string{
		"[Main]",
		"hostname = thing",
		"api_key = apikey_12",
		"bind_host = 0.0.0.0",
		"dogstatsd_port = 28125",
		"log_level = DEBUG",
	}, "\n")))
	configFile := &File{instance: ddAgentConf, Path: "whatever"}
	agentConfig, _ := NewAgentConfig(configFile, nil)

	assert.Equal("thing", agentConfig.HostName)
	assert.Equal("apikey_12", agentConfig.APIKey)
	assert.Equal("0.0.0.0", agentConfig.ReceiverHost)
	assert.Equal(28125, agentConfig.StatsdPort)
	assert.Equal("DEBUG", agentConfig.LogLevel)
}

func TestDDAgentMultiAPIKeys(t *testing.T) {
	assert := assert.New(t)
	ddAgentConf, _ := ini.Load([]byte("[Main]\n\napi_key=foo, bar "))
	configFile := &File{instance: ddAgentConf, Path: "whatever"}

	agentConfig, _ := NewAgentConfig(configFile, nil)
	assert.Equal("foo", agentConfig.APIKey)
}

func TestDDAgentConfigWithLegacy(t *testing.T) {
	assert := assert.New(t)

	defaultConfig := NewDefaultAgentConfig()

	// check that legacy conf file overrides dd-agent.conf
	dd, _ := ini.Load([]byte("[Main]\n\nhostname=thing\napi_key=apikey_12"))
	legacy, _ := ini.Load([]byte(strings.Join([]string{
		"[trace.api]",
		"api_key = pommedapi",
		"endpoint = an_endpoint",
		"[trace.concentrator]",
		"extra_aggregators=region,error",
		"[trace.sampler]",
		"extra_sample_rate=0.33",
	}, "\n")))

	conf := &File{instance: dd, Path: "whatever"}
	legacyConf := &File{instance: legacy, Path: "whatever"}

	agentConfig, _ := NewAgentConfig(conf, legacyConf)

	// Properly loaded attributes
	assert.Equal("pommedapi", agentConfig.APIKey)
	assert.Equal("an_endpoint", agentConfig.APIEndpoint)

	// ExtraAggregators contains Datadog defaults + user-specified aggregators
	assert.Equal([]string{"http.status_code", "region", "error"}, agentConfig.ExtraAggregators)
	assert.Equal(0.33, agentConfig.ExtraSampleRate)

	// Check some defaults
	assert.Equal(defaultConfig.BucketInterval, agentConfig.BucketInterval)
	assert.Equal(defaultConfig.StatsdHost, agentConfig.StatsdHost)
}

func TestDDAgentConfigWithNewOpts(t *testing.T) {
	assert := assert.New(t)
	// check that providing trace.* options in the dd-agent conf file works
	dd, _ := ini.Load([]byte(strings.Join([]string{
		"[Main]",
		"hostname = thing",
		"api_key = apikey_12",
		"[trace.concentrator]",
		"extra_aggregators=region,error",
		"[trace.sampler]",
		"extra_sample_rate=0.33",
	}, "\n")))

	conf := &File{instance: dd, Path: "whatever"}
	agentConfig, _ := NewAgentConfig(conf, nil)

	// ExtraAggregators contains Datadog defaults + user-specified aggregators
	assert.Equal([]string{"http.status_code", "region", "error"}, agentConfig.ExtraAggregators)
	assert.Equal(0.33, agentConfig.ExtraSampleRate)
}

func TestEmptyExtraAggregatorsFromConfig(t *testing.T) {
	assert := assert.New(t)

	// providing empty extra_aggregators leaves the Datadog default in place
	dd, _ := ini.Load([]byte(strings.Join([]string{
		"[Main]",
		"hostname = thing",
		"api_key = apikey_12",
		"[trace.concentrator]",
		"extra_aggregators = ",
	}, "\n")))

	conf := &File{instance: dd, Path: "whatever"}
	agentConfig, _ := NewAgentConfig(conf, nil)
	assert.Equal([]string{"http.status_code"}, agentConfig.ExtraAggregators)
}

func TestConfigNewIfExists(t *testing.T) {
	// The file does not exist: no error returned
	conf, err := NewIfExists("/does-not-exist")
	assert.Nil(t, err)
	assert.Nil(t, conf)

	// The file exists but cannot be read for another reason: an error is
	// returned.
	filename := "/tmp/trace-agent-test-config.ini"
	os.Remove(filename)
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0200) // write only
	assert.Nil(t, err)
	f.Close()
	conf, err = NewIfExists(filename)
	assert.NotNil(t, err)
	assert.Nil(t, conf)
	os.Remove(filename)
}

func TestGetHostname(t *testing.T) {
	h, err := getHostname()
	assert.Nil(t, err)
	assert.NotEqual(t, "", h)
}

func TestAnalyzedRateByService(t *testing.T) {
	assert := assert.New(t)
	config, _ := ini.Load([]byte(strings.Join([]string{
		"[trace.analyzed_rate_by_service]",
		"web = 0.8",
		"intake = 0.05",
		"bad_service = ",
	}, "\n")))

	conf := &File{instance: config, Path: "whatever"}
	agentConfig, _ := NewAgentConfig(conf, nil)

	assert.Equal(agentConfig.AnalyzedRateByService["web"], 0.8)
	assert.Equal(agentConfig.AnalyzedRateByService["intake"], 0.05)
}
