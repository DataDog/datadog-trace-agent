package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	log "github.com/cihub/seelog"
)

const (
	envAPIKey          = "DD_API_KEY"               // API KEY
	envAPMEnabled      = "DD_APM_ENABLED"           // APM enabled
	envURL             = "DD_APM_DD_URL"            // APM URL
	envHostname        = "DD_HOSTNAME"              // agent hostname
	envBindHost        = "DD_BIND_HOST"             // statsd & receiver hostname
	envReceiverPort    = "DD_RECEIVER_PORT"         // receiver port
	envDogstatsdPort   = "DD_DOGSTATSD_PORT"        // dogstatsd port
	envRemoteTraffic   = "DD_APM_NON_LOCAL_TRAFFIC" // alow non-local traffic
	envIgnoreResources = "DD_IGNORE_RESOURCE"       // ignored resources
	envLogLevel        = "DD_LOG_LEVEL"             // logging level
	envAnalyzedSpans   = "DD_APM_ANALYZED_SPANS"    // spans to analyze for transactions
)

// loadEnv applies overrides from environment variables to the trace agent configuration
func (c *AgentConfig) loadEnv() {
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

	if v := os.Getenv(envURL); v != "" {
		c.APIEndpoint = v
	}

	if v := os.Getenv(envAPIKey); v != "" {
		vals := strings.Split(v, ",")
		for i := range vals {
			vals[i] = strings.TrimSpace(vals[i])
		}
		c.APIKey = vals[0]
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
