package model

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSort(t *testing.T) {
	t1 := NewTagsFromString("a:2,a:1,a:3")
	t2 := NewTagsFromString("a:1,a:2,a:3")
	sort.Sort(t1)
	assert.Equal(t, t1, t2)
}

func TestTagMerge(t *testing.T) {
	t1 := NewTagsFromString("a:1,a:2")
	t2 := NewTagsFromString("a:2,a:3")
	t3 := MergeTagSets(t1, t2)
	assert.Equal(t, t3, NewTagsFromString("a:1,a:2,a:3"))

	t1 = NewTagsFromString("a:1")
	t2 = NewTagsFromString("a:2")
	t3 = MergeTagSets(t1, t2)
	assert.Equal(t, t3, NewTagsFromString("a:1,a:2"))

	t1 = NewTagsFromString("a:2,a:1")
	t2 = NewTagsFromString("a:6,a:2")
	t3 = MergeTagSets(t1, t2)
	assert.Equal(t, t3, NewTagsFromString("a:1,a:2,a:6"))

}
