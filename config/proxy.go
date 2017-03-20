package config

import (
	"fmt"
	"github.com/go-ini/ini"
	"net/url"
	"strings"
)

// mirror default behavior of the infra agent
const defaultProxyPort = 3128

// ProxySettings contains configuration for http/https proxying
type ProxySettings struct {
	User     string
	Password string
	Host     string
	Port     int
	Scheme   string
}

func getProxySettings(m *ini.Section) *ProxySettings {
	p := ProxySettings{Port: defaultProxyPort, Scheme: "http"}

	if v := m.Key("proxy_host").MustString(""); v != "" {
		// accept either http://myproxy.com or myproxy.com
		if i := strings.Index(v, "://"); i != -1 {
			// when available, parse the scheme from the url
			p.Scheme = v[0:i]
			p.Host = v[i+3:]
		} else {
			p.Host = v
		}
	}
	if v := m.Key("proxy_port").MustInt(-1); v != -1 {
		p.Port = v
	}
	if v := m.Key("proxy_user").MustString(""); v != "" {
		p.User = v
	}
	if v := m.Key("proxy_password").MustString(""); v != "" {
		p.Password = v
	}

	return &p
}

// URL turns ProxySettings into an idiomatic URL struct
func (p *ProxySettings) URL() (*url.URL, error) {
	// construct scheme://user:pass@host:port
	var userpass string
	if p.User != "" {
		if p.Password != "" {
			userpass = fmt.Sprintf("%s:%s@", p.User, p.Password)
		}
	}

	return url.Parse(fmt.Sprintf("%s://%s%s:%v", p.Scheme, userpass, p.Host, p.Port))
}
