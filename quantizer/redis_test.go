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

func RedisSpan(query string) model.Span {
	return model.Span{
		Resource: query,
		Type:     "redis",
	}
}

func TestRedisQuantizer(t *testing.T) {
	assert := assert.New(t)

	queryToExpected := []redisTestCase{
		{"CLIENT LIST",
			"CLIENT LIST"},

		{"get my_key",
			"GET"},

		{"SET le_key le_value",
			"SET"},

		{"CONFIG SET parameter value",
			"CONFIG SET"},

		{"SET toto tata \n \n  EXPIRE toto 15  ",
			"SET EXPIRE"},

		{"MSET toto tata toto tata toto tata \n ",
			"MSET"},

		{"MULTI\nSET k1 v1\nSET k2 v2\nSET k3 v3\nSET k4 v4\nDEL to_del\nEXEC",
			"MULTI SET* DEL EXEC"},
	}

	for _, testCase := range queryToExpected {
		assert.Equal(testCase.expectedResource, Quantize(RedisSpan(testCase.query)).Resource)
	}

}
