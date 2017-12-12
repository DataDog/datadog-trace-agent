package writer

import (
	"net/http"

	"github.com/DataDog/datadog-trace-agent/config"
	log "github.com/cihub/seelog"
)

// NewClient returns a http.Client configured with the Agent options.
func NewClient(conf *config.AgentConfig) (client *http.Client) {
	if conf.Proxy != nil {
		proxyPath, err := conf.Proxy.URL()
		if err != nil {
			log.Errorf("failed to configure proxy: %v", err)
			return
		}

		log.Infof("configuring proxy through host %s", conf.Proxy.Host)
		client = &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyPath),
			},
		}
	} else {
		client = &http.Client{
			Timeout: timeout,
		}
	}

	return
}
