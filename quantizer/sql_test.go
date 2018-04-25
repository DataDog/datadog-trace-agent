package quantizer

import (
	"flag"
	"os"
	"testing"

	log "github.com/cihub/seelog"

	"github.com/DataDog/datadog-trace-agent/model"
	"github.com/stretchr/testify/assert"
)

type sqlTestCase struct {
	query    string
	expected string
}

func SQLSpan(query string) *model.Span {
	return &model.Span{
		Resource: query,
		Type:     "sql",
		Meta: map[string]string{
			"sql.query": query,
		},
	}
}

func TestMain(m *testing.M) {
	flag.Parse()

	// Disable debug logs in these tests
	log.UseLogger(log.Disabled)

	os.Exit(m.Run())
}

func TestSQLResourceQuery(t *testing.T) {
	assert := assert.New(t)
	span := &model.Span{
		Resource: "SELECT * FROM users WHERE id = 42",
		Type:     "sql",
		Meta: map[string]string{
			"sql.query": "SELECT * FROM users WHERE id = 42",
		},
	}

	Quantize(span)
	assert.Equal("SELECT * FROM users WHERE id = ?", span.Resource)
	assert.Equal("SELECT * FROM users WHERE id = 42", span.Meta["sql.query"])
}

func TestSQLResourceWithoutQuery(t *testing.T) {
	assert := assert.New(t)
	span := &model.Span{
		Resource: "SELECT * FROM users WHERE id = 42",
		Type:     "sql",
	}

	Quantize(span)
	assert.Equal("SELECT * FROM users WHERE id = ?", span.Resource)
	assert.Equal("SELECT * FROM users WHERE id = ?", span.Meta["sql.query"])
}

func TestSQLResourceWithError(t *testing.T) {
	assert := assert.New(t)
	testCases := []struct {
		span model.Span
	}{
		{
			model.Span{
				Resource: "SELECT * FROM users WHERE id = '' AND '",
				Type:     "sql",
			},
		},
		{
			model.Span{
				Resource: "INSERT INTO pages (id, name) VALUES (%(id0)s, %(name0)s), (%(id1)s, %(name1",
				Type:     "sql",
			},
		},
		{
			model.Span{
				Resource: "INSERT INTO pages (id, name) VALUES (%(id0)s, %(name0)s), (%(id1)s, %(name1)",
				Type:     "sql",
			},
		},
	}

	for _, tc := range testCases {
		// copy test cases as Quantize mutates
		testSpan := tc.span

		Quantize(&tc.span)
		assert.Equal("Non-parsable SQL query", tc.span.Resource)
		assert.Equal("Query not parsed", tc.span.Meta["agent.parse.error"])
		assert.Equal(testSpan.Resource, tc.span.Meta["sql.query"])
	}
}

func TestSQLQuantizer(t *testing.T) {
	assert := assert.New(t)

	cases := []sqlTestCase{
		{
			"select * from users where id = 42",
			"SELECT * FROM users WHERE id = ?",
		},
		{
			"SELECT host, status FROM ec2_status WHERE org_id = 42",
			"SELECT host, status FROM ec2_status WHERE org_id = ?",
		},
		{
			"SELECT status, host  FROM ec2_status WHERE org_id=42",
			"SELECT host, status FROM ec2_status WHERE org_id = ?",
		},
		{
			"-- get user \n--\n select * \n   from users \n    where\n       id = 214325346",
			"SELECT * FROM users WHERE id = ?",
		},
		{
			"SELECT * FROM `host` WHERE `id` IN (42, 43) /*comment with parameters,host:localhost,url:controller#home,id:FF005:00CAA*/",
			"SELECT * FROM host WHERE id IN ( ? )",
		},
		{
			"SELECT `host`.`address` FROM `host` WHERE org_id=42",
			"SELECT host . address FROM host WHERE org_id = ?",
		},
		{
			`SELECT "host"."address" FROM "host" WHERE org_id=42`,
			`SELECT host . address FROM host WHERE org_id = ?`,
		},
		{
			`SELECT * FROM host WHERE id IN (42, 43) /*
			multiline comment with parameters,
			host:localhost,url:controller#home,id:FF005:00CAA
			*/`,
			"SELECT * FROM host WHERE id IN ( ? )",
		},
		{
			"UPDATE user_dash_pref SET modified = '2015-08-27 22:10:32.492912', json_prefs = %(json_prefs)s WHERE user_id = %(user_id)s AND url = %(url)s",
			"UPDATE user_dash_pref SET json_prefs = ? modified = ? WHERE user_id = ? AND url = ?",
		},
		{
			"UPDATE user_dash_pref SET modified = '2015-08-27 22:10:32.492912', x = x + 1, json_prefs = %(json_prefs)s WHERE user_id = %(user_id)s AND url = %(url)s",
			"UPDATE user_dash_pref SET json_prefs = ? modified = ? x = x + ? WHERE user_id = ? AND url = ?",
		},
		{
			"SELECT DISTINCT host.id AS host_id FROM host JOIN host_alias ON host_alias.host_id = host.id WHERE host.org_id = %(org_id_1)s AND host.name NOT IN (%(name_1)s) AND host.name IN (%(name_2)s, %(name_3)s, %(name_4)s, %(name_5)s)",
			"SELECT DISTINCT host.id FROM host JOIN host_alias ON host_alias.host_id = host.id WHERE host.org_id = ? AND host.name NOT IN ( ? ) AND host.name IN ( ? )",
		},
		{
			"SELECT org_id, metric_key FROM metrics_metadata WHERE org_id = %(org_id)s AND metric_key = ANY(array[75])",
			"SELECT metric_key, org_id FROM metrics_metadata WHERE org_id = ? AND metric_key = ANY ( array [ ? ] )",
		},
		{
			"SELECT org_id, metric_key   FROM metrics_metadata   WHERE org_id = %(org_id)s AND metric_key = ANY(array[21, 25, 32])",
			"SELECT metric_key, org_id FROM metrics_metadata WHERE org_id = ? AND metric_key = ANY ( array [ ? ] )",
		},
		{
			"SELECT articles.* FROM articles WHERE articles.id = 1 LIMIT 1",
			"SELECT articles.* FROM articles WHERE articles.id = ? LIMIT ?",
		},

		{
			"SELECT articles.* FROM articles WHERE articles.id = 1 LIMIT 1, 20",
			"SELECT articles.* FROM articles WHERE articles.id = ? LIMIT ?",
		},
		{
			"SELECT articles.* FROM articles WHERE articles.id = 1 LIMIT 1, 20;",
			"SELECT articles.* FROM articles WHERE articles.id = ? LIMIT ?",
		},
		{
			"SELECT articles.* FROM articles WHERE articles.id = 1 LIMIT 15,20;",
			"SELECT articles.* FROM articles WHERE articles.id = ? LIMIT ?",
		},
		{
			"SELECT articles.* FROM articles WHERE articles.id = 1 LIMIT 1;",
			"SELECT articles.* FROM articles WHERE articles.id = ? LIMIT ?",
		},
		{
			"SELECT articles.* FROM articles WHERE (articles.created_at BETWEEN '2016-10-31 23:00:00.000000' AND '2016-11-01 23:00:00.000000')",
			"SELECT articles.* FROM articles WHERE ( articles.created_at BETWEEN ? AND ? )",
		},
		{
			"SELECT articles.* FROM articles WHERE (articles.created_at BETWEEN $1 AND $2)",
			"SELECT articles.* FROM articles WHERE ( articles.created_at BETWEEN ? AND ? )",
		},
		{
			"SELECT articles.* FROM articles WHERE (articles.published != true)",
			"SELECT articles.* FROM articles WHERE ( articles.published != ? )",
		},
		{
			"SELECT articles.* FROM articles WHERE (title = 'guides.rubyonrails.org')",
			"SELECT articles.* FROM articles WHERE ( title = ? )",
		},
		{
			"SELECT articles.* FROM articles WHERE ( title = ? ) AND ( author = ? )",
			"SELECT articles.* FROM articles WHERE ( title = ? ) AND ( author = ? )",
		},
		{
			"SELECT articles.* FROM articles WHERE ( title = :title )",
			"SELECT articles.* FROM articles WHERE ( title = :title )",
		},
		{
			"SELECT articles.* FROM articles WHERE ( title = @title )",
			"SELECT articles.* FROM articles WHERE ( title = @title )",
		},
		{
			"SELECT date(created_at) as ordered_date, sum(price) as total_price FROM orders GROUP BY date(created_at) HAVING sum(price) > 100",
			"SELECT date ( created_at ), sum ( price ) FROM orders GROUP BY date ( created_at ) HAVING sum ( price ) > ?",
		},
		{
			"SELECT * FROM articles WHERE id > 10 ORDER BY id asc LIMIT 20",
			"SELECT * FROM articles WHERE id > ? ORDER BY id asc LIMIT ?",
		},
		{
			"SELECT clients.* FROM clients INNER JOIN posts ON posts.author_id = author.id AND posts.published = 't'",
			"SELECT clients.* FROM clients INNER JOIN posts ON posts.author_id = author.id AND posts.published = ?",
		},
		{
			"SELECT articles.* FROM articles WHERE articles.id IN (1, 3, 5)",
			"SELECT articles.* FROM articles WHERE articles.id IN ( ? )",
		},
		{
			"SELECT * FROM clients WHERE (clients.first_name = 'Andy') LIMIT 1 BEGIN INSERT INTO clients (created_at, first_name, locked, orders_count, updated_at) VALUES ('2011-08-30 05:22:57', 'Andy', 1, NULL, '2011-08-30 05:22:57') COMMIT",
			"SELECT * FROM clients WHERE ( clients.first_name = ? ) LIMIT ? BEGIN INSERT INTO clients ( created_at, first_name, locked, orders_count, updated_at ) VALUES ( ? ) COMMIT",
		},
		{
			"SELECT * FROM clients WHERE (clients.first_name = 'Andy') LIMIT 15, 25 BEGIN INSERT INTO clients (created_at, first_name, locked, orders_count, updated_at) VALUES ('2011-08-30 05:22:57', 'Andy', 1, NULL, '2011-08-30 05:22:57') COMMIT",
			"SELECT * FROM clients WHERE ( clients.first_name = ? ) LIMIT ? BEGIN INSERT INTO clients ( created_at, first_name, locked, orders_count, updated_at ) VALUES ( ? ) COMMIT",
		},
		{
			"SAVEPOINT \"s139956586256192_x1\"",
			"SAVEPOINT ?",
		},
		{
			"INSERT INTO user (id, username) VALUES ('Fred','Smith'), ('John','Smith'), ('Michael','Smith'), ('Robert','Smith');",
			"INSERT INTO user ( id, username ) VALUES ( ? )",
		},
		{
			"CREATE KEYSPACE Excelsior WITH replication = {'class': 'SimpleStrategy', 'replication_factor' : 3};",
			"CREATE KEYSPACE Excelsior WITH replication = ?",
		},
		{
			`SELECT "webcore_page"."id" FROM "webcore_page" WHERE "webcore_page"."slug" = %s ORDER BY "webcore_page"."path" ASC LIMIT 1`,
			"SELECT webcore_page . id FROM webcore_page WHERE webcore_page . slug = ? ORDER BY webcore_page . path ASC LIMIT ?",
		},
		{
			`INSERT INTO delayed_jobs (attempts, created_at, failed_at, handler, last_error, locked_at, locked_by, priority, queue, run_at, updated_at) VALUES (0, '2016-12-04 17:09:59', NULL, '--- !ruby/object:Delayed::PerformableMethod\nobject: !ruby/object:Item\n  store:\n  - a simple string\n  - an \'escaped \' string\n  - another \'escaped\' string\n  - 42\n  string: a string with many \\\\\'escapes\\\\\'\nmethod_name: :show_store\nargs: []\n', NULL, NULL, NULL, 0, NULL, '2016-12-04 17:09:59', '2016-12-04 17:09:59')`,
			"INSERT INTO delayed_jobs ( attempts, created_at, failed_at, handler, last_error, locked_at, locked_by, priority, queue, run_at, updated_at ) VALUES ( ? )",
		},
		{
			"SELECT name, pretty_print(address) FROM people;",
			"SELECT name, pretty_print ( address ) FROM people",
		},
		{
			"* SELECT * FROM fake_data(1, 2, 3);",
			"* SELECT * FROM fake_data ( ? )",
		},
		{
			"CREATE FUNCTION add(integer, integer) RETURNS integer\n AS 'select $1 + $2;'\n LANGUAGE SQL\n IMMUTABLE\n RETURNS NULL ON NULL INPUT;",
			"CREATE FUNCTION add ( integer, integer ) RETURNS integer LANGUAGE SQL IMMUTABLE RETURNS ? ON ? INPUT",
		},
		{
			"SELECT * FROM public.table ( array [ ROW ( array [ 'magic', 'foo',",
			"SELECT * FROM public.table ( array [ ROW ( array [ ?",
		},
		{
			"SELECT pg_try_advisory_lock (123) AS t46eef3f025cc27feb31ca5a2d668a09a",
			"SELECT pg_try_advisory_lock ( ? )",
		},
		{
			"INSERT INTO `qual-aa`.issues (alert0 , alert1) VALUES (NULL, NULL)",
			"INSERT INTO qual-aa . issues ( alert0, alert1 ) VALUES ( ? )",
		},
		{
			"INSERT INTO user (id, email, name) VALUES (null, ?, ?)",
			"INSERT INTO user ( id, email, name ) VALUES ( ? )",
		},
		{
			"select * from users where id = 214325346     # This comment continues to the end of line",
			"SELECT * FROM users WHERE id = ?",
		},
		{
			"select * from users where id = 214325346     -- This comment continues to the end of line",
			"SELECT * FROM users WHERE id = ?",
		},
		{
			"SELECT * FROM /* this is an in-line comment */ users;",
			"SELECT * FROM users",
		},
		{
			"SELECT /*! STRAIGHT_JOIN */ col1 FROM table1",
			"SELECT col1 FROM table1",
		},
		{
			`DELETE FROM t1
			WHERE s11 > ANY
			(SELECT COUNT(*) /* no hint */ FROM t2
			WHERE NOT EXISTS
			(SELECT * FROM t3
			WHERE ROW(5*t2.s1,77)=
			(SELECT 50,11*s1 FROM t4 UNION SELECT 50,77 FROM
			(SELECT * FROM t5) AS t5)));`,
			"DELETE FROM t1 WHERE s11 > ANY ( SELECT COUNT ( * ) FROM t2 WHERE NOT EXISTS ( SELECT * FROM t3 WHERE ROW ( ? * t2.s1, ? ) = ( SELECT ? * s1 FROM t4 UNION SELECT ? FROM ( SELECT * FROM t5 ) ) ) )",
		},
		{
			"SET @g = 'POLYGON((0 0,10 0,10 10,0 10,0 0),(5 5,7 5,7 7,5 7, 5 5))';",
			"SET @g = ?",
		},
		{
			"SELECT d.y AS x, c.x, b, a FROM metrics_metadata WHERE org_id = %(org_id)s AND metric_key = ANY(array[75])",
			"SELECT a, b, c.x, d.y FROM metrics_metadata WHERE org_id = ? AND metric_key = ANY ( array [ ? ] )",
		},
		{
			"SELECT server_table.host AS host_id FROM table#.host_tags as server_table WHERE server_table.host_id = 50",
			"SELECT server_table.host FROM table#.host_tags WHERE server_table.host_id = ?",
		},
		{
			"UPDATE user_dash_pref SET modified = '2015-08-27 22:10:32.492912', x = x + 1, json_prefs = %(json_prefs)s WHERE user_id = %(user_id)s AND url = %(url)s",
			"UPDATE user_dash_pref SET json_prefs = ? modified = ? x = x + ? WHERE user_id = ? AND url = ?",
		},
		{
			"INSERT INTO `qual-aa`.issues (alert0 , alert1) VALUES (NULL, NULL)",
			"INSERT INTO qual-aa . issues ( alert0, alert1 ) VALUES ( ? )",
		},
		{
			"UPDATE tickets SET subject=1, latest_comment_added_at=1, latest_public_comment_added_at=1, latest_agent_comment_added_at=1, updated_at=1, inbox_updated_at=1 WHERE tickets.id=1",
			"UPDATE tickets SET inbox_updated_at = ? latest_agent_comment_added_at = ? latest_comment_added_at = ? latest_public_comment_added_at = ? subject = ? updated_at = ? WHERE tickets.id = ?",
		},
		{
			"UPDATE tickets SET group_id = 1, current_tags = 1, status_id = 1, base_score = 1, solved_at = 1, assignee_updated_at = 1, status_updated_at = 1, resolution_time = 1, latest_comment_added_at = 1, latest_public_comment_added_at = 1, latest_agent_comment_added_at = 1, updated_at = 1, inbox_updated_at = 1 WHERE tickets.id = 1",
			"UPDATE tickets SET assignee_updated_at = ? base_score = ? current_tags = ? group_id = ? inbox_updated_at = ? latest_agent_comment_added_at = ? latest_comment_added_at = ? latest_public_comment_added_at = ? resolution_time = ? solved_at = ? status_id = ? status_updated_at = ? updated_at = ? WHERE tickets.id = ?",
		}, {
			"UPDATE cia_events SET cia_events.visible=1 WHERE cia_events.id=1",
			"UPDATE cia_events SET cia_events.visible = ? WHERE cia_events.id = ?",
		},
		{
			"UPDATE tickets SET status_id=1, current_tags=1, status_updated_at=1, latest_comment_added_at=1, latest_public_comment_added_at=1, updated_at=1, inbox_updated_at=1 WHERE tickets.id=1",
			"UPDATE tickets SET current_tags = ? inbox_updated_at = ? latest_comment_added_at = ? latest_public_comment_added_at = ? status_id = ? status_updated_at = ? updated_at = ? WHERE tickets.id = ?",
		},
		{
			"UPDATE tickets SET group_id=1, base_score=1, assignee_updated_at=1, updated_at=1, inbox_updated_at=1 WHERE tickets.id=1",
			"UPDATE tickets SET assignee_updated_at = ? base_score = ? group_id = ? inbox_updated_at = ? updated_at = ? WHERE tickets.id = ?",
		},
		{
			"UPDATE tickets SET status_id=1, base_score=1, current_tags=1, assignee_id=1, status_updated_at=1, resolution_time=1, solved_at=1, latest_comment_added_at=1, latest_public_comment_added_at=1, updated_at=1, inbox_updated_at=1 WHERE tickets.id=1",
			"UPDATE tickets SET assignee_id = ? base_score = ? current_tags = ? inbox_updated_at = ? latest_comment_added_at = ? latest_public_comment_added_at = ? resolution_time = ? solved_at = ? status_id = ? status_updated_at = ? updated_at = ? WHERE tickets.id = ?",
		},
		{
			"UPDATE tickets SET subject=1, current_tags=1, assignee_updated_at=1, latest_comment_added_at=1, latest_agent_comment_added_at=1, updated_at=1, inbox_updated_at=1 WHERE tickets.id=1",
			"UPDATE tickets SET assignee_updated_at = ? current_tags = ? inbox_updated_at = ? latest_agent_comment_added_at = ? latest_comment_added_at = ? subject = ? updated_at = ? WHERE tickets.id = ?",
		},
		{
			"UPDATE ticket_metric_sets SET full_resolution_time_in_minutes = 1, full_resolution_time_in_minutes_within_business_hours = 1, full_resolution_time_in_hours = 1, replies = 1, first_reply_time_in_minutes = 1, first_reply_time_in_minutes_within_business_hours = 1, requester_wait_time_in_minutes = 1, requester_wait_time_in_minutes_within_business_hours = 1, updated_at = 1 WHERE ticket_metric_sets.id = 1",
			"UPDATE ticket_metric_sets SET first_reply_time_in_minutes = ? first_reply_time_in_minutes_within_business_hours = ? full_resolution_time_in_hours = ? full_resolution_time_in_minutes = ? full_resolution_time_in_minutes_within_business_hours = ? replies = ? requester_wait_time_in_minutes = ? requester_wait_time_in_minutes_within_business_hours = ? updated_at = ? WHERE ticket_metric_sets.id = ?",
		},
		{
			"UPDATE tickets SET assignee_id = 1, group_id = 1, status_id = 1, current_collaborators = 1, base_score = 1, assignee_updated_at = 1, updated_by_type_id = 1, status_updated_at = 1, assigned_at = 1, initially_assigned_at = 1, latest_comment_added_at = 1, latest_public_comment_added_at = 1, latest_agent_comment_added_at = 1, updated_at = 1, inbox_updated_at = 1 WHERE tickets.id = 1",
			"UPDATE tickets SET assigned_at = ? assignee_id = ? assignee_updated_at = ? base_score = ? current_collaborators = ? group_id = ? inbox_updated_at = ? initially_assigned_at = ? latest_agent_comment_added_at = ? latest_comment_added_at = ? latest_public_comment_added_at = ? status_id = ? status_updated_at = ? updated_at = ? updated_by_type_id = ? WHERE tickets.id = ?",
		},
		{
			"UPDATE devices SET ip = 1, user_agent = 1, token = 1, updated_at = 1 WHERE devices.type IN ( 1 ) AND devices.id = 1",
			"UPDATE devices SET ip = ? token = ? updated_at = ? user_agent = ? WHERE devices.type IN ( ? ) AND devices.id = ?",
		},
		{
			`SELECT * FROM ( ( SELECT tickets.id, tickets.account_id, tickets.requester_id, tickets.submitter_id, tickets.assignee_id, tickets.group_id, tickets.status_id, tickets.priority_id, tickets.via_id, tickets.ticket_type_id, tickets.linked_id, tickets.created_at, tickets.updated_at, tickets.description, tickets.assignee_updated_at, tickets.requester_updated_at, tickets.assigned_at, tickets.status_updated_at, tickets.nice_id, tickets.recipient, tickets.organization_id, tickets.due_date, tickets.initially_assigned_at, tickets.solved_at, tickets.resolution_time, tickets.current_tags, tickets.current_collaborators, tickets.updated_by_type_id, tickets.subject, tickets.external_id, tickets.original_recipient_address, tickets.base_score, tickets.entry_id, tickets.generated_timestamp, tickets.satisfaction_score, tickets.locale_id, tickets.latest_comment_added_at, tickets.latest_agent_comment_added_at, tickets.latest_public_comment_added_at, tickets.ticket_form_id, tickets.brand_id, tickets.inbox_updated_at, tickets.sla_breach_status, tickets.satisfaction_reason_code, tickets.number_of_incidents, tickets.via_reference_id, tickets.is_public, "str123" FROM tickets WHERE tickets.requester_id = "str123" AND ( tickets.status_id NOT IN ( "str123" ) ) ORDER BY created_at ASC, id ASC LIMIT 20 ) UNION ALL ( SELECT ticket_archive_stubs.id, ticket_archive_stubs.account_id, ticket_archive_stubs.requester_id, "str123", ticket_archive_stubs.assignee_id, ticket_archive_stubs.group_id, ticket_archive_stubs.status_id, "str123", "str123", ticket_archive_stubs.ticket_type_id, ticket_archive_stubs.linked_id, ticket_archive_stubs.created_at, ticket_archive_stubs.updated_at, ticket_archive_stubs.description, "str123", "str123", "str123", "str123", ticket_archive_stubs.nice_id, "str123", ticket_archive_stubs.organization_id, "str123", "str123", "str123", "str123", "str123", "str123", "str123", ticket_archive_stubs.subject, ticket_archive_stubs.external_id, "str123", "str123", "str123", ticket_archive_stubs.generated_timestamp, "str123", "str123", "str123", "str123", "str123", "str123", ticket_archive_stubs.brand_id, "str123", "str123", "str123", "str123", "str123", ticket_archive_stubs.is_public, "str123" FROM ticket_archive_stubs WHERE ( ticket_archive_stubs.requester_id = "str123" AND ( ticket_archive_stubs.status_id NOT IN ( "str123" ) ) ) ORDER BY created_at ASC, id ASC LIMIT 20 ) ) ORDER BY created_at asc, id asc LIMIT 20 OFFSET "str123"`,
			"SELECT * FROM ( ( SELECT str123, tickets.account_id, tickets.assigned_at, tickets.assignee_id, tickets.assignee_updated_at, tickets.base_score, tickets.brand_id, tickets.created_at, tickets.current_collaborators, tickets.current_tags, tickets.description, tickets.due_date, tickets.entry_id, tickets.external_id, tickets.generated_timestamp, tickets.group_id, tickets.id, tickets.inbox_updated_at, tickets.initially_assigned_at, tickets.is_public, tickets.latest_agent_comment_added_at, tickets.latest_comment_added_at, tickets.latest_public_comment_added_at, tickets.linked_id, tickets.locale_id, tickets.nice_id, tickets.number_of_incidents, tickets.organization_id, tickets.original_recipient_address, tickets.priority_id, tickets.recipient, tickets.requester_id, tickets.requester_updated_at, tickets.resolution_time, tickets.satisfaction_reason_code, tickets.satisfaction_score, tickets.sla_breach_status, tickets.solved_at, tickets.status_id, tickets.status_updated_at, tickets.subject, tickets.submitter_id, tickets.ticket_form_id, tickets.ticket_type_id, tickets.updated_at, tickets.updated_by_type_id, tickets.via_id, tickets.via_reference_id FROM tickets WHERE tickets.requester_id = str123 AND ( tickets.status_id NOT IN ( str123 ) ) ORDER BY created_at ASC, id ASC LIMIT ? ) UNION ALL ( SELECT str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, str123, ticket_archive_stubs.account_id, ticket_archive_stubs.assignee_id, ticket_archive_stubs.brand_id, ticket_archive_stubs.created_at, ticket_archive_stubs.description, ticket_archive_stubs.external_id, ticket_archive_stubs.generated_timestamp, ticket_archive_stubs.group_id, ticket_archive_stubs.id, ticket_archive_stubs.is_public, ticket_archive_stubs.linked_id, ticket_archive_stubs.nice_id, ticket_archive_stubs.organization_id, ticket_archive_stubs.requester_id, ticket_archive_stubs.status_id, ticket_archive_stubs.subject, ticket_archive_stubs.ticket_type_id, ticket_archive_stubs.updated_at FROM ticket_archive_stubs WHERE ( ticket_archive_stubs.requester_id = str123 AND ( ticket_archive_stubs.status_id NOT IN ( str123 ) ) ) ORDER BY created_at ASC, id ASC LIMIT ? ) ) ORDER BY created_at asc, id asc LIMIT ? OFFSET str123",
		},
		{
			"zd_shard 890 / SELECT c,b,a FROM permission_sets WHERE permission_sets.account_id = 1 AND permission_sets.id = 3 LIMIT 1",
			"zd_shard ? / SELECT a, b, c FROM permission_sets WHERE permission_sets.account_id = ? AND permission_sets.id = ? LIMIT ?",
		},
		{
			"zd_shard 1 / SELECT COUNT ( DISTINCT users.id ) FROM users LEFT OUTER JOIN memberships ON memberships.user_id = users.id LEFT OUTER JOIN groups ON groups.id = memberships.group_id AND groups.is_active = 1 LEFT OUTER JOIN user_identities ON user_identities.user_id = users.id LEFT OUTER JOIN photos ON photos.user_id = users.id AND photos.thumbnail IS 1 LEFT OUTER JOIN user_settings ON user_settings.user_id = users.id LEFT OUTER JOIN signatures ON signatures.user_id = users.id LEFT OUTER JOIN taggings ON taggings.taggable_id = users.id AND taggings.taggable_type = 1 LEFT OUTER JOIN user_texts ON user_texts.user_id = users.id WHERE users.account_id = 1 AND users.is_active = 1 AND users.roles IN ( 1 ) AND user_identities.value = 1 AND user_identities.type = 1 AND user_identities.account_id = 1 AND users.roles IN ( 1 ) AND user_identities.value = 1 AND user_identities.type = 1 AND user_identities.account_id = 1",
			"zd_shard ? / SELECT COUNT ( DISTINCT users.id ) FROM users LEFT OUTER JOIN memberships ON memberships.user_id = users.id LEFT OUTER JOIN groups ON groups.id = memberships.group_id AND groups.is_active = ? LEFT OUTER JOIN user_identities ON user_identities.user_id = users.id LEFT OUTER JOIN photos ON photos.user_id = users.id AND photos.thumbnail IS ? LEFT OUTER JOIN user_settings ON user_settings.user_id = users.id LEFT OUTER JOIN signatures ON signatures.user_id = users.id LEFT OUTER JOIN taggings ON taggings.taggable_id = users.id AND taggings.taggable_type = ? LEFT OUTER JOIN user_texts ON user_texts.user_id = users.id WHERE users.account_id = ? AND users.is_active = ? AND users.roles IN ( ? ) AND user_identities.value = ? AND user_identities.type = ? AND user_identities.account_id = ? AND users.roles IN ( ? ) AND user_identities.value = ? AND user_identities.type = ? AND user_identities.account_id = ?",
		},
		{
			"shard_tag / SELECT DISTINCT translation_locales.* FROM translation_locales LEFT JOIN accounts_allowed_translation_locales ON accounts_allowed_translation_locales.translation_locale_id = translation_locales.id AND accounts_allowed_translation_locales.account_id = 1 WHERE translation_locales.deleted_at IS 1 AND ( translation_locales.public = 1 OR translation_locales.official_translation = 1 OR translation_locales.account_id = 1 OR accounts_allowed_translation_locales.account_id IS NOT 1 )",
			"shard_tag / SELECT DISTINCT translation_locales.* FROM translation_locales LEFT JOIN accounts_allowed_translation_locales ON accounts_allowed_translation_locales.translation_locale_id = translation_locales.id AND accounts_allowed_translation_locales.account_id = ? WHERE translation_locales.deleted_at IS ? AND ( translation_locales.public = ? OR translation_locales.official_translation = ? OR translation_locales.account_id = ? OR accounts_allowed_translation_locales.account_id IS NOT ? )",
		},
		{
			"zd_shard 4 / SELECT MAX ( user_identities.priority ) FROM user_identities WHERE user_identities.type IN ( 1 ) AND user_identities.type = 1 AND user_identities.user_id IS 1",
			"zd_shard ? / SELECT MAX ( user_identities.priority ) FROM user_identities WHERE user_identities.type IN ( ? ) AND user_identities.type = ? AND user_identities.user_id IS ?",
		},
		{
			"Update C Set C.Name = CAST(p.Number as varchar(10)) + '|'+ C.Name FROM Catelog.Component C JOIN Catelog.ComponentPart cp ON p.ID = cp.PartID JOIN Catelog.Component c ON cp.ComponentID = c.ID where p.BrandID = 1003 AND ct.Name='Door' + '|'+ C.Name;",
			"Update C SET C.Name = CAST ( p.Number ( ? ) ) + ? + C.Name FROM Catelog.Component C JOIN Catelog.ComponentPart cp ON p.ID = cp.PartID JOIN Catelog.Component c ON cp.ComponentID = c.ID WHERE p.BrandID = ? AND ct.Name = ? + ? + C.Name",
		},
		/* PROBLEMATIC QUERIES:
		{
			"UPDATE table1 SET column1 = (SELECT expression1 FROM table2 WHERE conditions), a = 2 [WHERE conditions];",
			"",
		},
		*/
	}

	for _, c := range cases {
		s := SQLSpan(c.query)
		Quantize(s)
		assert.Equal(c.expected, s.Resource)
	}
}

func TestMultipleProcess(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		query    string
		expected string
	}{
		{
			"SELECT clients.* FROM clients INNER JOIN posts ON posts.author_id = author.id AND posts.published = 't'",
			"SELECT clients.* FROM clients INNER JOIN posts ON posts.author_id = author.id AND posts.published = ?",
		},
		{
			"SELECT articles.* FROM articles WHERE articles.id IN (1, 3, 5)",
			"SELECT articles.* FROM articles WHERE articles.id IN ( ? )",
		},
	}

	filters := []TokenFilter{
		&DiscardFilter{},
		&ReplaceFilter{},
		&GroupingFilter{},
	}

	// The consumer is the same between executions
	consumer := NewTokenConsumer(filters)

	for _, tc := range testCases {
		output, err := consumer.Process(tc.query)
		assert.Nil(err)
		assert.Equal(tc.expected, output)
	}
}

func TestConsumerError(t *testing.T) {
	assert := assert.New(t)

	// Malformed SQL is not accepted and the outer component knows
	// what to do with malformed SQL
	input := "SELECT * FROM users WHERE users.id = '1 AND users.name = 'dog'"
	filters := []TokenFilter{
		&DiscardFilter{},
		&ReplaceFilter{},
		&GroupingFilter{},
	}
	consumer := NewTokenConsumer(filters)

	output, err := consumer.Process(input)
	assert.NotNil(err)
	assert.Equal("", output)
}

// Benchmark the Tokenizer using a SQL statement
func BenchmarkTokenizer(b *testing.B) {
	benchmarks := []struct {
		name  string
		query string
	}{
		{"Escaping", `INSERT INTO delayed_jobs (attempts, created_at, failed_at, handler, last_error, locked_at, locked_by, priority, queue, run_at, updated_at) VALUES (0, '2016-12-04 17:09:59', NULL, '--- !ruby/object:Delayed::PerformableMethod\nobject: !ruby/object:Item\n  store:\n  - a simple string\n  - an \'escaped \' string\n  - another \'escaped\' string\n  - 42\n  string: a string with many \\\\\'escapes\\\\\'\nmethod_name: :show_store\nargs: []\n', NULL, NULL, NULL, 0, NULL, '2016-12-04 17:09:59', '2016-12-04 17:09:59')`},
		{"Grouping", `INSERT INTO delayed_jobs (created_at, failed_at, handler) VALUES (0, '2016-12-04 17:09:59', NULL), (0, '2016-12-04 17:09:59', NULL), (0, '2016-12-04 17:09:59', NULL), (0, '2016-12-04 17:09:59', NULL)`},
	}
	filters := []TokenFilter{
		&DiscardFilter{},
		&ReplaceFilter{},
		&GroupingFilter{},
	}
	consumer := NewTokenConsumer(filters)

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = consumer.Process(bm.query)
			}
		})
	}
}

func BenchmarkQuantizeSQL(b *testing.B) {
	queries := [...]string{
		"SELECT host, status FROM ec2_status WHERE org_id=42",
		"UPDATE user_dash_pref SET json_prefs = %(json_prefs)s, modified = '2015-08-27 22:10:32.492912' WHERE user_id = %(user_id)s AND url = %(url)s",
		"CREATE FUNCTION add(integer, integer) RETURNS integer\n AS 'select $1 + $2;'\n LANGUAGE SQL\n IMMUTABLE\n RETURNS NULL ON NULL INPUT;",
	}
	span := new(model.Span)

	b.Run("SELECT", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			span.Resource = queries[0]
			QuantizeSQL(span)
		}
	})

	b.Run("UPDATE", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			span.Resource = queries[1]
			QuantizeSQL(span)
		}
	})

	b.Run("OTHER", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			span.Resource = queries[2]
			QuantizeSQL(span)
		}
	})
}

func BenchmarkSortFilter(b *testing.B) {
	type testCase struct {
		token, lastToken int
		buffer           []byte
	}
	caseSelect := []testCase{
		// SELECT d.y AS x, c.x, b, a FROM metrics_metadata WHERE org_id = %(org_id)s AND metric_key = ANY(array[75])
		{token: 57367, lastToken: 0, buffer: []byte{83, 69, 76, 69, 67, 84}},
		{token: 57347, lastToken: 57367, buffer: []byte{100, 46, 121}},
		{token: 57365, lastToken: 57347, buffer: []byte{}},
		{token: 57370, lastToken: 57365, buffer: []byte{}},
		{token: 44, lastToken: 57370, buffer: []byte{44}},
		{token: 57347, lastToken: 44, buffer: []byte{99, 46, 120}},
		{token: 44, lastToken: 57347, buffer: []byte{44}},
		{token: 57347, lastToken: 44, buffer: []byte{98}},
		{token: 44, lastToken: 57347, buffer: []byte{44}},
		{token: 57347, lastToken: 44, buffer: []byte{97}},
		{token: 57369, lastToken: 57347, buffer: []byte{70, 82, 79, 77}},
		{token: 57347, lastToken: 57369, buffer: []byte{109, 101, 116, 114, 105, 99, 115, 95, 109, 101, 116, 97, 100, 97, 116, 97}},
		{token: 57369, lastToken: 57347, buffer: []byte{87, 72, 69, 82, 69}},
		{token: 57347, lastToken: 57369, buffer: []byte{111, 114, 103, 95, 105, 100}},
		{token: 61, lastToken: 57347, buffer: []byte{61}},
		{token: 57364, lastToken: 61, buffer: []byte{63}},
		{token: 57347, lastToken: 57364, buffer: []byte{65, 78, 68}},
		{token: 57347, lastToken: 57347, buffer: []byte{109, 101, 116, 114, 105, 99, 95, 107, 101, 121}},
		{token: 61, lastToken: 57347, buffer: []byte{61}},
		{token: 57347, lastToken: 61, buffer: []byte{65, 78, 89}},
		{token: 40, lastToken: 57347, buffer: []byte{40}},
		{token: 57347, lastToken: 40, buffer: []byte{97, 114, 114, 97, 121}},
		{token: 91, lastToken: 57347, buffer: []byte{91}},
		{token: 57364, lastToken: 91, buffer: []byte{63}},
		{token: 93, lastToken: 57364, buffer: []byte{93}},
		{token: 41, lastToken: 93, buffer: []byte{41}},
		{token: 256, lastToken: 41, buffer: []byte{}},
	}
	caseUpdate := []testCase{
		// UPDATE user_dash_pref SET modified = '2015-08-27 22:10:32.492912', x = x + 1, json_prefs = %(json_prefs)s WHERE user_id = %(user_id)s AND url = %(url)s
		{token: 57347, lastToken: 0, buffer: []byte{85, 80, 68, 65, 84, 69}},
		{token: 57347, lastToken: 57347, buffer: []byte{117, 115, 101, 114, 95, 100, 97, 115, 104, 95, 112, 114, 101, 102}},
		{token: 57368, lastToken: 57347, buffer: []byte{83, 69, 84}},
		{token: 57347, lastToken: 57368, buffer: []byte{109, 111, 100, 105, 102, 105, 101, 100}},
		{token: 61, lastToken: 57347, buffer: []byte{61}},
		{token: 57364, lastToken: 61, buffer: []byte{63}},
		{token: 57366, lastToken: 57364, buffer: []byte{}},
		{token: 57347, lastToken: 57366, buffer: []byte{120}},
		{token: 61, lastToken: 57347, buffer: []byte{61}},
		{token: 57347, lastToken: 61, buffer: []byte{120}},
		{token: 43, lastToken: 57347, buffer: []byte{43}},
		{token: 57364, lastToken: 43, buffer: []byte{63}},
		{token: 57366, lastToken: 57364, buffer: []byte{}},
		{token: 57347, lastToken: 57366, buffer: []byte{106, 115, 111, 110, 95, 112, 114, 101, 102, 115}},
		{token: 61, lastToken: 57347, buffer: []byte{61}},
		{token: 57364, lastToken: 61, buffer: []byte{63}},
		{token: 57369, lastToken: 57364, buffer: []byte{87, 72, 69, 82, 69}},
		{token: 57347, lastToken: 57369, buffer: []byte{117, 115, 101, 114, 95, 105, 100}},
		{token: 61, lastToken: 57347, buffer: []byte{61}},
		{token: 57364, lastToken: 61, buffer: []byte{63}},
		{token: 57347, lastToken: 57364, buffer: []byte{65, 78, 68}},
		{token: 57347, lastToken: 57347, buffer: []byte{117, 114, 108}},
		{token: 61, lastToken: 57347, buffer: []byte{61}},
		{token: 57364, lastToken: 61, buffer: []byte{63}},
		{token: 256, lastToken: 57364, buffer: []byte{}},
	}
	caseOther := []testCase{
		// INSERT INTO `qual-aa`.issues (alert0 , alert1) VALUES (NULL, NULL)
		{token: 57347, lastToken: 0, buffer: []byte{73, 78, 83, 69, 82, 84}},
		{token: 57347, lastToken: 57347, buffer: []byte{73, 78, 84, 79}},
		{token: 57347, lastToken: 57347, buffer: []byte{113, 117, 97, 108, 45, 97, 97}},
		{token: 46, lastToken: 57347, buffer: []byte{46}},
		{token: 57347, lastToken: 46, buffer: []byte{105, 115, 115, 117, 101, 115}},
		{token: 40, lastToken: 57347, buffer: []byte{40}},
		{token: 57347, lastToken: 40, buffer: []byte{97, 108, 101, 114, 116, 48}},
		{token: 44, lastToken: 57347, buffer: []byte{44}},
		{token: 57347, lastToken: 44, buffer: []byte{97, 108, 101, 114, 116, 49}},
		{token: 41, lastToken: 57347, buffer: []byte{41}},
		{token: 57347, lastToken: 41, buffer: []byte{86, 65, 76, 85, 69, 83}},
		{token: 40, lastToken: 57347, buffer: []byte{40}},
		{token: 57364, lastToken: 40, buffer: []byte{63}},
		{token: 57366, lastToken: 57364, buffer: []byte{}},
		{token: 57364, lastToken: 57366, buffer: []byte{}},
		{token: 41, lastToken: 57364, buffer: []byte{41}},
		{token: 256, lastToken: 41, buffer: []byte{}},
	}

	b.Run("SELECT", func(b *testing.B) {
		b.ReportAllocs()
		sf := newSortFilter()
		for i := 0; i < b.N; i++ {
			for _, tc := range caseSelect {
				sf.Filter(tc.token, tc.lastToken, tc.buffer)
			}
		}
	})

	b.Run("UPDATE", func(b *testing.B) {
		b.ReportAllocs()
		sf := newSortFilter()
		for i := 0; i < b.N; i++ {
			for _, tc := range caseUpdate {
				sf.Filter(tc.token, tc.lastToken, tc.buffer)
			}
		}
	})

	b.Run("OTHER", func(b *testing.B) {
		b.ReportAllocs()
		sf := newSortFilter()
		for i := 0; i < b.N; i++ {
			for _, tc := range caseOther {
				sf.Filter(tc.token, tc.lastToken, tc.buffer)
			}
		}
	})
}
