package filters

import (
	"regexp"
	"testing"

	"github.com/StackVista/stackstate-trace-agent/config"
	"github.com/StackVista/stackstate-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func TestTagReplacer(t *testing.T) {
	assert := assert.New(t)
	for _, tt := range []struct {
		rules     [][3]string
		got, want map[string]string
	}{
		{
			rules: [][3]string{
				{"http.url", "(token/)([^/]*)", "${1}?"},
				{"http.url", "guid", "[REDACTED]"},
				{"custom.tag", "(/foo/bar/).*", "${1}extra"},
				{"a", "b", "c"},
			},
			got: map[string]string{
				"http.url":   "some/guid/token/abcdef/abc",
				"custom.tag": "/foo/bar/foo",
			},
			want: map[string]string{
				"http.url":   "some/[REDACTED]/token/?/abc",
				"custom.tag": "/foo/bar/extra",
			},
		},
		{
			rules: [][3]string{
				{"*", "(token/)([^/]*)", "${1}?"},
				{"*", "this", "that"},
				{"http.url", "guid", "[REDACTED]"},
				{"custom.tag", "(/foo/bar/).*", "${1}extra"},
				{"resource.name", "prod", "stage"},
			},
			got: map[string]string{
				"resource.name": "this is prod",
				"http.url":      "some/[REDACTED]/token/abcdef/abc",
				"other.url":     "some/guid/token/abcdef/abc",
				"custom.tag":    "/foo/bar/foo",
			},
			want: map[string]string{
				"resource.name": "that is stage",
				"http.url":      "some/[REDACTED]/token/?/abc",
				"other.url":     "some/guid/token/?/abc",
				"custom.tag":    "/foo/bar/extra",
			},
		},
	} {
		rules := parseRulesFromString(tt.rules)
		tr := &tagReplacer{replace: rules}
		root := replaceFilterTestSpan(tt.got)
		childSpan := replaceFilterTestSpan(tt.got)
		trace := model.Trace{root, childSpan}
		ok := tr.Keep(root, &trace)
		assert.True(ok)
		for k, v := range tt.want {
			switch k {
			case "resource.name":
				// test that the filter applies to all spans, not only the root
				assert.Equal(v, root.Resource)
				assert.Equal(v, childSpan.Resource)
			default:
				assert.Equal(v, root.Meta[k])
				assert.Equal(v, childSpan.Meta[k])
			}
		}
	}
}

func parseRulesFromString(rules [][3]string) []*config.ReplaceRule {
	r := make([]*config.ReplaceRule, 0, len(rules))
	for _, rule := range rules {
		key, re, str := rule[0], rule[1], rule[2]
		r = append(r, &config.ReplaceRule{
			Name:    key,
			Pattern: re,
			Re:      regexp.MustCompile(re),
			Repl:    str,
		})
	}
	return r
}

// replaceFilterTestSpan creates a span from a list of tags and uses
// special tag names (e.g. resource.name) to target attributes.
func replaceFilterTestSpan(tags map[string]string) *model.Span {
	span := &model.Span{Meta: make(map[string]string)}
	for k, v := range tags {
		switch k {
		case "resource.name":
			span.Resource = v
		default:
			span.Meta[k] = v
		}
	}
	return span
}

// TestReplaceFilterTestSpan tests the replaceFilterTestSpan test
// helper function.
func TestReplaceFilterTestSpan(t *testing.T) {
	for _, tt := range []struct {
		tags map[string]string
		want *model.Span
	}{
		{
			tags: map[string]string{
				"resource.name": "a",
				"http.url":      "url",
				"custom.tag":    "val",
			},
			want: &model.Span{
				Resource: "a",
				Meta: map[string]string{
					"http.url":   "url",
					"custom.tag": "val",
				},
			},
		},
	} {
		got := replaceFilterTestSpan(tt.tags)
		assert.Equal(t, tt.want, got)
	}
}
