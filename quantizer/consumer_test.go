package quantizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			"SELECT articles.* FROM articles WHERE articles.id IN ?",
		},
	}

	filters := []TokenFilter{
		&DiscardFilter{},
		&ReplaceFilter{},
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
	}
	consumer := NewTokenConsumer(filters)

	output, err := consumer.Process(input)
	assert.NotNil(err)
	assert.Equal("", output)
}

// Benchmark the Tokenizer using a SQL statement
func BenchmarkTokenizer(b *testing.B) {
	query := `INSERT INTO delayed_jobs (attempts, created_at, failed_at, handler, last_error, locked_at, locked_by, priority, queue, run_at, updated_at) VALUES (0, '2016-12-04 17:09:59', NULL, '--- !ruby/object:Delayed::PerformableMethod\nobject: !ruby/object:Item\n  store:\n  - a simple string\n  - an \'escaped \' string\n  - another \'escaped\' string\n  - 42\n  string: a string with many \\\\\'escapes\\\\\'\nmethod_name: :show_store\nargs: []\n', NULL, NULL, NULL, 0, NULL, '2016-12-04 17:09:59', '2016-12-04 17:09:59')`
	filters := []TokenFilter{
		&DiscardFilter{},
		&ReplaceFilter{},
	}
	consumer := NewTokenConsumer(filters)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = consumer.Process(query)
	}
}
