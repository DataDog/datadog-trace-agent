package writer

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/info"
	log "github.com/cihub/seelog"
)

const (
	userAgentPrefix     = "Datadog Trace Agent"
	userAgentSupportURL = "https://github.com/DataDog/datadog-trace-agent"
)

// userAgent is the computed user agent we'll use when
// communicating with Datadog
var userAgent = fmt.Sprintf(
	"%s/%s/%s (+%s)",
	userAgentPrefix, info.Version, info.GitCommit, userAgentSupportURL,
)

// DatadogEndpoint sends payloads to Datadog API.
type DatadogEndpoint struct {
	APIKey  string
	Host    string
	NoProxy bool

	client *http.Client
	path   string
}

// NewEndpoints returns the set of endpoints configured in the AgentConfig, appending the given path.
// The first endpoint is the main API endpoint, followed by any additional endpoints.
func NewEndpoints(conf *config.AgentConfig, path string) []Endpoint {
	if !conf.APIEnabled {
		log.Info("API interface is disabled, flushing to /dev/null instead")
		return []Endpoint{&NullEndpoint{}}
	}
	apiKey := conf.APIKey
	url := conf.APIEndpoint
	if apiKey == "" {
		panic("No API key")
	}
	client := newClient(conf, conf.NoProxy)
	endpoints := []Endpoint{&DatadogEndpoint{
		APIKey: apiKey,
		Host:   url,
		path:   path,
		client: client,
	}}
	for _, e := range conf.AdditionalEndpoints {
		c := client
		if e.NoProxy != conf.NoProxy {
			// this client differs, set up a new one
			c = newClient(conf, e.NoProxy)
		}
		apiKey := e.APIKey
		if apiKey == "" {
			// if this endpoint doesn't have its own API key, try the main one.
			apiKey = conf.APIKey
		}
		endpoints = append(endpoints, &DatadogEndpoint{
			APIKey: e.APIKey,
			Host:   e.Host,
			path:   path,
			client: c,
		})
	}
	return endpoints
}

// Write will send the serialized traces payload to the Datadog traces endpoint.
func (e *DatadogEndpoint) Write(payload *Payload) error {
	// Create the request to be sent to the API
	url := e.Host + e.path
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload.Bytes))
	if err != nil {
		return err
	}

	req.Header.Set("DD-Api-Key", e.APIKey)
	req.Header.Set("User-Agent", userAgent)
	SetExtraHeaders(req.Header, payload.Headers)

	resp, err := e.client.Do(req)

	if err != nil {
		return &RetriableError{
			err:      err,
			endpoint: e,
		}
	}
	defer resp.Body.Close()

	// We check the status code to see if the request has succeeded.
	// TODO: define all legit status code and behave accordingly.
	if resp.StatusCode/100 != 2 {
		err := fmt.Errorf("request to %s responded with %s", url, resp.Status)
		if resp.StatusCode/100 == 5 {
			// 5xx errors are retriable
			return &RetriableError{
				err:      err,
				endpoint: e,
			}
		}

		// All others aren't
		return err
	}

	// Everything went fine
	return nil
}

func (e *DatadogEndpoint) String() string {
	return fmt.Sprintf("DataDogEndpoint(%q)", e.Host+e.path)
}

// timeout is the HTTP timeout for POST requests to the Datadog backend
const timeout = 10 * time.Second

// newClient returns a http.Client configured with the Agent options.
func newClient(conf *config.AgentConfig, noProxy bool) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: conf.SkipSSLValidation},
	}
	if conf.ProxyURL != nil && !noProxy {
		log.Infof("configuring proxy through: %s", conf.ProxyURL.String())
		transport.Proxy = http.ProxyURL(conf.ProxyURL)
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}
