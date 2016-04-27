package model

// ServicesMetadata is a standard key/val meta map attached to each named service
type ServicesMetadata map[string]map[string]string

// Update just replaces the last metas stored for each service by the one given in argument
func (s1 ServicesMetadata) Update(s2 ServicesMetadata) {
	for s, metas := range s2 {
		s1[s] = metas
	}
}
