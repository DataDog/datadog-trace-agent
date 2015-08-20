package model

import "strings"

// Tag represents a key / value dimension on traces and stats.
type Tag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
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

func NewTagFromString(raw string) Tag {
	name, val := SplitTag(raw)
	return Tag{name, val}
}

func NewTagsFromString(raw string) []Tag {
	var tags []Tag
	for _, t := range strings.Split(raw, ",") {
		tags = append(tags, NewTagFromString(t))
	}
	return tags
}

func (t Tag) String() string {
	return t.Name + ":" + t.Value
}
