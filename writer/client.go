package writer

import (
	"net/http"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	log "github.com/cihub/seelog"
)

// timeout is the HTTP timeout for POST requests to the Datadog backend
const timeout = 10 * time.Second

// NewClient returns a http.Client configured with the Agent options.
func NewClient(conf *config.AgentConfig) (client *http.Client) {
	if conf.ProxyURL != nil {
		log.Infof("configuring proxy through host %s", conf.ProxyURL.Hostname())
		client = &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				Proxy: http.ProxyURL(conf.ProxyURL),
			},
		}
	} else {
		client = &http.Client{
			Timeout: timeout,
		}
	}

	return
}
