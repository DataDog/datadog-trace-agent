package main

import (
	"net/http"

	"github.com/DataDog/datadog-trace-agent/statsd"
)

// HTTPFormatError is used for payload format errors
func HTTPFormatError(tags []string, w http.ResponseWriter) {
	tags = append(tags, "error:format-error")
	statsd.Client.Count("trace_agent.receiver.error", 1, tags, 1)
	http.Error(w, "format-error", 415)
}

// HTTPDecodingError is used for errors happening in decoding
func HTTPDecodingError(tags []string, w http.ResponseWriter) {
	tags = append(tags, "error:decoding-error")
	statsd.Client.Count("trace_agent.receiver.error", 1, tags, 1)
	http.Error(w, "decoding-error", 500)
}

// HTTPEndpointNotSupported is for payloads getting sent to a wrong endpoint
func HTTPEndpointNotSupported(tags []string, w http.ResponseWriter) {
	tags = append(tags, "error:unsupported-endpoint")
	statsd.Client.Count("trace_agent.receiver.error", 1, tags, 1)
	http.Error(w, "unsupported-endpoint", 500)
}

// HTTPOK is a dumb response for when things are a OK
func HTTPOK(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}
