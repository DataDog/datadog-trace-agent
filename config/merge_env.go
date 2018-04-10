package config

import (
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
}
