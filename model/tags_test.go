package model

import (
	"sort"
	"testing"
	"time"

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

func TestAggrString(t *testing.T) {
	sb := NewStatsBucket(0, 1e9, time.Millisecond)

	assert := assert.New(t)
	span := Span{Service: "thing", Name: "other", Resource: "yo"}
	aggregators := []string{}

	res := getAggregateString(span, aggregators, sb.keyBuf)
	assert.Equal(res, "name:other,resource:yo,service:thing")

	aggregators = []string{"version"}

	span = Span{Service: "thing", Name: "other", Resource: "yo", Meta: map[string]string{"version": "1.5"}}
	res = getAggregateString(span, aggregators, sb.keyBuf)
	assert.Equal(res, "name:other,resource:yo,service:thing,version:1.5")

	span = Span{TraceID: 0, SpanID: 1}
	assert.Equal("", getAggregateString(span, []string{}, sb.keyBuf))
}
