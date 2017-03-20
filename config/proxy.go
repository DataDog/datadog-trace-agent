package config

import (
	"github.com/go-ini/ini"
)

// mirror default behavior of the infra agent
const DefaultProxyPort = 3128

type ProxySettings struct {
	User     string
	Password string
	Host     string
	Port     int
}

func getProxySettings(m *ini.Section) *ProxySettings {
	p := ProxySettings{Port: DefaultProxyPort}

	if v := m.Key("proxy_host").MustString(""); v != "" {
		p.Host = v
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
