package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseReplaceRules tests the compileReplaceRules helper function.
func TestParseRepaceRules(t *testing.T) {
	assert := assert.New(t)
	rules := []*ReplaceRule{
		{Name: "http.url", Pattern: "(token/)([^/]*)", Repl: "${1}?"},
		{Name: "http.url", Pattern: "guid", Repl: "[REDACTED]"},
		{Name: "custom.tag", Pattern: "(/foo/bar/).*", Repl: "${1}extra"},
	}
	err := compileReplaceRules(rules)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rules {
		assert.Equal(r.Pattern, r.Re.String())
	}
}

func TestLoadYamlAgentConfig(t *testing.T) {
	t.Run("Obfuscation", func(t *testing.T) {
		assert := assert.New(t)
		conf := New()
		yc := &YamlAgentConfig{
			TraceAgent: traceAgent{
				Obfuscation: &ObfuscationConfig{RemoveStackTraces: true},
			},
		}
		conf.loadYamlConfig(yc)
		assert.NotNil(conf.Obfuscation)
		assert.NotNil(conf.ReplaceTags)
		assert.Len(conf.ReplaceTags, 1)
		assert.Equal("error.stack", conf.ReplaceTags[0].Name)
		assert.NotNil(conf.ReplaceTags[0].Re)
		assert.Equal("?", conf.ReplaceTags[0].Repl)
	})
}
