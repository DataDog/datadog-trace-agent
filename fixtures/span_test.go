package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomSpan(t *testing.T) {
	assert := assert.New(t)

	for i := 0; i < 1000; i++ {
		s := RandomSpan()
		err := s.Normalize()
		assert.Nil(err)
	}
}

func TestTestSpan(t *testing.T) {
	assert := assert.New(t)
	ts := TestSpan()
	err := ts.Normalize()
	assert.Nil(err)
}
