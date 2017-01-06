package model

const (
	refResource = "_resource"
)

// MetaCompact compacts tags so that if they refer to an existing field, they are not duped
func (s *Span) MetaCompact() {
	for k, v := range s.Meta {
		switch v {
		case s.Resource:
			s.Meta[k] = refResource
		}
	}
}

// MetaExpand expands tags so that they are replaced with their original values
func (s *Span) MetaExpand() {
	for k, v := range s.Meta {
		switch v {
		case refResource:
			s.Meta[k] = s.Resource
		}
	}
}
