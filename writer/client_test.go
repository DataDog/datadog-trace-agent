package writer

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	assert := assert.New(t)
	url, err := url.Parse("test_url")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("blank", func(t *testing.T) {
		client := NewClient(&config.AgentConfig{})
		transport := client.Transport.(*http.Transport)
		assert.False(transport.TLSClientConfig.InsecureSkipVerify)
		assert.Nil(transport.Proxy)
	})

	t.Run("no_proxy", func(t *testing.T) {
		client := NewClient(&config.AgentConfig{
			SkipSSLValidation: true,
			ProxyURL:          url,
			NoProxy:           true,
		})
		transport := client.Transport.(*http.Transport)
		assert.True(transport.TLSClientConfig.InsecureSkipVerify)
		assert.Nil(transport.Proxy)
	})

	t.Run("proxy", func(t *testing.T) {
		client := NewClient(&config.AgentConfig{ProxyURL: url})
		transport := client.Transport.(*http.Transport)
		goturl, _ := transport.Proxy(nil)
		assert.False(transport.TLSClientConfig.InsecureSkipVerify)
		assert.Equal("test_url", goturl.String())
	})
}
