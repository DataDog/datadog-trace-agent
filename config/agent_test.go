package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigHostname(t *testing.T) {
	t.Run("nothing", func(t *testing.T) {
		assert := assert.New(t)
		fallbackHostnameFunc = func() (string, error) {
			return "", nil
		}
		defer func() {
			fallbackHostnameFunc = os.Hostname
		}()
		_, err := Load("./testdata/multi_api_keys.ini")
		assert.Equal(ErrMissingHostname, err)
	})

	t.Run("fallback", func(t *testing.T) {
		host, err := os.Hostname()
		if err != nil || host == "" {
			// can't say
			t.Skip()
		}
		assert := assert.New(t)
		cfg, err := Load("./testdata/multi_api_keys.ini")
		assert.NoError(err)
		assert.Equal(host, cfg.Hostname)
	})

	t.Run("file", func(t *testing.T) {
		assert := assert.New(t)
		cfg, err := Load("./testdata/full.yaml")
		assert.NoError(err)
		assert.Equal("mymachine", cfg.Hostname)
	})

	t.Run("env", func(t *testing.T) {
		// hostname from env
		assert := assert.New(t)
		err := os.Setenv(envHostname, "onlyenv")
		defer os.Unsetenv(envHostname)
		assert.NoError(err)
		cfg, err := Load("./testdata/multi_api_keys.ini")
		assert.NoError(err)
		assert.Equal("onlyenv", cfg.Hostname)
	})

	t.Run("file+env", func(t *testing.T) {
		// hostname from file, overwritten from env
		assert := assert.New(t)
		err := os.Setenv(envHostname, "envoverride")
		defer os.Unsetenv(envHostname)
		assert.NoError(err)
		cfg, err := Load("./testdata/full.yaml")
		assert.NoError(err)
		assert.Equal("envoverride", cfg.Hostname)
	})
}

func TestSite(t *testing.T) {
	for name, tt := range map[string]struct {
		file string
		url  string
	}{
		"default":  {"./testdata/site_default.yaml", "https://trace.agent.datadoghq.com"},
		"eu":       {"./testdata/site_eu.yaml", "https://trace.agent.datadoghq.eu"},
		"url":      {"./testdata/site_url.yaml", "some.other.datadoghq.eu"},
		"override": {"./testdata/site_override.yaml", "some.other.datadoghq.eu"},
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := Load(tt.file)
			assert.NoError(t, err)
			assert.Equal(t, tt.url, cfg.APIEndpoint)
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	assert := assert.New(t)
	c := New()

	// assert that some sane defaults are set
	assert.Equal("localhost", c.ReceiverHost)
	assert.Equal(8126, c.ReceiverPort)

	assert.Equal("localhost", c.StatsdHost)
	assert.Equal(8125, c.StatsdPort)

	assert.Equal("INFO", c.LogLevel)
	assert.Equal(true, c.Enabled)

}

func TestOnlyEnvConfig(t *testing.T) {
	// setting an API Key should be enough to generate valid config
	os.Setenv("DD_API_KEY", "apikey_from_env")

	c := New()
	c.loadEnv()
	assert.Equal(t, "apikey_from_env", c.APIKey)

	os.Setenv("DD_API_KEY", "")
}

func TestOnlyDDAgentConfig(t *testing.T) {
	assert := assert.New(t)

	c, err := loadFile("./testdata/no_apm_config.ini")
	assert.NoError(err)

	assert.Equal("thing", c.Hostname)
	assert.Equal("apikey_12", c.APIKey)
	assert.Equal("0.0.0.0", c.ReceiverHost)
	assert.Equal(28125, c.StatsdPort)
	assert.Equal("DEBUG", c.LogLevel)
}

func TestDDAgentMultiAPIKeys(t *testing.T) {
	// old feature Datadog Agent feature, got dropped since
	// TODO: at some point, expire this case
	assert := assert.New(t)

	c, err := loadFile("./testdata/multi_api_keys.ini")
	assert.NoError(err)

	assert.Equal("foo", c.APIKey)
}

func TestFullIniConfig(t *testing.T) {
	assert := assert.New(t)

	c, err := loadFile("./testdata/full.ini")
	assert.NoError(err)

	assert.Equal("api_key_test", c.APIKey)
	assert.Equal("mymachine", c.Hostname)
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

	c, err := loadFile("./testdata/full.yaml")
	assert.NoError(err)

	assert.Equal("api_key_test", c.APIKey)
	assert.Equal("mymachine", c.Hostname)
	assert.Equal("https://user:password@proxy_for_https:1234", c.ProxyURL.String())
	assert.Equal("https://datadog.unittests", c.APIEndpoint)
	assert.Equal(false, c.Enabled)
	assert.Equal("test", c.DefaultEnv)
	assert.Equal(18126, c.ReceiverPort)
	assert.Equal(0.5, c.ExtraSampleRate)
	assert.Equal(5.0, c.MaxTPS)
	assert.Equal("0.0.0.0", c.ReceiverHost)
	assert.EqualValues([]*Endpoint{
		{Host: "https://my1.endpoint.com", APIKey: "apikey1"},
		{Host: "https://my1.endpoint.com", APIKey: "apikey2"},
		{Host: "https://my2.endpoint.eu", APIKey: "apikey3"},
	}, c.AdditionalEndpoints)

	assert.EqualValues([]string{"/health", "/500"}, c.Ignore["resource"])

	o := c.Obfuscation
	assert.NotNil(o)
	assert.True(o.ES.Enabled)
	assert.EqualValues([]string{"user_id", "category_id"}, o.ES.KeepValues)
	assert.True(o.Mongo.Enabled)
	assert.EqualValues([]string{"uid", "cat_id"}, o.Mongo.KeepValues)
	assert.True(o.HTTP.RemoveQueryString)
	assert.True(o.HTTP.RemovePathDigits)
	assert.True(o.RemoveStackTraces)
	assert.True(c.Obfuscation.Redis.Enabled)
	assert.True(c.Obfuscation.Memcached.Enabled)
}

func TestUndocumentedYamlConfig(t *testing.T) {
	assert := assert.New(t)

	c, err := loadFile("./testdata/undocumented.yaml")
	assert.NoError(err)

	assert.Equal("thing", c.Hostname)
	assert.Equal("apikey_12", c.APIKey)
	assert.Equal(0.33, c.ExtraSampleRate)
	assert.Equal(100.0, c.MaxTPS)
	assert.Equal(25, c.ReceiverPort)
	// watchdog
	assert.Equal(0.07, c.MaxCPU)
	assert.Equal(30e6, c.MaxMemory)
	assert.Equal(50, c.MaxConnections)

	// Assert Trace Writer
	assert.Equal(11, c.TraceWriterConfig.MaxSpansPerPayload)
	assert.Equal(22*time.Second, c.TraceWriterConfig.FlushPeriod)
	assert.Equal(33*time.Second, c.TraceWriterConfig.UpdateInfoPeriod)
	assert.Equal(15*time.Second, c.TraceWriterConfig.SenderConfig.MaxAge)
	assert.Equal(int64(2048), c.TraceWriterConfig.SenderConfig.MaxQueuedBytes)
	assert.Equal(100, c.TraceWriterConfig.SenderConfig.MaxQueuedPayloads)
	// Assert Service Writer
	assert.Equal(55*time.Second, c.ServiceWriterConfig.FlushPeriod)
	assert.Equal(44*time.Second, c.ServiceWriterConfig.UpdateInfoPeriod)
	assert.Equal(15*time.Second, c.ServiceWriterConfig.SenderConfig.MaxAge)
	assert.Equal(int64(2048), c.ServiceWriterConfig.SenderConfig.MaxQueuedBytes)
	assert.Equal(100, c.ServiceWriterConfig.SenderConfig.MaxQueuedPayloads)
	// Assert Stats Writer
	assert.Equal(66*time.Second, c.StatsWriterConfig.UpdateInfoPeriod)
	assert.Equal(15*time.Second, c.StatsWriterConfig.SenderConfig.MaxAge)
	assert.Equal(int64(2048), c.StatsWriterConfig.SenderConfig.MaxQueuedBytes)
	assert.Equal(100, c.StatsWriterConfig.SenderConfig.MaxQueuedPayloads)
	// analysis legacy
	assert.Equal(1.0, c.AnalyzedRateByServiceLegacy["db"])
	assert.Equal(0.9, c.AnalyzedRateByServiceLegacy["web"])
	assert.Equal(0.5, c.AnalyzedRateByServiceLegacy["index"])
	// analysis
	assert.Len(c.AnalyzedSpansByService, 2)
	assert.Len(c.AnalyzedSpansByService["web"], 2)
	assert.Len(c.AnalyzedSpansByService["db"], 1)
	assert.Equal(0.8, c.AnalyzedSpansByService["web"]["request"])
	assert.Equal(0.9, c.AnalyzedSpansByService["web"]["django.request"])
	assert.Equal(0.05, c.AnalyzedSpansByService["db"]["intake"])
}

func TestConfigNewIfExists(t *testing.T) {
	// The file does not exist: no error returned
	conf, err := NewIni("/does-not-exist")
	assert.True(t, os.IsNotExist(err))
	assert.Nil(t, conf)

	// The file exists but cannot be read for another reason: an error is
	// returned.
	filename := "/tmp/trace-agent-test-config.ini"
	os.Remove(filename)
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0200) // write only
	assert.Nil(t, err)
	f.Close()
	conf, err = NewIni(filename)
	assert.NotNil(t, err)
	assert.Nil(t, conf)
	os.Remove(filename)
}

func TestAcquireHostname(t *testing.T) {
	c := New()
	err := c.acquireHostname()
	assert.Nil(t, err)
	host, _ := os.Hostname()
	assert.Equal(t, host, c.Hostname)
}

func TestUndocumentedIni(t *testing.T) {
	assert := assert.New(t)

	c, err := loadFile("./testdata/undocumented.ini")
	assert.NoError(err)

	// analysis legacy
	assert.Equal(0.8, c.AnalyzedRateByServiceLegacy["web"])
	assert.Equal(0.05, c.AnalyzedRateByServiceLegacy["intake"])
	// analysis
	assert.Len(c.AnalyzedSpansByService, 2)
	assert.Len(c.AnalyzedSpansByService["web"], 2)
	assert.Len(c.AnalyzedSpansByService["db"], 1)
	assert.Equal(0.8, c.AnalyzedSpansByService["web"]["request"])
	assert.Equal(0.9, c.AnalyzedSpansByService["web"]["django.request"])
	assert.Equal(0.05, c.AnalyzedSpansByService["db"]["intake"])
}

func TestAnalyzedSpansEnvConfig(t *testing.T) {
	assert := assert.New(t)
	os.Setenv("DD_APM_ANALYZED_SPANS", "service1|operation1=0.5,service2|operation2=1,service1|operation3=0")
	defer os.Unsetenv("DD_APM_ANALYZED_SPANS")

	c := New()
	c.loadEnv()

	assert.Len(c.AnalyzedSpansByService, 2)
	assert.Len(c.AnalyzedSpansByService["service1"], 2)
	assert.Len(c.AnalyzedSpansByService["service2"], 1)
	assert.Equal(0.5, c.AnalyzedSpansByService["service1"]["operation1"], 0.5)
	assert.Equal(float64(0), c.AnalyzedSpansByService["service1"]["operation3"])
	assert.Equal(float64(1), c.AnalyzedSpansByService["service2"]["operation2"])

}
