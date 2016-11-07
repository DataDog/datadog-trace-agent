package model

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestServiceMetadataUpdate(t *testing.T) {
	metas := make(ServicesMetadata)
	metas["web-server"] = make(map[string]string)
	metas["web-server"]["app_type"] = "web"
	metas["web-server"]["app"] = "pylons"

	// metadata unchanged
	metas2 := make(ServicesMetadata)
	metas2["web-server"] = make(map[string]string)
	metas2["web-server"]["app_type"] = "web"
	metas2["web-server"]["app"] = "pylons"

	assert.False(t, metas.Update(metas2))

	// metadata app changed
	metas3 := make(ServicesMetadata)
	metas3["web-server"] = make(map[string]string)
	metas3["web-server"]["app_type"] = "web"
	metas3["web-server"]["app"] = "rails"

	assert.True(t, metas.Update(metas3))
	assert.Equal(t, "rails", metas["web-server"]["app"])
}
