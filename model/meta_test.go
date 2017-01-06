package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpanMetaCompactDiff(t *testing.T) {
	assert := assert.New(t)
	ts1 := testSpan()
	ts2 := testSpan()
	ts1.MetaCompact()
	assert.Equal(ts1, ts2, "compact modified data which should have remained untouched")
	ts1.MetaExpand()
	assert.Equal(ts1, ts2, "compact + expand does not restitute original data")
}

func TestSpanMetaCompactMatch(t *testing.T) {
	assert := assert.New(t)
	ts1 := testSpan()
	ts1.Meta["foo"] = ts1.Resource
	ts2 := testSpan()
	ts2.Meta["foo"] = ts2.Resource
	ts1.MetaCompact()
	assert.NotEqual(ts1, ts2, "compact did nothing")
	assert.Equal("_resource", ts1.Meta["foo"], "compact did not set resource to the right ref")
	ts1.MetaCompact()
	assert.Equal("_resource", ts1.Meta["foo"], "compact is not idempotent")
	ts1.MetaExpand()
	assert.Equal(ts1, ts2, "compact + expand does not restitute original data")
	ts1.MetaExpand()
	assert.Equal(ts1, ts2, "expand is not idempotent")
}
