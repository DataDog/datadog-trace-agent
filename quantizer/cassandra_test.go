package quantizer

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DataDog/raclette/model"
)

func CassSpan(query string) model.Span {
	return model.Span{
		Resource: "",
		Type:     "cassandra",
		Meta: map[string]string{
			"query": query,
		},
	}
}

type cassTestCase struct {
	query            string
	expectedResource string
}

func TestCassQuantizer(t *testing.T) {
	assert := assert.New(t)

	queryToExpected := []cassTestCase{
		// List compacted and replaced
		{"select key, status, modified from org_check_run where org_id = %s and check in (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)", "select key, status, modified from org_check_run where org_id = ? and check in (?)"},

		// %s replaced as with sql quantize
		{"select key, status, modified from org_check_run where org_id = %s and check = %s", "select key, status, modified from org_check_run where org_id = ? and check = ?"},

		// unchanged
		{"SELECT timestamp, processes FROM process_snapshot.minutely WHERE org_id = ? AND host = ? AND timestamp >= ? AND timestamp <= ?", "SELECT timestamp, processes FROM process_snapshot.minutely WHERE org_id = ? AND host = ? AND timestamp >= ? AND timestamp <= ?"},
	}

	for _, testCase := range queryToExpected {
		assert.Equal(testCase.expectedResource, Quantize(CassSpan(testCase.query)).Resource)
	}
}
