package filters

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

func TestFilter(t *testing.T) {
	tests := []struct {
		filter      string
		resource    string
		expectation bool
	}{
		{"/foo/bar", "/foo/bar", false},
		{"/foo/b.r", "/foo/bar", false},
		{"[0-9]+", "/abcde", true},
		{"[0-9]+", "/abcde123", false},
		{"\\(foobar\\)", "(foobar)", false},
		{"\\(foobar\\)", "(bar)", true},
		{"(GET|POST) /healthcheck", "GET /foobar", true},
		{"(GET|POST) /healthcheck", "GET /healthcheck", false},
		{"(GET|POST) /healthcheck", "POST /healthcheck", false},
		{"SELECT COUNT\\(\\*\\) FROM BAR", "SELECT COUNT(*) FROM BAR", false},
	}

	for _, test := range tests {
		span := newTestSpan(test.resource)
		trace := model.Trace{span}
		filter := newTestFilter(test.filter)

		assert.Equal(t, test.expectation, filter.Keep(span, &trace))
	}
}

// a filter instantiated with malformed expressions should let anything pass
func TestRegexCompilationFailure(t *testing.T) {
	filter := newTestFilter("[123", "]123", "{6}")

	for i := 0; i < 100; i++ {
		span := fixtures.RandomSpan()
		trace := model.Trace{span}
		assert.True(t, filter.Keep(span, &trace))
	}
}

func TestRegexEscaping(t *testing.T) {
	span := newTestSpan("[123")
	trace := model.Trace{span}

	filter := newTestFilter("[123")
	assert.True(t, filter.Keep(span, &trace))

	filter = newTestFilter("\\[123")
	assert.False(t, filter.Keep(span, &trace))
}

func TestMultipleEntries(t *testing.T) {
	filter := newTestFilter("ABC+", "W+")

	span := newTestSpan("ABCCCC")
	trace := model.Trace{span}
	assert.False(t, filter.Keep(span, &trace))

	span = newTestSpan("WWW")
	trace = model.Trace{span}
	assert.False(t, filter.Keep(span, &trace))
}

func newTestFilter(blacklist ...string) Filter {
	c := config.NewDefaultAgentConfig()
	c.Ignore["resource"] = blacklist

	return newResourceFilter(c)
}

func newTestSpan(resource string) *model.Span {
	span := fixtures.RandomSpan()
	span.Resource = resource
	return span
}
