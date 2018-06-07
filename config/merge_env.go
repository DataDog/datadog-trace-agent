package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	log "github.com/cihub/seelog"
)

// mergeEnv applies overrides from environment variables to the trace agent configuration
func mergeEnv(c *AgentConfig) {
	if v := os.Getenv("DD_APM_ENABLED"); v == "true" {
		c.Enabled = true
	} else if v == "false" {
		c.Enabled = false
	}

	if v := os.Getenv("DD_APM_NON_LOCAL_TRAFFIC"); v == "true" {
		c.ReceiverHost = "0.0.0.0"
	} else if v == "false" {
		c.ReceiverHost = "localhost"
	}

	if v := os.Getenv("DD_HOSTNAME"); v != "" {
		c.HostName = v
	}

	if v := os.Getenv("DD_APM_DD_URL"); v != "" {
		c.APIEndpoint = v
	}

	if v := os.Getenv("DD_API_KEY"); v != "" {
		vals := strings.Split(v, ",")
		for i := range vals {
			vals[i] = strings.TrimSpace(vals[i])
		}
		c.APIKey = vals[0]
	}

	if v := os.Getenv("DD_RECEIVER_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			log.Error("Failed to parse DD_RECEIVER_PORT: it should be a port number")
		} else {
			c.ReceiverPort = port
		}
	}

	if v := os.Getenv("DD_BIND_HOST"); v != "" {
		c.StatsdHost = v
		c.ReceiverHost = v
	}

	if v := os.Getenv("DD_IGNORE_RESOURCE"); v != "" {
		c.Ignore["resource"], _ = splitString(v, ',')
	}

	if v := os.Getenv("DD_DOGSTATSD_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			log.Error("Failed to parse DD_DOGSTATSD_PORT: it should be a port number")
		} else {
			c.StatsdPort = port
		}
	}

	if v := os.Getenv("DD_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}

	if v := os.Getenv("DD_APM_ANALYZED_SPANS"); v != "" {
		analyzedSpans, err := parseAnalyzedSpans(v)
		if err == nil {
			c.AnalyzedSpansByService = analyzedSpans
		} else {
			log.Errorf("Bad format for DD_APM_ANALYZED_SPANS it should be of the form \"service_name|operation_name=rate,other_service|other_operation=rate\", error: %v", err)
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
