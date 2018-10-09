package config

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseReplaceRules tests the compileReplaceRules helper function.
func TestParseRepaceRules(t *testing.T) {
	assert := assert.New(t)
	rules := []*ReplaceRule{
		{Name: "http.url", Pattern: "(token/)([^/]*)", Repl: "${1}?"},
		{Name: "http.url", Pattern: "guid", Repl: "[REDACTED]"},
		{Name: "custom.tag", Pattern: "(/foo/bar/).*", Repl: "${1}extra"},
	}
	err := compileReplaceRules(rules)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rules {
		assert.Equal(r.Pattern, r.Re.String())
	}
}

func TestProxyURL(t *testing.T) {
	for name, tt := range map[string]struct {
		cfg  proxy
		want *url.URL
	}{
		"http": {
			cfg: proxy{HTTP: "http://proxy.addr"},
			want: &url.URL{
				Scheme: "http",
				Host:   "proxy.addr",
			},
		},
		"https": {
			cfg: proxy{HTTPS: "https://proxy.addr"},
			want: &url.URL{
				Scheme: "https",
				Host:   "proxy.addr",
			},
		},
		"url/http": {
			cfg: proxy{URL: "http://proxy.addr"},
			want: &url.URL{
				Scheme: "http",
				Host:   "proxy.addr",
			},
		},
		"url/https": {
			cfg: proxy{URL: "https://proxy.addr"},
			want: &url.URL{
				Scheme: "https",
				Host:   "proxy.addr",
			},
		},
		"url/socks5": {
			cfg: proxy{URL: "socks5://proxy.addr"},
			want: &url.URL{
				Scheme: "socks5",
				Host:   "proxy.addr",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := New()
			cfg.loadYamlConfig(&YamlAgentConfig{Proxy: tt.cfg})
			if got := cfg.ProxyURL; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("expected %#v, got %#v", tt.want, got)
			}
		})
	}
}
