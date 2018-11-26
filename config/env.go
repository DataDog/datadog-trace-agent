package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/DataDog/datadog-agent/pkg/config"
	log "github.com/cihub/seelog"
)

func applyEnv() {
	// Warning: do not use BindEnv to bind config variables. They will be overriden
	// when using the legacy config loader.
	for envKey, cfgKey := range map[string]string{
		"DD_SITE":                  "site",
		"DD_API_KEY":               "api_key",
		"DD_HOSTNAME":              "hostname",
		"DD_BIND_HOST":             "bind_host",
		"DD_DOGSTATSD_PORT":        "dogstatsd_port",
		"DD_LOG_LEVEL":             "log_level",
		"DD_CONNECTION_LIMIT":      "apm_config.connection_limit",
		"DD_APM_ENABLED":           "apm_config.enabled",
		"DD_APM_NON_LOCAL_TRAFFIC": "apm_config.apm_non_local_traffic",
		"DD_APM_DD_URL":            "apm_config.apm_dd_url",
		"DD_RECEIVER_PORT":         "apm_config.receiver_port",
		"DD_MAX_EPS":               "apm_config.max_events_per_second",
		"DD_MAX_TPS":               "apm_config.max_traces_per_second",
		"HTTPS_PROXY":              "proxy.https",
		"DD_PROXY_HTTPS":           "proxy.https",
	} {
		if v, ok := os.LookupEnv(envKey); ok {
			config.Datadog.Set(cfgKey, v)
		}
	}
	if v, ok := os.LookupEnv("DD_IGNORE_RESOURCE"); ok {
		if r, err := splitString(v, ','); err != nil {
			log.Warn("%q value not loaded: %v", "DD_IGNORE_RESOURCE", err)
		} else {
			config.Datadog.Set("apm_config.ignore_resources", r)
		}
	}
	if v, ok := os.LookupEnv("DD_APM_ANALYZED_SPANS"); ok {
		analyzedSpans, err := parseAnalyzedSpans(v)
		if err == nil {
			config.Datadog.Set("apm_config.analyzed_spans", analyzedSpans)
		} else {
			log.Errorf("Bad format for %s it should be of the form \"service_name|operation_name=rate,other_service|other_operation=rate\", error: %v", "DD_APM_ANALYZED_SPANS", err)
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
func parseAnalyzedSpans(env string) (analyzedSpans map[string]float64, err error) {
	analyzedSpans = make(map[string]float64)
	if env == "" {
		return
	}
	tokens := strings.Split(env, ",")
	for _, token := range tokens {
		name, rate, err := parseNameAndRate(token)
		if err != nil {
			return nil, err
		}
		analyzedSpans[name] = rate
	}
	return
}
