package model

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const maxTagLength = 200

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
	if IsTraceSpecific(name) {
		name = WithTracePrefix(name)
	}
	return Tag{name, val}
}

// TagSet is an ordered and unique combination of tags
type TagSet []Tag

// NewTagSetFromString returns a new TagSet from a raw string
func NewTagSetFromString(raw string) TagSet {
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
func (t TagSet) Less(i, j int) bool { return t[i].Name < t[j].Name || t[i].Value < t[j].Value }

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

// GetWithTracePrefix gets the tag with the name prefixed for Trace domain.
// For transition sake, if the prefixed tag is not found, it will query
// the original, raw value. This feature could be removed after a while.
func (t TagSet) GetWithTracePrefix(name string) Tag {
	with := WithTracePrefix(name)
	for _, tag := range t {
		if tag.Name == with {
			return tag
		}
	}
	without := WithoutTracePrefix(name)
	for _, tag := range t {
		if tag.Name == without {
			tag.Name = with // modifying sur that it reflects the real, prefixed value
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

// MatchFilters returns a tag set of the tags that match certain filters.
// A filter is defined as : "KEY:VAL" where:
//  * KEY is a non-empty string
//  * VALUE is a string (can be empty)
// A tag {Name:k, Value:v} from the input tag set will match if:
//  * KEY==k and VALUE is non-empty and v==VALUE
//  * KEY==k and VALUE is empty (don't care about v)
func (t TagSet) MatchFilters(filters []string) TagSet {
	// FIXME: ugly ?
	filterMap := make(map[string]map[string]struct{})

	for _, f := range filters {
		g, v := SplitTag(f)
		m, ok := filterMap[g]
		if !ok {
			m = make(map[string]struct{})
			filterMap[g] = m
		}

		if v != "" {
			filterMap[g][v] = struct{}{}
		}
	}

	matchedFilters := TagSet{}

	for _, tag := range t {
		vals, ok := filterMap[tag.Name]
		if ok {
			if len(vals) == 0 {
				matchedFilters = append(matchedFilters, tag)
			} else {
				_, ok := vals[tag.Value]
				if ok {
					matchedFilters = append(matchedFilters, tag)
				}
			}
		}
	}
	return matchedFilters
}

// MergeTagSets merge two tag sets lazily
func MergeTagSets(t1, t2 TagSet) TagSet {
	if t1 == nil {
		return t2
	}
	if t2 == nil {
		return t1
	}
	t := append(t1, t2...)

	if len(t) < 2 {
		return t
	}

	// sorting is actually expensive so skip it if we can
	if !sort.IsSorted(t) {
		sort.Sort(t)
	}

	last := t[0]
	idx := 1
	for i := 1; i < len(t); i++ {
		if t[i].Name != last.Name || t[i].Value != last.Value {
			last = t[i]
			t[idx] = last
			idx++

		}
	}
	return t[:idx]
}

// TagGroup will return the tag group from the given string. For example,
// "host:abc" => "host"
func TagGroup(tag string) string {
	for i, c := range tag {
		if c == ':' {
			return tag[0:i]
		}
	}
	return ""
}

// FilterTags will return the tags that have the given group.
func FilterTags(tags, groups []string) []string {
	var out []string
	for _, t := range tags {
		tg := TagGroup(t)
		for _, g := range groups {
			if g == tg {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

// Taken from dd-go.model.NormalizeTag
func normalizeTagContent(tag string) string {
	// unless you just throw out unicode, this is already as fast as it gets
	var buf bytes.Buffer

	lastWasUnderscore := false

	for _, c := range tag {
		// fast path for len check
		if buf.Len() >= maxTagLength {
			break
		}
		// fast path for ascii alphabetic chars
		switch {
		case c >= 'a' && c <= 'z':
			buf.WriteRune(c)
			lastWasUnderscore = false
			continue
		case c >= 'A' && c <= 'Z':
			c -= 'A' - 'a'
			buf.WriteRune(c)
			lastWasUnderscore = false
			continue
		}

		c = unicode.ToLower(c)
		switch {
		// handle always valid cases
		case unicode.IsLetter(c) || c == ':':
			buf.WriteRune(c)
			lastWasUnderscore = false
		// skip any characters that can't start the string
		case buf.Len() == 0:
			continue
		// handle valid characters that can't start the string.
		case unicode.IsDigit(c) || c == '.' || c == '/' || c == '-':
			buf.WriteRune(c)
			lastWasUnderscore = false
		// convert anything else to underscores (including underscores), but only allow one in a row.
		case !lastWasUnderscore:
			buf.WriteRune('_')
			lastWasUnderscore = true
		}
	}

	b := buf.Bytes()

	// strip trailing underscores
	if lastWasUnderscore {
		return string(b[:len(b)-1])
	}

	return string(b)
}

// Avoids collision with existing tags
func normalizeTagPrefix(tag string) string {
	e := strings.Split(tag, ":")
	if IsTraceSpecific(e[0]) {
		e[0] = WithTracePrefix(e[0])
	}
	return strings.Join(e, ":")
}

// NormalizeTag applies some normalization to ensure the tags match the
// backend requirements. Additionnally, it rewrites some entries which
// are generated by trace code (name, resource, service).
func NormalizeTag(tag string) string {
	return normalizeTagPrefix(normalizeTagContent(tag))
}
