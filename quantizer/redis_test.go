package quantizer

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DataDog/datadog-trace-agent/model"
)

type redisTestCase struct {
	query            string
	expectedResource string
}

func RedisSpan(query string) *model.Span {
	return &model.Span{
		Resource: query,
		Type:     "redis",
	}
}

func TestRedisQuantizer(t *testing.T) {
	assert := assert.New(t)

	queryToExpected := []redisTestCase{
		{"CLIENT",
			"CLIENT"}, // regression test for DataDog/datadog-trace-agent#421

		{"CLIENT LIST",
			"CLIENT LIST"},

		{"get my_key",
			"GET"},

		{"SET le_key le_value",
			"SET"},

		{"\n\n  \nSET foo bar  \n  \n\n  ",
			"SET"},

		{"CONFIG SET parameter value",
			"CONFIG SET"},

		{"SET toto tata \n \n  EXPIRE toto 15  ",
			"SET EXPIRE"},

		{"MSET toto tata toto tata toto tata \n ",
			"MSET"},

		{"MULTI\nSET k1 v1\nSET k2 v2\nSET k3 v3\nSET k4 v4\nDEL to_del\nEXEC",
			"MULTI SET SET ..."},

		{"DEL k1\nDEL k2\nHMSET k1 \"a\" 1 \"b\" 2 \"c\" 3\nHMSET k2 \"d\" \"4\" \"e\" \"4\"\nDEL k3\nHMSET k3 \"f\" \"5\"\nDEL k1\nDEL k2\nHMSET k1 \"a\" 1 \"b\" 2 \"c\" 3\nHMSET k2 \"d\" \"4\" \"e\" \"4\"\nDEL k3\nHMSET k3 \"f\" \"5\"\nDEL k1\nDEL k2\nHMSET k1 \"a\" 1 \"b\" 2 \"c\" 3\nHMSET k2 \"d\" \"4\" \"e\" \"4\"\nDEL k3\nHMSET k3 \"f\" \"5\"\nDEL k1\nDEL k2\nHMSET k1 \"a\" 1 \"b\" 2 \"c\" 3\nHMSET k2 \"d\" \"4\" \"e\" \"4\"\nDEL k3\nHMSET k3 \"f\" \"5\"",
			"DEL DEL HMSET ..."},

		{"GET...",
			"..."},

		{"GET k...",
			"GET"},

		{"GET k1\nGET k2\nG...",
			"GET GET ..."},

		{"GET k1\nGET k2\nDEL k3\nGET k...",
			"GET GET DEL ..."},

		{"GET k1\nGET k2\nHDEL k3 a\nG...",
			"GET GET HDEL ..."},

		{"GET k...\nDEL k2\nMS...",
			"GET DEL ..."},

		{"GET k...\nDE...\nMS...",
			"GET ..."},

		{"GET k1\nDE...\nGET k2",
			"GET GET"},

		{"GET k1\nDE...\nGET k2\nHDEL k3 a\nGET k4\nDEL k5",
			"GET GET HDEL ..."},
	}

	for _, testCase := range queryToExpected {
		s := RedisSpan(testCase.query)
		Quantize(s)
		assert.Equal(testCase.expectedResource, s.Resource)
	}

}

func BenchmarkTestRedisQuantizer(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		span := RedisSpan(`DEL k1\nDEL k2\nHMSET k1 "a" 1 "b" 2 "c" 3\nHMSET k2 "d" "4" "e" "4"\nDEL k3\nHMSET k3 "f" "5"\nDEL k1\nDEL k2\nHMSET k1 "a" 1 "b" 2 "c" 3\nHMSET k2 "d" "4" "e" "4"\nDEL k3\nHMSET k3 "f" "5"\nDEL k1\nDEL k2\nHMSET k1 "a" 1 "b" 2 "c" 3\nHMSET k2 "d" "4" "e" "4"\nDEL k3\nHMSET k3 "f" "5"\nDEL k1\nDEL k2\nHMSET k1 "a" 1 "b" 2 "c" 3\nHMSET k2 "d" "4" "e" "4"\nDEL k3\nHMSET k3 "f" "5"`)
		QuantizeRedis(span)
	}
}
