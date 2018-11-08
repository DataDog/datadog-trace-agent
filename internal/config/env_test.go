package config

import (
	"os"
	"testing"

	log "github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	log.UseLogger(log.Disabled)
	os.Exit(m.Run())
}

func TestLoadEnv(t *testing.T) {
	for _, ext := range []string{"yaml", "ini"} {
		t.Run(ext, func(t *testing.T) {
			env := "DD_API_KEY"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "123")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal("123", cfg.Endpoints[0].APIKey)
			})

			env = "DD_SITE"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "my-site.com")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/undocumented." + ext)
				assert.NoError(err)
				assert.Equal(apiEndpointPrefix+"my-site.com", cfg.Endpoints[0].Host)
			})

			env = "DD_APM_ENABLED"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "true")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.True(cfg.Enabled)
			})

			env = "DD_APM_DD_URL"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "my-site.com")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal("my-site.com", cfg.Endpoints[0].Host)
			})

			env = "HTTPS_PROXY"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "my-proxy.url")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal("my-proxy.url", cfg.ProxyURL.String())
			})

			env = "DD_PROXY_HTTPS"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "my-proxy.url")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal("my-proxy.url", cfg.ProxyURL.String())
			})

			env = "DD_HOSTNAME"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "local.host")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal("local.host", cfg.Hostname)
			})

			env = "DD_BIND_HOST"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "bindhost.com")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal("bindhost.com", cfg.StatsdHost)
			})

			env = "DD_RECEIVER_PORT"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "1234")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal(1234, cfg.ReceiverPort)
			})

			env = "DD_DOGSTATSD_PORT"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "4321")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal(4321, cfg.StatsdPort)
			})

			env = "DD_APM_NON_LOCAL_TRAFFIC"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "true")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/undocumented." + ext)
				assert.NoError(err)
				assert.Equal("0.0.0.0", cfg.ReceiverHost)
			})

			env = "DD_IGNORE_RESOURCE"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "1,2,3")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal([]string{"1", "2", "3"}, cfg.Ignore["resource"])
			})

			env = "DD_LOG_LEVEL"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "warn")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal("warn", cfg.LogLevel)
			})

			env = "DD_APM_ANALYZED_SPANS"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "web|http.request=1,db|sql.query=0.5")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal(map[string]map[string]float64{
					"web": map[string]float64{"http.request": 1},
					"db":  map[string]float64{"sql.query": 0.5},
				}, cfg.AnalyzedSpansByService)
			})

			env = "DD_CONNECTION_LIMIT"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "50")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal(50, cfg.ConnectionLimit)
			})

			env = "DD_MAX_TPS"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "6")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal(6., cfg.MaxTPS)
			})

			env = "DD_MAX_EPS"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, "7")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal(7., cfg.MaxEPS)
			})

			env = "DD_APM_COLLECTOR_ADDRESS"
			t.Run(env, func(t *testing.T) {
				assert := assert.New(t)
				err := os.Setenv(env, ":5555")
				assert.NoError(err)
				defer os.Unsetenv(env)
				cfg, err := Load("./testdata/full." + ext)
				assert.NoError(err)
				assert.Equal(":5555", cfg.CollectorConfig.CollectorAddr)
			})
		})
	}
}
