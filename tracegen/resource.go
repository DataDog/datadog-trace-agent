package main

import "math/rand"

// ResourceGenerator is a function that returns a span resource
type ResourceGenerator func() string

// ChooseRandomString is a ResourceGenerator that returns a random choice of a given slice of possible resources
func ChooseRandomString(choices []string) string {
	idx := rand.Intn(len(choices))
	return choices[idx]
}
