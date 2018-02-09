package config

import (
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-trace-agent/backoff"
	"github.com/DataDog/datadog-trace-agent/model"
	writerconfig "github.com/DataDog/datadog-trace-agent/writer/config"
	log "github.com/cihub/seelog"
	"github.com/go-ini/ini"
)

func mergeIniConfig(c *AgentConfig, conf *File) error {
	if conf == nil {
		return nil
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
			c.HostName = v
		} else {
			log.Info("Failed to parse hostname from dd-agent config")
		}

		if v := m.Key("api_key").Strings(","); len(v) != 0 {
			c.APIKey = v[0]
		} else {
			log.Info("Failed to parse api_key from dd-agent config")
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

		if p := readProxySettings(m); p.Host != "" {
			c.Proxy = p
		}
	}

	// [trace.config] section
	if v, _ := conf.Get("trace.config", "env"); v != "" {
		c.DefaultEnv = model.NormalizeTag(v)
	}
	if v, _ := conf.Get("trace.config", "log_level"); v != "" {
		c.LogLevel = v
	}
	if v, _ := conf.Get("trace.config", "log_file"); v != "" {
		c.LogFilePath = v
	}
	if v := strings.ToLower(conf.GetDefault("trace.config", "log_throttling", "")); v == "no" || v == "false" {
		c.LogThrottlingEnabled = false
	}

	// [trace.ignore] section
	if v, e := conf.GetStrArray("trace.ignore", "resource", ','); e == nil {
		c.Ignore["resource"] = v
	}

	// [trace.analyzed_rate_by_service] section
	if v, e := conf.GetSection("trace.analyzed_rate_by_service"); e == nil {
		rates := v.KeysHash()
		for service, rate := range rates {
			rate, err := strconv.ParseFloat(rate, 64)
			if err != nil {
				log.Infof("failed to parse rate for analyzed service: %v", service)
				continue
			}

			c.AnalyzedRateByService[service] = rate
		}
	}

	// [trace.api] section
	if v, _ := conf.Get("trace.api", "api_key"); v != "" {
		vals := strings.Split(v, ",")
		for i := range vals {
			vals[i] = strings.TrimSpace(vals[i])
		}
		c.APIKey = vals[0]
	}
	if v, _ := conf.Get("trace.api", "endpoint"); v != "" {
		vals := strings.Split(v, ",")
		for i := range vals {
			vals[i] = strings.TrimSpace(vals[i])
		}

		// Takes the first endpoint
		c.APIEndpoint = vals[0]
	}

	// [trace.concentrator] section
	if v, e := conf.GetInt("trace.concentrator", "bucket_size_seconds"); e == nil {
		c.BucketInterval = time.Duration(v) * time.Second
	}
	if v, e := conf.GetStrArray("trace.concentrator", "extra_aggregators", ','); e == nil {
		c.ExtraAggregators = append(c.ExtraAggregators, v...)
	} else {
		log.Debug("No aggregator configuration, using defaults")
	}

	// [trace.sampler] section
	if v, e := conf.GetFloat("trace.sampler", "extra_sample_rate"); e == nil {
		c.ExtraSampleRate = v
	}
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
	if v, e := conf.GetInt("trace.receiver", "connection_limit"); e == nil {
		c.ConnectionLimit = v
	}
	if v, e := conf.GetInt("trace.receiver", "timeout"); e == nil {
		c.ReceiverTimeout = v
	}

	// [trace.watchdog] section
	if v, e := conf.GetFloat("trace.watchdog", "max_memory"); e == nil {
		c.MaxMemory = v
	}
	if v, e := conf.GetFloat("trace.watchdog", "max_cpu_percent"); e == nil {
		c.MaxCPU = v / 100
	}
	if v, e := conf.GetInt("trace.watchdog", "max_connections"); e == nil {
		c.MaxConnections = v
	}
	if v, e := conf.GetInt("trace.watchdog", "check_delay_seconds"); e == nil {
		c.WatchdogInterval = time.Duration(v) * time.Second
	}

	// [trace.writer.*] sections
	c.ServiceWriterConfig = readServiceWriterConfig(conf, "trace.writer.services")
	c.StatsWriterConfig = readStatsWriterConfig(conf, "trace.writer.stats")
	c.TraceWriterConfig = readTraceWriterConfig(conf, "trace.writer.traces")

	return nil
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

func readProxySettings(m *ini.Section) *ProxySettings {
	p := ProxySettings{Port: defaultProxyPort, Scheme: "http"}

	if v := m.Key("proxy_host").MustString(""); v != "" {
		// accept either http://myproxy.com or myproxy.com
		if i := strings.Index(v, "://"); i != -1 {
			// when available, parse the scheme from the url
			p.Scheme = v[0:i]
			p.Host = v[i+3:]
		} else {
			p.Host = v
		}
	}
	if v := m.Key("proxy_port").MustInt(-1); v != -1 {
		p.Port = v
	}
	if v := m.Key("proxy_user").MustString(""); v != "" {
		p.User = v
	}
	if v := m.Key("proxy_password").MustString(""); v != "" {
		p.Password = v
	}

	return &p
}
