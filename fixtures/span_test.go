package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomSpan(t *testing.T) {
	assert := assert.New(t)

	for i := 0; i < 1000; i++ {
		s := RandomSpan()
		assert.Nil(s.Normalize())
	}
}

func TestTestSpan(t *testing.T) {
	assert := assert.New(t)
	ts := TestSpan()
	assert.Nil(ts.Normalize())
}
