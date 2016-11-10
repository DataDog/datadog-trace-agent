package quantizer

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DataDog/raclette/model"
)

func SQLSpan(query string) model.Span {
	return model.Span{
		Resource: query,
		Type:     "sql",
		Meta: map[string]string{
			"sql.query": query,
		},
	}
}

type sqlTestCase struct {
	query    string
	expected string
}

func TestSQLResourceQuery(t *testing.T) {
	assert := assert.New(t)
	span := model.Span{
		Resource: "SELECT * FROM users WHERE id = 42",
		Type:     "sql",
		Meta: map[string]string{
			"sql.query": "SELECT * FROM users WHERE id = 42",
		},
	}

	spanQ := Quantize(span)
	assert.Equal("SELECT * FROM users WHERE id = ?", spanQ.Resource)
	assert.Equal("SELECT * FROM users WHERE id = 42", spanQ.Meta["sql.query"])
}

func TestSQLResourceWithoutQuery(t *testing.T) {
	assert := assert.New(t)
	span := model.Span{
		Resource: "SELECT * FROM users WHERE id = 42",
		Type:     "sql",
	}

	spanQ := Quantize(span)
	assert.Equal("SELECT * FROM users WHERE id = ?", spanQ.Resource)
	assert.Equal("SELECT * FROM users WHERE id = ?", spanQ.Meta["sql.query"])
}

func TestSQLQuantizer(t *testing.T) {
	assert := assert.New(t)

	cases := []sqlTestCase{
		{
			"select * from users where id = 42",
			"select * from users where id = ?",
		},
		{
			"SELECT host, status FROM ec2_status WHERE org_id = 42",
			"SELECT host, status FROM ec2_status WHERE org_id = ?",
		},
		{
			"SELECT host, status FROM ec2_status WHERE org_id=42",
			"SELECT host, status FROM ec2_status WHERE org_id=?",
		},
		{
			"-- get user \n--\n select * \n   from users \n    where\n       id = 214325346",
			"select * from users where id = ?",
		},
		{
			"SELECT * FROM `host` WHERE `id` IN (42, 43) /*comment with parameters,host:localhost,url:controller#home,id:FF005:00CAA*/",
			"SELECT * FROM `host` WHERE `id` IN (?)",
		},
		{
			"UPDATE user_dash_pref SET json_prefs = %(json_prefs)s, modified = '2015-08-27 22:10:32.492912' WHERE user_id = %(user_id)s AND url = %(url)s",
			"UPDATE user_dash_pref SET json_prefs = ?, modified = ? WHERE user_id = ? AND url = ?"},
		{
			"SELECT DISTINCT host.id AS host_id FROM host JOIN host_alias ON host_alias.host_id = host.id WHERE host.org_id = %(org_id_1)s AND host.name NOT IN (%(name_1)s) AND host.name IN (%(name_2)s, %(name_3)s, %(name_4)s, %(name_5)s)",
			"SELECT DISTINCT host.id AS host_id FROM host JOIN host_alias ON host_alias.host_id = host.id WHERE host.org_id = ? AND host.name NOT IN (?) AND host.name IN (?)",
		},
		{
			"SELECT org_id,metric_key FROM metrics_metadata WHERE org_id = %(org_id)s AND metric_key = ANY(array[75])",
			"SELECT org_id,metric_key FROM metrics_metadata WHERE org_id = ? AND metric_key = ANY(array[?])",
		},
		{
			"SELECT org_id,metric_key   FROM metrics_metadata   WHERE org_id = %(org_id)s AND metric_key = ANY(array[21, 25, 32])",
			"SELECT org_id,metric_key FROM metrics_metadata WHERE org_id = ? AND metric_key = ANY(array[?])",
		},
		{
			"SELECT articles.* FROM articles WHERE articles.id = 1 LIMIT 1",
			"SELECT articles.* FROM articles WHERE articles.id = ? LIMIT ?",
		},
		{
			"SELECT articles.* FROM articles WHERE articles.id = 1 LIMIT 1;",
			"SELECT articles.* FROM articles WHERE articles.id = ? LIMIT ?",
		},
		{
			"SELECT articles.* FROM articles WHERE (articles.created_at BETWEEN '2016-10-31 23:00:00.000000' AND '2016-11-01 23:00:00.000000')",
			"SELECT articles.* FROM articles WHERE (articles.created_at BETWEEN ? AND ?)",
		},
		{
			"SELECT articles.* FROM articles WHERE (articles.created_at BETWEEN $1 AND $2)",
			"SELECT articles.* FROM articles WHERE (articles.created_at BETWEEN ? AND ?)",
		},
		{
			"SELECT articles.* FROM articles WHERE (articles.published != true)",
			"SELECT articles.* FROM articles WHERE (articles.published != ?)",
		},
		{
			"SELECT articles.* FROM articles WHERE (title = 'guides.rubyonrails.org')",
			"SELECT articles.* FROM articles WHERE (title = ?)",
		},
		{
			"SELECT date(created_at) as ordered_date, sum(price) as total_price FROM orders GROUP BY date(created_at) HAVING sum(price) > 100",
			"SELECT date(created_at) as ordered_date, sum(price) as total_price FROM orders GROUP BY date(created_at) HAVING sum(price) > ?",
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
			"SELECT articles.* FROM articles WHERE articles.id IN (?)",
		},
		{
			"SELECT * FROM clients WHERE (clients.first_name = 'Andy') LIMIT 1 BEGIN INSERT INTO clients (created_at, first_name, locked, orders_count, updated_at) VALUES ('2011-08-30 05:22:57', 'Andy', 1, NULL, '2011-08-30 05:22:57') COMMIT",
			"SELECT * FROM clients WHERE (clients.first_name = ?) LIMIT ? BEGIN INSERT INTO clients (created_at, first_name, locked, orders_count, updated_at) VALUES (?) COMMIT",
		},
	}

	for _, c := range cases {
		assert.Equal(c.expected, Quantize(SQLSpan(c.query)).Resource)
	}
}
