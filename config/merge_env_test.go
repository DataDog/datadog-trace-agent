package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzedSpansEnvConfigParsing(t *testing.T) {
	assert := assert.New(t)

	t.Run("valid", func(t *testing.T) {
		a, err := parseAnalyzedSpans("service|operation=1")
		assert.Nil(err)
		assert.Len(a, 1)
		assert.Len(a["service"], 1)
		assert.Equal(float64(1), a["service"]["operation"])

		a, err = parseAnalyzedSpans("service|operation=0.01")
		assert.Nil(err)
		assert.Len(a, 1)
		assert.Len(a["service"], 1)
		assert.Equal(0.01, a["service"]["operation"])

		a, err = parseAnalyzedSpans("service|operation=1,service2|operation2=1")
		assert.Nil(err)
		assert.Len(a, 2)
		assert.Len(a["service"], 1)
		assert.Equal(float64(1), a["service"]["operation"])
		assert.Equal(float64(1), a["service2"]["operation2"])

		a, err = parseAnalyzedSpans("")
		assert.Nil(err)
		assert.Len(a, 0)
	})

	t.Run("errors", func(t *testing.T) {
		_, err := parseAnalyzedSpans("service|operation=")
		assert.NotNil(err)

		_, err = parseAnalyzedSpans("serviceoperation=1")
		assert.NotNil(err)

		_, err = parseAnalyzedSpans("service|operation=1,")
		assert.NotNil(err)
	})
}

func TestLoadEnvMaxTPS(t *testing.T) {
	assert := assert.New(t)

	t.Run("default", func(t *testing.T) {
		ac := New()
		ac.loadEnv()
		assert.EqualValues(10.0, ac.MaxTPS)
	})

	t.Run("env", func(t *testing.T) {
		if err := os.Setenv("DD_MAX_TPS", "123.4"); err != nil {
			t.Fatal(err)
		}
		defer os.Unsetenv("DD_MAX_TPS")

		ac := New()
		ac.loadEnv()
		assert.EqualValues(123.4, ac.MaxTPS)
	})
}
