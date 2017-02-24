package quantizer

import (
	"testing"

	"github.com/DataDog/datadog-trace-agent/fixtures"
	"github.com/stretchr/testify/assert"
)

func TestHTTPQuantizer(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			"https://datadog.slack.com/messages/kennel/details/",
			"https://datadog.slack.com",
		},
		{
			"https://app.datadoghq.com/dash/1337/my-amazing-dashboard?live=true&page=0&is_auto=false&from_ts=1487856590189&to_ts=1487942990189&tile_size=m",
			"https://app.datadoghq.com",
		},
		{
			"THIS_URL_IS_OBVIOUSLY_BAD",
			"Non-parsable URL",
		},
		{
			"https://léo.cavaille.net/もの",
			"https://léo.cavaille.net",
		},
		{
			"http://google.com:8888/a/path?with_params=true",
			"http://google.com:8888",
		},
		{
			"datadog.com",
			"Non-parsable URL",
		},
	}

	for _, c := range cases {
		s := fixtures.TestSpan()
		s.Type = "http"
		s.Resource = c.input

		s = Quantize(s)

		assert.Equal(t, c.expected, s.Resource)
		assert.Equal(t, c.input, s.Meta["http.url"])
	}
}
