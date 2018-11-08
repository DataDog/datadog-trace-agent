package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	log "github.com/cihub/seelog"
)

const (
	envAPIKey          = "DD_API_KEY"               // API KEY
	envSite            = "DD_SITE"                  // server site (us, eu)
	envAPMEnabled      = "DD_APM_ENABLED"           // APM enabled
	envURL             = "DD_APM_DD_URL"            // APM URL
	envProxyDeprecated = "HTTPS_PROXY"              // (deprecated) proxy URL
	envProxy           = "DD_PROXY_HTTPS"           // proxy URL (overrides deprecated)
	envHostname        = "DD_HOSTNAME"              // agent hostname
	envBindHost        = "DD_BIND_HOST"             // statsd & receiver hostname
	envReceiverPort    = "DD_RECEIVER_PORT"         // receiver port
	envDogstatsdPort   = "DD_DOGSTATSD_PORT"        // dogstatsd port
	envRemoteTraffic   = "DD_APM_NON_LOCAL_TRAFFIC" // alow non-local traffic
	envIgnoreResources = "DD_IGNORE_RESOURCE"       // ignored resources
	envLogLevel        = "DD_LOG_LEVEL"             // logging level
	envAnalyzedSpans   = "DD_APM_ANALYZED_SPANS"    // spans to analyze for transactions
	envConnectionLimit = "DD_CONNECTION_LIMIT"      // (deprecated) limit of unique connections
	envMaxTPS          = "DD_MAX_TPS"               // maximum limit to the total number of traces per second to sample (MaxTPS)
	envMaxEPS          = "DD_MAX_EPS"               // maximum limit to the total number of events per second to sample (MaxEPS)
	envCollectorAddr   = "DD_APM_COLLECTOR_ADDRESS" // The address of the collector to send data to
)

// loadEnv applies overrides from environment variables to the trace agent configuration
func (c *AgentConfig) loadEnv() {
	if v, ok := os.LookupEnv(envConnectionLimit); ok {
		limit, err := strconv.Atoi(v)
		if err != nil {
			log.Errorf("failed to parse DD_CONNECTION_LIMIT: %v", err)
		} else {
			c.ConnectionLimit = limit
		}
	}
	if v := os.Getenv(envAPMEnabled); v == "true" {
		c.Enabled = true
	} else if v == "false" {
		c.Enabled = false
	}

	if v := os.Getenv(envRemoteTraffic); v == "true" {
		c.ReceiverHost = "0.0.0.0"
	} else if v == "false" {
		c.ReceiverHost = "localhost"
	}

	if v := os.Getenv(envHostname); v != "" {
		c.Hostname = v
	}

	if len(c.Endpoints) == 0 {
		c.Endpoints = []*Endpoint{{}}
	}
	site := os.Getenv(envSite)
	if site != "" {
		c.Endpoints[0].Host = apiEndpointPrefix + site
	}
	if v := os.Getenv(envURL); v != "" {
		c.Endpoints[0].Host = v
		if site != "" {
			log.Infof("'DD_SITE' and 'DD_APM_DD_URL' are both set, using endpoint: %q", v)
		}
	}

	if v := os.Getenv(envAPIKey); v != "" {
		v := strings.Split(v, ",")[0]
		c.Endpoints[0].APIKey = strings.TrimSpace(v)
	}

	if v := os.Getenv(envReceiverPort); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			log.Errorf("Failed to parse %s: it should be a number", envReceiverPort)
		} else {
			c.ReceiverPort = port
		}
	}

	if v := os.Getenv(envBindHost); v != "" {
		c.StatsdHost = v
		c.ReceiverHost = v
	}

	if v := os.Getenv(envIgnoreResources); v != "" {
		c.Ignore["resource"], _ = splitString(v, ',')
	}

	if v := os.Getenv(envDogstatsdPort); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			log.Errorf("Failed to parse %s: it should be a port number", envDogstatsdPort)
		} else {
			c.StatsdPort = port
		}
	}

	if v := os.Getenv(envLogLevel); v != "" {
		c.LogLevel = v
	}

	if v := os.Getenv(envAnalyzedSpans); v != "" {
		analyzedSpans, err := parseAnalyzedSpans(v)
		if err == nil {
			c.AnalyzedSpansByService = analyzedSpans
		} else {
			log.Errorf("Bad format for %s it should be of the form \"service_name|operation_name=rate,other_service|other_operation=rate\", error: %v", envAnalyzedSpans, err)
		}
	}

	if v := os.Getenv(envMaxEPS); v != "" {
		maxEPS, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.Errorf("Failed to parse %s: it should be a floating point number", envMaxEPS)
		} else {
			c.MaxEPS = maxEPS
		}
	}

	for _, env := range []string{envProxyDeprecated, envProxy} {
		if v, ok := os.LookupEnv(env); ok {
			url, err := url.Parse(v)
			if err == nil {
				c.ProxyURL = url
			} else {
				log.Errorf("Failed to parse proxy URL from proxy.https configuration: %s", err)
			}
		}
	}

	if v := os.Getenv(envMaxTPS); v != "" {
		maxTPS, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.Errorf("Failed to parse %s: it should be a float number", envMaxTPS)
		} else {
			c.MaxTPS = maxTPS
		}
	}

	if v := os.Getenv(envCollectorAddr); v != "" {
		c.CollectorAddr = v
	}
}

func parseNameAndRate(token string) (string, float64, error) {
	parts := strings.Split(token, "=")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("Bad format")
	}
	rate, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return "", 0, fmt.Errorf("Unabled to parse rate")
	}
	return parts[0], rate, nil
}

// parseAnalyzedSpans parses the env string to extract a map of spans to be analyzed by service and operation.
// the format is: service_name|operation_name=rate,other_service|other_operation=rate
func parseAnalyzedSpans(env string) (analyzedSpans map[string]map[string]float64, err error) {
	analyzedSpans = make(map[string]map[string]float64)
	if env == "" {
		return
	}
	tokens := strings.Split(env, ",")
	for _, token := range tokens {
		name, rate, err := parseNameAndRate(token)
		if err != nil {
			return nil, err
		}
		serviceName, operationName, err := parseServiceAndOp(name)
		if err != nil {
			return nil, err
		}

		if _, ok := analyzedSpans[serviceName]; !ok {
			analyzedSpans[serviceName] = make(map[string]float64)
		}
		analyzedSpans[serviceName][operationName] = rate
	}
	return
}
