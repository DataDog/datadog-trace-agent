package filters

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/testutil"

	"github.com/stretchr/testify/assert"
)

func TestBlacklister(t *testing.T) {
	tests := []struct {
		filter      []string
		resource    string
		expectation bool
	}{
		{[]string{"/foo/bar"}, "/foo/bar", false},
		{[]string{"/foo/b.r"}, "/foo/bar", false},
		{[]string{"[0-9]+"}, "/abcde", true},
		{[]string{"[0-9]+"}, "/abcde123", false},
		{[]string{"\\(foobar\\)"}, "(foobar)", false},
		{[]string{"\\(foobar\\)"}, "(bar)", true},
		{[]string{"(GET|POST) /healthcheck"}, "GET /foobar", true},
		{[]string{"(GET|POST) /healthcheck"}, "GET /healthcheck", false},
		{[]string{"(GET|POST) /healthcheck"}, "POST /healthcheck", false},
		{[]string{"SELECT COUNT\\(\\*\\) FROM BAR"}, "SELECT COUNT(*) FROM BAR", false},
		{[]string{"[123"}, "[123", true},
		{[]string{"\\[123"}, "[123", false},
		{[]string{"ABC+", "W+"}, "ABCCCC", false},
		{[]string{"ABC+", "W+"}, "WWW", false},
	}

	for _, test := range tests {
		span := testutil.RandomSpan()
		span.Resource = test.resource
		filter := NewBlacklister(test.filter)

		assert.Equal(t, test.expectation, filter.Allows(span))
	}
}

func TestCompileRules(t *testing.T) {
	filter := NewBlacklister([]string{"[123", "]123", "{6}"})
	for i := 0; i < 100; i++ {
		span := testutil.RandomSpan()
		assert.True(t, filter.Allows(span))
	}
}
