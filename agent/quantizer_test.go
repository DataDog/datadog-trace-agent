package main

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DataDog/raclette/model"
)

func NewTestQuantizer() (*Quantizer, chan model.Span) {
	inSpans := make(chan model.Span)
	exit := make(chan struct{})
	var exitGroup sync.WaitGroup
	return NewQuantizer(inSpans, exit, &exitGroup)
}

func SQLSpan(query string) model.Span {
	return model.Span{
		Resource: "",
		Type:     "sql",
		Meta: map[string]string{
			"query": query,
		},
	}
}

type sqlTestCase struct {
	query            string
	expectedResource string
}

func TestSQLQuantizer(t *testing.T) {
	assert := assert.New(t)

	quantizer, _ := NewTestQuantizer()

	queryToExpected := []sqlTestCase{
		{"select * from users where id = 42",
			"select * from users where id = ?"},

		{"UPDATE user_dash_pref SET json_prefs = %(json_prefs)s, modified = '2015-08-27 22:10:32.492912' WHERE user_id = %(user_id)s AND url = %(url)s",
			"UPDATE user_dash_pref SET json_prefs = ?, modified = ? WHERE user_id = ? AND url = ?"},

		{"-- get_incidents \n-- select aggregate_event_id from incident i left join dduser u on i.ack_user_id = u.id where aggregate_event_id = any(%(event_ids)s)",
			"get_incidents"},

		{"SELECT DISTINCT host.id AS host_id FROM host JOIN host_alias ON host_alias.host_id = host.id WHERE host.org_id = %(org_id_1)s AND host.name NOT IN (%(name_1)s) AND host.name IN (%(name_2)s, %(name_3)s, %(name_4)s, %(name_5)s)",
			"SELECT DISTINCT host.id AS host_id FROM host JOIN host_alias ON host_alias.host_id = host.id WHERE host.org_id = ? AND host.name NOT IN (?) AND host.name IN (?)"},

		{"SELECT org_id,metric_key FROM metrics_metadata WHERE org_id = %(org_id)s AND metric_key = ANY(array[75])",
			"SELECT org_id,metric_key FROM metrics_metadata WHERE org_id = ? AND metric_key = ANY(array[?])"},

		{"SELECT org_id,metric_key FROM metrics_metadata WHERE org_id = %(org_id)s AND metric_key = ANY(array[21, 25, 32])",
			"SELECT org_id,metric_key FROM metrics_metadata WHERE org_id = ? AND metric_key = ANY(array[?])"},
	}

	for _, testCase := range queryToExpected {
		assert.Equal(testCase.expectedResource, quantizer.Quantize(SQLSpan(testCase.query)).Resource)
	}

}
