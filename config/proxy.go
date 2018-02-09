package config

import (
	"fmt"
	"net/url"
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

// URL turns ProxySettings into an idiomatic URL struct
func (p *ProxySettings) URL() (*url.URL, error) {
	// construct scheme://user:pass@host:port
	var userpass *url.Userinfo
	if p.User != "" {
		if p.Password != "" {
			userpass = url.UserPassword(p.User, p.Password)
		} else {
			userpass = url.User(p.User)
		}
	}

	var path string
	if userpass != nil {
		path = fmt.Sprintf("%s://%s@%s:%v", p.Scheme, userpass.String(), p.Host, p.Port)
	} else {
		path = fmt.Sprintf("%s://%s:%v", p.Scheme, p.Host, p.Port)
	}

	return url.Parse(path)
}
