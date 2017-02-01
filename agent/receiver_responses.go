package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/DataDog/datadog-trace-agent/statsd"
)

// HTTPFormatError is used for payload format errors
func HTTPFormatError(tags []string, w http.ResponseWriter) {
	tags = append(tags, "error:format-error")
	statsd.Client.Count("datadog.trace_agent.receiver.error", 1, tags, 1)
	http.Error(w, "format-error", http.StatusUnsupportedMediaType)
}

// HTTPDecodingError is used for errors happening in decoding
func HTTPDecodingError(err error, tags []string, w http.ResponseWriter) {
	status := http.StatusBadRequest
	errtag := "decoding-error"
	msg := errtag

	if err == model.ErrLimitedReaderLimitReached {
		status = http.StatusRequestEntityTooLarge
		errtag := "payload-too-large"
		msg = errtag
	}

	tags = append(tags, fmt.Sprintf("error:%s", errtag))
	statsd.Client.Count("datadog.trace_agent.receiver.error", 1, tags, 1)

	http.Error(w, msg, status)
}

// HTTPEndpointNotSupported is for payloads getting sent to a wrong endpoint
func HTTPEndpointNotSupported(tags []string, w http.ResponseWriter) {
	tags = append(tags, "error:unsupported-endpoint")
	statsd.Client.Count("datadog.trace_agent.receiver.error", 1, tags, 1)
	http.Error(w, "unsupported-endpoint", http.StatusInternalServerError)
}

// HTTPOK is a dumb response for when things are a OK
func HTTPOK(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "OK\n")
}
