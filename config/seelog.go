package config

import (
	"encoding/xml"
	"fmt"
	"strings"

	log "github.com/cihub/seelog"
)

type outputs struct {
	FormatID string `xml:"formatid,attr"`
	Console  string `xml:",innerxml"`
}

type format struct {
	ID     string `xml:"id,attr"`
	Format string `xml:"format,attr"`
}

type formats struct {
	Format format `xml:"format"`
}

type seelog struct {
	Outputs  outputs `xml:"outputs,omitempty"`
	Formats  formats `xml:"formats,omitempty"`
	LogLevel string  `xml:"minlevel,attr"`
}

func newSeelogConfig(logFilePath string) seelog {
	// Rotate log files when size reaches 10MB
	outputXML := fmt.Sprintf(
		"<console /> <rollingfile type=\"size\" filename=\"%s\" maxsize=\"10000000\" maxrolls=\"5\" />",
		logFilePath,
	)

	return seelog{
		Outputs: outputs{"common", outputXML},
		Formats: formats{
			format{
				ID:     "common",
				Format: "%Date %Time %LEVEL (%File:%Line) - %Msg%n",
			},
		},
		LogLevel: "info",
	}
}

// NewLoggerLevel sets the global log level.
func NewLoggerLevel(debug bool) error {
	if debug {
		return NewLoggerLevelCustom("debug")
	}
	return NewLoggerLevelCustom("info")
}

// NewLoggerLevelCustom creates a logger with the given level.
func NewLoggerLevelCustom(level, logFilePath string) error {
	cfg := newSeelogConfig(logFilePath)
	ll, ok := log.LogLevelFromString(strings.ToLower(level))
	if !ok {
		ll = log.InfoLvl
	}
	cfg.LogLevel = ll.String()

	l, err := log.LoggerFromConfigAsString(cfg.String())
	if err != nil {
		return err
	}
	log.ReplaceLogger(l)
	return nil
}

func (s seelog) String() string {
	b, err := xml.MarshalIndent(s, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}
