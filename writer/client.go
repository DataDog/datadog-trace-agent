package writer

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	log "github.com/cihub/seelog"
)

// timeout is the HTTP timeout for POST requests to the Datadog backend
const timeout = 10 * time.Second

// NewClient returns a http.Client configured with the Agent options.
func NewClient(conf *config.AgentConfig) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: conf.SkipSSLValidation},
	}
	if conf.ProxyURL != nil && !conf.NoProxy {
		log.Infof("configuring proxy through: %s", conf.ProxyURL.String())
		transport.Proxy = http.ProxyURL(conf.ProxyURL)
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}
