package config

import "time"

// maxEntriesPerPayload is the maximum number of entries in a stat payload. A
// value of 0 disables splitting all together. For now, the feature is disabled
// by default.
const maxEntriesPerPayload = 0

// StatsWriterConfig contains the configuration to customize the behaviour of a TraceWriter.
type StatsWriterConfig struct {
	MaxEntriesPerPayload int
	UpdateInfoPeriod     time.Duration
	SenderConfig         QueuablePayloadSenderConf
}

// DefaultStatsWriterConfig creates a new instance of a StatsWriterConfig using default values.
func DefaultStatsWriterConfig() StatsWriterConfig {
	return StatsWriterConfig{
		MaxEntriesPerPayload: maxEntriesPerPayload,
		UpdateInfoPeriod:     1 * time.Minute,
		SenderConfig:         DefaultQueuablePayloadSenderConf(),
	}
}
