package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMerge(t *testing.T) {
	assert := assert.New(t)

	s1 := ServicesMetadata{
		"web-server": {"app_type": "web"},
		"memcached":  {"app_type": "cache"},
	}

	s2 := ServicesMetadata{
		"web-server": {"app_type": "custom"},
		"mysql":      {"app_type": "db"},
	}

	s1.Merge(s2)

	assert.Equal(ServicesMetadata{
		"web-server": {"app_type": "custom"},
		"memcached":  {"app_type": "cache"},
		"mysql":      {"app_type": "db"},
	}, s1)
}
