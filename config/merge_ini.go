package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-trace-agent/backoff"
	"github.com/DataDog/datadog-trace-agent/model"
	writerconfig "github.com/DataDog/datadog-trace-agent/writer/config"
	log "github.com/cihub/seelog"
	"github.com/go-ini/ini"
)

func (c *AgentConfig) loadIniConfig(conf *File) {
	if conf == nil {
		return
	}

	// [Main] section
	m, err := conf.GetSection("Main")
	if err == nil {
		if v := strings.ToLower(m.Key("apm_enabled").MustString("")); v != "" {
			if v == "no" || v == "false" {
				c.Enabled = false
			} else if v == "yes" || v == "true" {
				c.Enabled = true
			}
		}

		if v := m.Key("hostname").MustString(""); v != "" {
			c.Hostname = v
		} else {
			log.Error("Failed to parse hostname from dd-agent config")
		}

		if v := m.Key("api_key").Strings(","); len(v) != 0 {
			c.APIKey = v[0]
		} else {
			log.Error("Failed to parse api_key from dd-agent config")
		}

		if v := m.Key("bind_host").MustString(""); v != "" {
			c.StatsdHost = v
			c.ReceiverHost = v
		}

		// non_local_traffic is a shorthand in dd-agent configuration that is
		// equivalent to setting `bind_host: 0.0.0.0`. Respect this flag
		// since it defaults to true in Docker and saves us a command-line param
		if v := strings.ToLower(m.Key("non_local_traffic").MustString("")); v == "yes" || v == "true" {
			c.StatsdHost = "0.0.0.0"
			c.ReceiverHost = "0.0.0.0"
		}

		if v := m.Key("dogstatsd_port").MustInt(-1); v != -1 {
			c.StatsdPort = v
		}
		if v := m.Key("log_level").MustString(""); v != "" {
			c.LogLevel = v
		}

		if pURL, err := readProxyURL(m); err != nil {
			log.Errorf("Failed to configure proxy: %s", err)
		} else if pURL != nil {
			c.ProxyURL = pURL
		}

		switch m.Key("skip_ssl_validation").MustString("") {
		case "yes", "true", "1":
			c.SkipSSLValidation = true
		case "no", "false", "0":
			c.SkipSSLValidation = false
		}
	}

	// [trace.api] section
	// TODO: deprecate this old api_key location
	if v, _ := conf.Get("trace.api", "api_key"); v != "" {
		vals := strings.Split(v, ",")
		for i := range vals {
			vals[i] = strings.TrimSpace(vals[i])
		}
		c.APIKey = vals[0]
	}
	// undocumented
	if v, _ := conf.Get("trace.api", "endpoint"); v != "" {
		vals := strings.Split(v, ",")
		for i := range vals {
			vals[i] = strings.TrimSpace(vals[i])
		}

		// Takes the first endpoint
		c.APIEndpoint = vals[0]
	}

	// [trace.config] section
	if v, _ := conf.Get("trace.config", "env"); v != "" {
		c.DefaultEnv = model.NormalizeTag(v)
	}
	// undocumented
	// TODO: DEPRECATED? do we keep this one?
	if v, _ := conf.Get("trace.config", "log_level"); v != "" {
		c.LogLevel = v
	}
	// undocumented
	if v, _ := conf.Get("trace.config", "log_file"); v != "" {
		c.LogFilePath = v
	}
	// undocumented
	if v := strings.ToLower(conf.GetDefault("trace.config", "log_throttling", "")); v == "no" || v == "false" {
		c.LogThrottlingEnabled = false
	}

	// [trace.ignore] section
	if v, e := conf.GetStrArray("trace.ignore", "resource", ','); e == nil {
		c.Ignore["resource"] = v
	}

	// [trace.analyzed_rate_by_service] section
	// undocumented
	if v, e := conf.GetSection("trace.analyzed_rate_by_service"); e == nil {
		log.Warn("analyzed_rate_by_service is deprecated, please use analyzed_spans instead")
		rates := v.KeysHash()
		for service, rate := range rates {
			rate, err := strconv.ParseFloat(rate, 64)
			if err != nil {
				log.Errorf("failed to parse rate for analyzed service: %v", service)
				continue
			}

			c.AnalyzedRateByServiceLegacy[service] = rate
		}
	}

	// [trace.analyzed_spans] section
	// undocumented
	if v, e := conf.GetSection("trace.analyzed_spans"); e == nil {
		rates := v.KeysHash()
		for key, rate := range rates {
			serviceName, operationName, err := parseServiceAndOp(key)
			if err != nil {
				log.Errorf("Error when parsing names", err)
				continue
			}
			rate, err := strconv.ParseFloat(rate, 64)
			if err != nil {
				log.Errorf("failed to parse rate for analyzed service: %v", key)
				continue
			}

			if _, ok := c.AnalyzedSpansByService[serviceName]; !ok {
				c.AnalyzedSpansByService[serviceName] = make(map[string]float64)
			}
			c.AnalyzedSpansByService[serviceName][operationName] = rate
		}
	}

	// [trace.concentrator] section
	// undocumented
	// TODO: remove, should stay internal?
	if v, e := conf.GetInt("trace.concentrator", "bucket_size_seconds"); e == nil {
		c.BucketInterval = time.Duration(v) * time.Second
	}
	// undocumented
	// TODO: remove, should stay internal?
	if v, e := conf.GetStrArray("trace.concentrator", "extra_aggregators", ','); e == nil {
		c.ExtraAggregators = append(c.ExtraAggregators, v...)
	}

	// [trace.sampler] section
	if v, e := conf.GetFloat("trace.sampler", "extra_sample_rate"); e == nil {
		c.ExtraSampleRate = v
	}
	// undocumented
	// TODO: remove, should stay internal?
	if v, e := conf.GetFloat("trace.sampler", "pre_sample_rate"); e == nil {
		c.PreSampleRate = v
	}
	if v, e := conf.GetFloat("trace.sampler", "max_traces_per_second"); e == nil {
		c.MaxTPS = v
	}

	// [trace.receiver] section
	if v, e := conf.GetInt("trace.receiver", "receiver_port"); e == nil {
		c.ReceiverPort = v
	}
	// undocumented
	if v, e := conf.GetInt("trace.receiver", "connection_limit"); e == nil {
		c.ConnectionLimit = v
	}
	// undocumented
	if v, e := conf.GetInt("trace.receiver", "timeout"); e == nil {
		c.ReceiverTimeout = v
	}

	// [trace.watchdog] section
	// undocumented
	if v, e := conf.GetFloat("trace.watchdog", "max_memory"); e == nil {
		c.MaxMemory = v
	}
	// undocumented
	if v, e := conf.GetFloat("trace.watchdog", "max_cpu_percent"); e == nil {
		c.MaxCPU = v / 100
	}
	// undocumented
	if v, e := conf.GetInt("trace.watchdog", "max_connections"); e == nil {
		c.MaxConnections = v
	}
	// undocumented
	// TODO: remove, should stay internal?
	if v, e := conf.GetInt("trace.watchdog", "check_delay_seconds"); e == nil {
		c.WatchdogInterval = time.Duration(v) * time.Second
	}

	// [trace.writer.*] sections
	// undocumented, should they stay all internal?
	c.ServiceWriterConfig = readServiceWriterConfig(conf, "trace.writer.services")
	c.StatsWriterConfig = readStatsWriterConfig(conf, "trace.writer.stats")
	c.TraceWriterConfig = readTraceWriterConfig(conf, "trace.writer.traces")
}

func readServiceWriterConfig(confFile *File, section string) writerconfig.ServiceWriterConfig {
	c := writerconfig.DefaultServiceWriterConfig()

	if v, e := confFile.GetInt(section, "flush_period_seconds"); e == nil {
		c.FlushPeriod = time.Duration(v) * time.Second
	}

	if v, e := confFile.GetInt(section, "update_info_period_seconds"); e == nil {
		c.UpdateInfoPeriod = time.Duration(v) * time.Second
	}

	c.SenderConfig = readQueueablePayloadSenderConfig(confFile, section)

	return c
}

func readStatsWriterConfig(confFile *File, section string) writerconfig.StatsWriterConfig {
	c := writerconfig.DefaultStatsWriterConfig()

	if v, e := confFile.GetInt(section, "max_entries_per_payload"); e == nil {
		c.MaxEntriesPerPayload = v
	}

	if v, e := confFile.GetInt(section, "update_info_period_seconds"); e == nil {
		c.UpdateInfoPeriod = time.Duration(v) * time.Second
	}

	c.SenderConfig = readQueueablePayloadSenderConfig(confFile, section)

	return c
}

func readTraceWriterConfig(confFile *File, section string) writerconfig.TraceWriterConfig {
	c := writerconfig.DefaultTraceWriterConfig()

	if v, e := confFile.GetInt(section, "max_spans_per_payload"); e == nil {
		c.MaxSpansPerPayload = v
	}

	if v, e := confFile.GetInt(section, "flush_period_seconds"); e == nil {
		c.FlushPeriod = time.Duration(v) * time.Second
	}
	if v, e := confFile.GetInt(section, "update_info_period_seconds"); e == nil {
		c.UpdateInfoPeriod = time.Duration(v) * time.Second
	}

	c.SenderConfig = readQueueablePayloadSenderConfig(confFile, section)

	return c
}

func readQueueablePayloadSenderConfig(conf *File, section string) writerconfig.QueuablePayloadSenderConf {
	c := writerconfig.DefaultQueuablePayloadSenderConf()

	if v, e := conf.GetInt(section, "queue_max_age_seconds"); e == nil {
		c.MaxAge = time.Duration(v) * time.Second
	}

	if v, e := conf.GetInt64(section, "queue_max_bytes"); e == nil {
		c.MaxQueuedBytes = v
	}

	if v, e := conf.GetInt(section, "queue_max_payloads"); e == nil {
		c.MaxQueuedPayloads = v
	}

	c.ExponentialBackoff = readExponentialBackoffConfig(conf, section)

	return c
}

// TODO: maybe this is too many options exposed?
func readExponentialBackoffConfig(conf *File, section string) backoff.ExponentialConfig {
	c := backoff.DefaultExponentialConfig()

	if v, e := conf.GetInt(section, "exp_backoff_max_duration_seconds"); e == nil {
		c.MaxDuration = time.Duration(v) * time.Second
	}

	if v, e := conf.GetInt(section, "exp_backoff_base_milliseconds"); e == nil {
		c.Base = time.Duration(v) * time.Millisecond
	}

	if v, e := conf.GetInt(section, "exp_backoff_growth_base"); e == nil {
		c.GrowthBase = v
	}

	return c
}

// readProxyURL generates a URL from an Agent 5 configuration of proxy
func readProxyURL(m *ini.Section) (*url.URL, error) {
	// Same defaults as the Agent 5
	scheme := "http"
	port := 3128
	host := ""
	user := ""
	password := ""

	// Parse the configuration optiosn
	if v := m.Key("proxy_host").MustString(""); v != "" {
		// accept either http://myproxy.com or myproxy.com
		if i := strings.Index(v, "://"); i != -1 {
			// when available, parse the scheme from the url
			scheme = v[0:i]
			host = v[i+3:]
		} else {
			host = v
		}
	}
	if v := m.Key("proxy_port").MustInt(-1); v != -1 {
		port = v
	}
	if v := m.Key("proxy_user").MustString(""); v != "" {
		user = v
	}
	if v := m.Key("proxy_password").MustString(""); v != "" {
		password = v
	}

	// No proxy configured
	if host == "" {
		return nil, nil
	}

	// generate the URL
	var userpass *url.Userinfo
	if user != "" {
		if password != "" {
			userpass = url.UserPassword(user, password)
		} else {
			userpass = url.User(user)
		}
	}
	var path string
	if userpass != nil {
		path = fmt.Sprintf("%s://%s@%s:%v", scheme, userpass.String(), host, port)
	} else {
		path = fmt.Sprintf("%s://%s:%v", scheme, host, port)
	}

	return url.Parse(path)
}

func parseServiceAndOp(name string) (string, string, error) {
	splits := strings.Split(name, "|")
	if len(splits) != 2 {
		return "", "", fmt.Errorf("Bad format for operation name and service name in: %s, it should have format: service_name|operation_name", name)
	}
	return splits[0], splits[1], nil
}
