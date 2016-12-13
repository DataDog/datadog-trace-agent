package main

import (
	"net/http"

	"github.com/DataDog/dd-go/statsd"
)

func HTTPFormatError(tags []string, w http.ResponseWriter) {
	tags = append(tags, "error:format-error")
	statsd.Client.Count("trace_agent.receiver.error", 1, tags, 1)
	http.Error(w, "format-error", 415)
}

func HTTPDecodingError(tags []string, w http.ResponseWriter) {
	tags = append(tags, "error:decoding-error")
	statsd.Client.Count("trace_agent.receiver.error", 1, tags, 1)
	http.Error(w, "decoding-error", 500)
}

func HTTPEndpointNotSupported(tags []string, w http.ResponseWriter) {
	tags = append(tags, "error:unsupported-endpoint")
	statsd.Client.Count("trace_agent.receiver.error", 1, tags, 1)
	http.Error(w, "unsupported-endpoint", 500)
}

func HTTPOK(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}
