package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DataDog/raclette/model"
)

var quantizer = NewQuantizer()

func SQLSpan(query string) model.Span {
	return model.Span{
		Resource: "",
		Type:     "sql",
		Meta: map[string]string{
			"query": query,
		},
	}
}

func TestSQLQuantizer(t *testing.T) {
	assert := assert.New(t)

	queryToExpected := map[string]string{
		"select * from users where id = 42": "select * from users where id = ?",

		"UPDATE user_dash_pref SET json_prefs = %(json_prefs)s, modified = '2015-08-27 22:10:32.492912' WHERE user_id = %(user_id)s AND url = %(url)s": "UPDATE user_dash_pref SET json_prefs = ?, modified = ? WHERE user_id = ? AND url = ?",

		"-- get_incidents \n-- select aggregate_event_id, earliest_ack, latest_recovery, u.id as user_id, u.handle, u.name, u.role, u.team, u.org_id, u.email from incident i left join dduser u on i.ack_user_id = u.id where aggregate_event_id = any(%(event_ids)s)": "get_incidents",

		"SELECT DISTINCT host.id AS host_id, host.org_id AS host_org_id, host.name AS host_name, host.ip AS host_ip FROM host JOIN host_alias ON host_alias.host_id = host.id WHERE host.org_id = %(org_id_1)s AND host.name NOT IN (%(name_1)s) AND host.name IN (%(name_2)s, %(name_3)s, %(name_4)s, %(name_5)s, %(name_6)s, %(name_7)s, %(name_8)s, %(name_9)s, %(name_10)s, %(name_11)s, %(name_12)s, %(name_13)s, %(name_14)s, %(name_15)s, %(name_16)s, %(name_17)s, %(name_18)s, %(name_19)s, %(name_20)s, %(name_21)s, %(name_22)s, %(name_23)s, %(name_24)s, %(name_25)s, %(name_26)s, %(name_27)s, %(name_28)s, %(name_29)s, %(name_30)s, %(name_31)s, %(name_32)s, %(name_33)s, %(name_34)s, %(name_35)s, %(name_36)s, %(name_37)s, %(name_38)s, %(name_39)s, %(name_40)s, %(name_41)s, %(name_42)s)": "SELECT DISTINCT host.id AS host_id, host.org_id AS host_org_id, host.name AS host_name, host.ip AS host_ip FROM host JOIN host_alias ON host_alias.host_id = host.id WHERE host.org_id = ? AND host.name NOT IN (?) AND host.name IN (?)",
	}

	for query, expectedResource := range queryToExpected {
		assert.Equal(expectedResource, quantizer.Quantize(SQLSpan(query)).Resource)
	}

}
