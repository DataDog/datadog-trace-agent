package model

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroup(t *testing.T) {
	cases := map[string]string{
		"a:1":   "a",
		"a":     "",
		"a:1:1": "a",
		"abc:2": "abc",
	}

	assert := assert.New(t)
	for in, out := range cases {
		actual := TagGroup(in)
		assert.Equal(out, actual)
	}
}

func TestSort(t *testing.T) {
	t1 := NewTagSetFromString("a:2,a:1,a:3")
	t2 := NewTagSetFromString("a:1,a:2,a:3")
	sort.Sort(t1)
	assert.Equal(t, t1, t2)

	// trick: service<name but mcnulty<query (traps a bug if we consider
	// that "if not name1 < name2 then compare value1 and value2")
	t1 = NewTagSetFromString("mymetadata:cool,service:mcnulty,name:query")
	t2 = NewTagSetFromString("mymetadata:cool,name:query,service:mcnulty")
	sort.Sort(t1)
	sort.Sort(t2)
	assert.Equal(t, t1, t2)
}

func TestTagMerge(t *testing.T) {
	t1 := NewTagSetFromString("a:1,a:2")
	t2 := NewTagSetFromString("a:2,a:3")
	t3 := MergeTagSets(t1, t2)
	assert.Equal(t, t3, NewTagSetFromString("a:1,a:2,a:3"))

	t1 = NewTagSetFromString("a:1")
	t2 = NewTagSetFromString("a:2")
	t3 = MergeTagSets(t1, t2)
	assert.Equal(t, t3, NewTagSetFromString("a:1,a:2"))

	t1 = NewTagSetFromString("a:2,a:1")
	t2 = NewTagSetFromString("a:6,a:2")
	t3 = MergeTagSets(t1, t2)
	assert.Equal(t, t3, NewTagSetFromString("a:1,a:2,a:6"))

}

func TestSplitTag(t *testing.T) {
	k, v := SplitTag("k:v:w")
	assert.Equal(t, k, "k")
	assert.Equal(t, v, "v:w")
}

func TestTagColon(t *testing.T) {
	ts := NewTagSetFromString("a:1:2:3,url:http://localhost:1234/")
	t.Logf("ts: %v", ts)
	assert.Equal(t, "1:2:3", ts.Get("a").Value)
	assert.Equal(t, "http://localhost:1234/", ts.Get("url").Value)
}

func TestFilterTags(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		tags, groups, out []string
	}{
		{
			tags:   []string{"a:1", "a:2", "b:1", "c:2"},
			groups: []string{"a", "b"},
			out:    []string{"a:1", "a:2", "b:1"},
		},
		{
			tags:   []string{"a:1", "a:2", "b:1", "c:2"},
			groups: []string{"b"},
			out:    []string{"b:1"},
		},
		{
			tags:   []string{"a:1", "a:2", "b:1", "c:2"},
			groups: []string{"d"},
			out:    nil,
		},
		{
			tags:   nil,
			groups: []string{"d"},
			out:    nil,
		},
	}

	for _, c := range cases {
		out := FilterTags(c.tags, c.groups)
		assert.Equal(out, c.out)
	}

}
func TestUnset(t *testing.T) {
	assert := assert.New(t)
	ts := NewTagSetFromString("service:mcnulty,resource:template,name:magicfunc")
	ts2 := ts.Unset("name")
	assert.Len(ts, 3)
	assert.Equal("mcnulty", ts.Get("service").Value)
	assert.Equal("template", ts.Get("resource").Value)
	assert.Equal("magicfunc", ts.Get("name").Value)
	assert.Len(ts2, 2)
	assert.Equal("mcnulty", ts2.Get("service").Value)
	assert.Equal("template", ts2.Get("resource").Value)
	assert.Equal("", ts2.Get("name").Value)
}
