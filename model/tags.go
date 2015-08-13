package model

import (
	"strings"

	"github.com/DataDog/dd-go/model"
)

// Tag represents a key / value dimension on traces and stats.
type Tag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func NewTagFromString(raw string) Tag {
	name, val := model.SplitTag(raw)
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
