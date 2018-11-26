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
	// we have to apply the env. variables to the config because if we
	// use BindEnv it won't work with the legacy package, which will simply
	// overwrite them.
	if v, ok := os.LookupEnv("DD_SITE"); ok {
		config.Datadog.Set("site", v)
	}
	if v, ok := os.LookupEnv("DD_API_KEY"); ok {
		config.Datadog.Set("api_key", v)
	}
	if v, ok := os.LookupEnv("DD_HOSTNAME"); ok {
		config.Datadog.Set("hostname", v)
	}
	if v, ok := os.LookupEnv("DD_BIND_HOST"); ok {
		config.Datadog.Set("bind_host", v)
	}
	if v, ok := os.LookupEnv("DD_DOGSTATSD_PORT"); ok {
		config.Datadog.Set("dogstatsd_port", v)
	}
	if v, ok := os.LookupEnv("DD_LOG_LEVEL"); ok {
		config.Datadog.Set("log_level", v)
	}
	if v, ok := os.LookupEnv("DD_CONNECTION_LIMIT"); ok {
		config.Datadog.Set("apm_config.connection_limit", v)
	}
	if v, ok := os.LookupEnv("DD_APM_ENABLED"); ok {
		config.Datadog.Set("apm_config.enabled", v)
	}
	if v, ok := os.LookupEnv("DD_APM_NON_LOCAL_TRAFFIC"); ok {
		config.Datadog.Set("apm_config.apm_non_local_traffic", v)
	}
	if v, ok := os.LookupEnv("DD_APM_DD_URL"); ok {
		config.Datadog.Set("apm_config.apm_dd_url", v)
	}
	if v, ok := os.LookupEnv("DD_RECEIVER_PORT"); ok {
		config.Datadog.Set("apm_config.receiver_port", v)
	}
	if v, ok := os.LookupEnv("DD_MAX_EPS"); ok {
		config.Datadog.Set("apm_config.max_events_per_second", v)
	}
	if v, ok := os.LookupEnv("DD_MAX_TPS"); ok {
		config.Datadog.Set("apm_config.max_traces_per_second", v)
	}
	if v, ok := os.LookupEnv("HTTPS_PROXY"); ok {
		config.Datadog.Set("proxy.https", v)
	}
	if v, ok := os.LookupEnv("DD_PROXY_HTTPS"); ok {
		config.Datadog.Set("proxy.https", v)
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
