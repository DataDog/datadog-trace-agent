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
	sb := NewStatsBucket(0, 1e9)

	assert := assert.New(t)
	span := Span{Service: "thing", Name: "other", Resource: "yo"}
	aggregators := []string{}

	aggr, tgs := getAggregateGrain(span, aggregators, &sb.keyBuf)
	assert.Equal("resource:yo,service:thing", aggr)
	assert.Equal(TagSet{Tag{"resource", "yo"}, Tag{"service", "thing"}}, tgs)

	aggregators = []string{"version"}

	span = Span{Service: "thing", Name: "other", Resource: "yo", Meta: map[string]string{"version": "1.5"}}
	aggr, tgs = getAggregateGrain(span, aggregators, &sb.keyBuf)
	assert.Equal("resource:yo,service:thing,version:1.5", aggr)
	assert.Equal(TagSet{Tag{"resource", "yo"}, Tag{"service", "thing"}, Tag{"version", "1.5"}}, tgs)

	// test something with special chars
	span = Span{Service: "thing", Name: "other:brother", Resource: "yo,mec,how goes it", Meta: map[string]string{"version": "1.5"}}
	aggr, tgs = getAggregateGrain(span, aggregators, &sb.keyBuf)
	assert.Equal("resource:yo,mec,how goes it,service:thing,version:1.5", aggr)
	assert.Equal(TagSet{Tag{"resource", "yo,mec,how goes it"}, Tag{"service", "thing"}, Tag{"version", "1.5"}}, tgs)

	// test something empty
	span = Span{TraceID: 0, SpanID: 1}
	aggr, tgs = getAggregateGrain(span, []string{}, &sb.keyBuf)
	assert.Equal("", aggr)
	assert.Equal(0, len(tgs))
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
