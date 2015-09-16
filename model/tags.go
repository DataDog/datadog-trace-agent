package model

import (
	"fmt"
	"sort"
	"strings"
)

// Tag represents a key / value dimension on traces and stats.
type Tag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// String returns a string representation of a tag
func (t Tag) String() string {
	return t.Name + ":" + t.Value
}

// SplitTag splits the tag into group and value. If it doesn't have a seperator
// the empty string will be used for the group.
func SplitTag(tag string) (group, value string) {
	split := strings.SplitN(tag, ":", 2)
	if len(split) == 1 {
		return "", split[0]
	}
	return split[0], split[1]
}

// NewTagFromString returns a new Tag from a raw string
func NewTagFromString(raw string) Tag {
	name, val := SplitTag(raw)
	return Tag{name, val}
}

// TagSet is an ordered and unique combination of tags
type TagSet []Tag

// NewTagsFromString returns a new TagSet from a raw string
func NewTagsFromString(raw string) TagSet {
	var tags TagSet
	for _, t := range strings.Split(raw, ",") {
		tags = append(tags, NewTagFromString(t))
	}
	return tags
}

// TagKey returns a unique key from the string given and the tagset, useful to index stuff on tagsets
func (t TagSet) TagKey(m string) string {
	tagStrings := make([]string, len(t))
	for i, tag := range t {
		tagStrings[i] = tag.String()
	}
	sort.Strings(tagStrings)
	return fmt.Sprintf("%s|%s", m, strings.Join(tagStrings, ","))
}

func (t TagSet) Len() int           { return len(t) }
func (t TagSet) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t TagSet) Less(i, j int) bool { return t[i].Name < t[j].Name && t[i].Value < t[j].Value }

// Key returns a string representing a new set of tags.
func (t TagSet) Key() string {
	s := make([]string, len(t))
	for i, t := range t {
		s[i] = t.String()
	}
	sort.Strings(s)
	return strings.Join(s, ",")
}

// Get the tag with the particular name
func (t TagSet) Get(name string) Tag {
	for _, tag := range t {
		if tag.Name == name {
			return tag
		}
	}
	return Tag{}
}

// Match returns a new tag set with only the tags matching the given groups.
func (t TagSet) Match(groups []string) TagSet {
	if len(groups) == 0 {
		return nil
	}
	var match []Tag
	for _, g := range groups {
		tag := t.Get(g)
		if tag.Value == "" {
			continue
		}
		match = append(match, tag)
	}
	ts := TagSet(match)
	sort.Sort(ts)
	return ts
}

// HasExactly returns true if we have tags only for the given groups.
func (t TagSet) HasExactly(groups []string) bool {
	if len(groups) != len(t) {
		return false
	}
	// FIXME quadratic
	for _, g := range groups {
		if t.Get(g).Name == "" {
			return false
		}
	}
	return true
}
