package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	assert := assert.New(t)
	agentConfig := NewDefaultAgentConfig()

	// assert that some sane defaults are set
	assert.Equal(agentConfig.ReceiverHost, "localhost")
	assert.Equal(agentConfig.ReceiverPort, 8126)

	assert.Equal(agentConfig.StatsdHost, "localhost")
	assert.Equal(agentConfig.StatsdPort, 8125)

	assert.Equal(agentConfig.LogLevel, "INFO")
	assert.Equal(agentConfig.Enabled, true)

}

func TestOnlyEnvConfig(t *testing.T) {
	// setting an API Key should be enough to generate valid config
	os.Setenv("DD_API_KEY", "apikey_from_env")

	agentConfig, _ := NewAgentConfig(nil, nil, nil)
	assert.Equal(t, "apikey_from_env", agentConfig.APIKey)

	os.Setenv("DD_API_KEY", "")
}

func TestOnlyDDAgentConfig(t *testing.T) {
	assert := assert.New(t)

	iniConf, err := NewIni("./test_cases/no_apm_config.ini")
	assert.NoError(err)

	agentConfig, err := NewAgentConfig(iniConf, nil, nil)
	assert.NoError(err)

	assert.Equal("thing", agentConfig.HostName)
	assert.Equal("apikey_12", agentConfig.APIKey)
	assert.Equal("0.0.0.0", agentConfig.ReceiverHost)
	assert.Equal(28125, agentConfig.StatsdPort)
	assert.Equal("DEBUG", agentConfig.LogLevel)
}

func TestDDAgentMultiAPIKeys(t *testing.T) {
	// old feature Datadog Agent feature, got dropped since
	// TODO: at some point, expire this case
	assert := assert.New(t)

	iniConf, err := NewIni("./test_cases/multi_api_keys.ini")
	assert.NoError(err)

	agentConfig, err := NewAgentConfig(iniConf, nil, nil)
	assert.NoError(err)

	assert.Equal("foo", agentConfig.APIKey)
}

func TestFullIniConfig(t *testing.T) {
	assert := assert.New(t)

	iniConf, err := NewIni("./test_cases/full.ini")
	assert.NoError(err, "failed to parse valid configuration")

	c, err := NewAgentConfig(iniConf, nil, nil)
	assert.NoError(err)

	assert.Equal("api_key_test", c.APIKey)
	assert.Equal("mymachine", c.HostName)
	assert.Equal("https://user:password@proxy_for_https:1234", c.ProxyURL.String())
	assert.Equal("https://datadog.unittests", c.APIEndpoint)
	assert.Equal(false, c.Enabled)
	assert.Equal("test", c.DefaultEnv)
	assert.Equal(18126, c.ReceiverPort)
	assert.Equal(0.5, c.ExtraSampleRate)
	assert.Equal(5.0, c.MaxTPS)
	assert.Equal("0.0.0.0", c.ReceiverHost)

	assert.EqualValues([]string{"/health", "/500"}, c.Ignore["resource"])
}

func TestFullYamlConfig(t *testing.T) {
	assert := assert.New(t)

	yamlConf, err := NewYamlIfExists("./test_cases/full.yaml")
	assert.NoError(err, "failed to parse valid configuration")

	c, err := NewAgentConfig(nil, nil, yamlConf)
	assert.NoError(err)

	assert.Equal("api_key_test", c.APIKey)
	assert.Equal("mymachine", c.HostName)
	assert.Equal("https://user:password@proxy_for_https:1234", c.ProxyURL.String())
	assert.Equal("https://datadog.unittests", c.APIEndpoint)
	assert.Equal(false, c.Enabled)
	assert.Equal("test", c.DefaultEnv)
	assert.Equal(18126, c.ReceiverPort)
	assert.Equal(0.5, c.ExtraSampleRate)
	assert.Equal(5.0, c.MaxTPS)
	assert.Equal("0.0.0.0", c.ReceiverHost)

	assert.EqualValues([]string{"/health", "/500"}, c.Ignore["resource"])
}

func TestUndocumentedYamlConfig(t *testing.T) {
	assert := assert.New(t)

	yamlConfig, err := NewYamlIfExists("./test_cases/undocumented.yaml")
	assert.NoError(err)

	agentConfig, err := NewAgentConfig(nil, nil, yamlConfig)
	assert.NoError(err)

	assert.Equal("thing", agentConfig.HostName)
	assert.Equal("apikey_12", agentConfig.APIKey)
	assert.Equal(0.33, agentConfig.ExtraSampleRate)
	assert.Equal(100.0, agentConfig.MaxTPS)
	assert.Equal(25, agentConfig.ReceiverPort)
	// watchdog
	assert.Equal(0.07, agentConfig.MaxCPU)
	assert.Equal(30e6, agentConfig.MaxMemory)
	assert.Equal(50, agentConfig.MaxConnections)

	// Assert Trace Writer
	assert.Equal(11, agentConfig.TraceWriterConfig.MaxSpansPerPayload)
	assert.Equal(22*time.Second, agentConfig.TraceWriterConfig.FlushPeriod)
	assert.Equal(33*time.Second, agentConfig.TraceWriterConfig.UpdateInfoPeriod)
	assert.Equal(15*time.Second, agentConfig.TraceWriterConfig.SenderConfig.MaxAge)
	assert.Equal(int64(2048), agentConfig.TraceWriterConfig.SenderConfig.MaxQueuedBytes)
	assert.Equal(100, agentConfig.TraceWriterConfig.SenderConfig.MaxQueuedPayloads)
	// Assert Service Writer
	assert.Equal(55*time.Second, agentConfig.ServiceWriterConfig.FlushPeriod)
	assert.Equal(44*time.Second, agentConfig.ServiceWriterConfig.UpdateInfoPeriod)
	assert.Equal(15*time.Second, agentConfig.ServiceWriterConfig.SenderConfig.MaxAge)
	assert.Equal(int64(2048), agentConfig.ServiceWriterConfig.SenderConfig.MaxQueuedBytes)
	assert.Equal(100, agentConfig.ServiceWriterConfig.SenderConfig.MaxQueuedPayloads)
	// Assert Stats Writer
	assert.Equal(66*time.Second, agentConfig.StatsWriterConfig.UpdateInfoPeriod)
	assert.Equal(15*time.Second, agentConfig.StatsWriterConfig.SenderConfig.MaxAge)
	assert.Equal(int64(2048), agentConfig.StatsWriterConfig.SenderConfig.MaxQueuedBytes)
	assert.Equal(100, agentConfig.StatsWriterConfig.SenderConfig.MaxQueuedPayloads)
	// analysis legacy
	assert.Equal(1.0, agentConfig.AnalyzedRateByServiceLegacy["db"])
	assert.Equal(0.9, agentConfig.AnalyzedRateByServiceLegacy["web"])
	assert.Equal(0.5, agentConfig.AnalyzedRateByServiceLegacy["index"])
	// analysis
	assert.Len(agentConfig.AnalyzedSpansByService, 2)
	assert.Len(agentConfig.AnalyzedSpansByService["web"], 2)
	assert.Len(agentConfig.AnalyzedSpansByService["db"], 1)
	assert.Equal(agentConfig.AnalyzedSpansByService["web"]["request"], 0.8)
	assert.Equal(agentConfig.AnalyzedSpansByService["web"]["django.request"], 0.9)
	assert.Equal(agentConfig.AnalyzedSpansByService["db"]["intake"], 0.05)
}

func TestConfigNewIfExists(t *testing.T) {
	// The file does not exist: no error returned
	conf, err := NewIniIfExists("/does-not-exist")
	assert.Nil(t, err)
	assert.Nil(t, conf)

	// The file exists but cannot be read for another reason: an error is
	// returned.
	filename := "/tmp/trace-agent-test-config.ini"
	os.Remove(filename)
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0200) // write only
	assert.Nil(t, err)
	f.Close()
	conf, err = NewIniIfExists(filename)
	assert.NotNil(t, err)
	assert.Nil(t, conf)
	os.Remove(filename)
}

func TestGetHostname(t *testing.T) {
	h, err := getHostname("")
	assert.Nil(t, err)

	host, _ := os.Hostname()
	assert.Equal(t, host, h)
}

func TestUndocumentedIni(t *testing.T) {
	assert := assert.New(t)

	iniConf, err := NewIni("./test_cases/undocumented.ini")
	assert.NoError(err, "failed to parse valid configuration")

	c, err := NewAgentConfig(iniConf, nil, nil)
	assert.NoError(err)

	// analysis legacy
	assert.Equal(c.AnalyzedRateByServiceLegacy["web"], 0.8)
	assert.Equal(c.AnalyzedRateByServiceLegacy["intake"], 0.05)
	// analysis
	assert.Len(c.AnalyzedSpansByService, 2)
	assert.Len(c.AnalyzedSpansByService["web"], 2)
	assert.Len(c.AnalyzedSpansByService["db"], 1)
	assert.Equal(c.AnalyzedSpansByService["web"]["request"], 0.8)
	assert.Equal(c.AnalyzedSpansByService["web"]["django.request"], 0.9)
	assert.Equal(c.AnalyzedSpansByService["db"]["intake"], 0.05)
}
