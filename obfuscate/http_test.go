package obfuscate

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func TestObfuscateHTTP(t *testing.T) {
	const httpURL = "http://myweb.site/id/123/page/1?q=james&uid=a4f"
	testSpan := &model.Span{
		Type: "http",
		Meta: map[string]string{
			"http.url": httpURL,
		},
	}

	t.Run("disabled", func(t *testing.T) {
		assert := assert.New(t)
		span := *testSpan
		NewObfuscator(&config.ObfuscationConfig{}).Obfuscate(&span)
		assert.Equal(httpURL, span.Meta["http.url"])
	})

	t.Run("enabled-query", func(t *testing.T) {
		assert := assert.New(t)
		span := *testSpan
		NewObfuscator(&config.ObfuscationConfig{
			HTTP: config.HTTPObfuscationConfig{RemoveQueryString: true},
		}).Obfuscate(&span)
		assert.Equal("http://myweb.site/id/123/page/1?", span.Meta["http.url"])
	})

	t.Run("enabled-digits", func(t *testing.T) {
		assert := assert.New(t)
		span := *testSpan
		NewObfuscator(&config.ObfuscationConfig{
			HTTP: config.HTTPObfuscationConfig{RemovePathDigits: true},
		}).Obfuscate(&span)
		assert.Equal("http://myweb.site/id/?/page/??", span.Meta["http.url"])
		assert.Equal("foo bar", span.Meta["error.stack"])
	})
}
