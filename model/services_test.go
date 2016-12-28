package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceMetadataNotUpdated(t *testing.T) {
	// metadata should not change
	metas := make(ServicesMetadata)
	metas["web-server"] = make(map[string]string)
	metas["web-server"]["app_type"] = "web"
	metas["web-server"]["app"] = "pylons"

	metas2 := make(ServicesMetadata)
	metas2["web-server"] = make(map[string]string)
	metas2["web-server"]["app_type"] = "web"
	metas2["web-server"]["app"] = "pylons"

	assert.False(t, metas.Update(metas2))
}

func TestServiceMetadataUpdated(t *testing.T) {
	// metadata should be updated
	metas := make(ServicesMetadata)
	metas["web-server"] = make(map[string]string)
	metas["web-server"]["app_type"] = "web"
	metas["web-server"]["app"] = "pylons"

	metas2 := make(ServicesMetadata)
	metas2["web-server"] = make(map[string]string)
	metas2["web-server"]["app_type"] = "web"
	metas2["web-server"]["app"] = "rails"

	assert.True(t, metas.Update(metas2))
	assert.Equal(t, "rails", metas["web-server"]["app"])
}

func TestServiceMetadataPartial(t *testing.T) {
	// metadata should be updated
	metas := make(ServicesMetadata)
	metas["web-server"] = make(map[string]string)
	metas["web-server"]["app_type"] = "web"
	metas["web-server"]["app"] = "pylons"
	metas["postgres"] = make(map[string]string)
	metas["postgres"]["app_type"] = "db"
	metas["postgres"]["app"] = "postgres"

	metas2 := make(ServicesMetadata)
	metas2["web-server"] = make(map[string]string)
	metas2["web-server"]["app_type"] = "web"
	metas2["web-server"]["app"] = "pylons"

	assert.False(t, metas.Update(metas2))
}

func TestServiceMetadataEmpty(t *testing.T) {
	// metadata should not be updated
	metas := make(ServicesMetadata)
	metas2 := make(ServicesMetadata)

	assert.False(t, metas.Update(metas2))
}
