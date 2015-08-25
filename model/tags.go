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

// TagSet is an ordered and unique combination of tags
type TagSet []Tag

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

// NewTagsFromString returns a new TagSet from a raw string
func NewTagsFromString(raw string) TagSet {
	var tags TagSet
	for _, t := range strings.Split(raw, ",") {
		tags = append(tags, NewTagFromString(t))
	}
	return tags
}

// String returns a string representation of a tag
func (t Tag) String() string {
	return t.Name + ":" + t.Value
}

// TagKey returns a unique key from the string given and the tagset, useful to index stuff on tagsets
func (s TagSet) TagKey(m string) string {
	tagStrings := make([]string, len(s))
	for i, t := range s {
		tagStrings[i] = t.String()
	}
	sort.Strings(tagStrings)
	return fmt.Sprintf("%s|%s", m, strings.Join(tagStrings, ","))
}
