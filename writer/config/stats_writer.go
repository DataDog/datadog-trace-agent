package config

import "time"

// StatsWriterConfig contains the configuration to customize the behaviour of a TraceWriter.
type StatsWriterConfig struct {
	UpdateInfoPeriod time.Duration
	SenderConfig     QueuablePayloadSenderConf
}

// DefaultStatsWriterConfig creates a new instance of a StatsWriterConfig using default values.
func DefaultStatsWriterConfig() StatsWriterConfig {
	return StatsWriterConfig{
		UpdateInfoPeriod: 1 * time.Minute,
		SenderConfig:     DefaultQueuablePayloadSenderConf(),
	}
}
