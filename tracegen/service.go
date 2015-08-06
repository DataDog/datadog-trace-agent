package main

// Service represents a fake service reporting traces
// It has:
//   * a number of SubServices that are reporting nested spans
//   * a generator that determines the lengths of its spans
//	 * a generator that determines the resource reported in the spans
type Service struct {
	Name          string
	SubServices   []Service
	ResourceMaker ResourceGenerator
	DurationMaker DurationGenerator
}
