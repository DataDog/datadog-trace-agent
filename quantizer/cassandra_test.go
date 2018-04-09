package quantizer

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DataDog/datadog-trace-agent/model"
)

func CassSpan(query string) *model.Span {
	return &model.Span{
		Resource: query,
		Type:     "cassandra",
		Meta: map[string]string{
			"query": query,
		},
	}
}

func TestCassQuantizer(t *testing.T) {
	assert := assert.New(t)

	queryToExpected := []struct{ in, expected string }{
		// List compacted and replaced
		{
			"select key, status, modified from org_check_run where org_id = %s AND check IN (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)",
			"SELECT key, modified, status FROM org_check_run WHERE org_id = ? AND check IN ( ? )",
		},
		// Some whitespace-y things
		{
			"select key, status, modified from org_check_run where org_id = %s AND check IN (%s, %s, %s)",
			"SELECT key, modified, status FROM org_check_run WHERE org_id = ? AND check IN ( ? )",
		},
		{
			"select key, status, modified from org_check_run where org_id = %s AND check IN (%s , %s , %s )",
			"SELECT key, modified, status FROM org_check_run WHERE org_id = ? AND check IN ( ? )",
		},
		// %s replaced with ? as in sql quantize
		{
			"select key, status, modified from org_check_run where org_id = %s AND check = %s",
			"SELECT key, modified, status FROM org_check_run WHERE org_id = ? AND check = ?",
		},
		{
			"select key, status, modified from org_check_run where org_id = %s AND check = %s",
			"SELECT key, modified, status FROM org_check_run WHERE org_id = ? AND check = ?",
		},
		{
			"SELECT timestamp, processes FROM process_snapshot.minutely WHERE org_id = ? AND host = ? AND timestamp >= ? AND timestamp <= ?",
			"SELECT processes, timestamp FROM process_snapshot.minutely WHERE org_id = ? AND host = ? AND timestamp >= ? AND timestamp <= ?",
		},
	}

	for _, testCase := range queryToExpected {
		s := CassSpan(testCase.in)
		Quantize(s)
		assert.Equal(testCase.expected, s.Resource)
	}
}
