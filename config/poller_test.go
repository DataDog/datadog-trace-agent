package config

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func newTestConfigServer() *httptest.Server {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			payload := ServerConfig{
				AnalyzedServices: map[string]float64{
					"web": 1.0,
				},
			}
			bytes, _ := json.Marshal(payload)
			w.WriteHeader(http.StatusOK)
			w.Write(bytes)
		}))

	return server
}

func TestPoller(t *testing.T) {
	assert := assert.New(t)
	server := newTestConfigServer()
	defer server.Close()

	url, err := url.Parse(server.URL)
	done := make(chan struct{})

	assert.NotNil(url)
	assert.Nil(err)

	p := NewDefaultConfigPoller("apikey_2", "")
	p.endpoint = url.String()

	<-done
}
