package sampler

// SamplerState exposes all the main internal settings of the scope sampler
type SamplerState struct {
	Offset float64
	Slope  float64
	InTPS  float64
	OutTPS float64
	MaxTPS float64
}

// GetState collects and return internal statistics and coefficients for indication purposes
func (s *Sampler) GetState() SamplerState {
	return SamplerState{
		s.signatureScoreOffset,
		s.signatureScoreSlope,
		s.Backend.GetTotalScore(),
		s.Backend.GetSampledScore(),
		s.maxTPS,
	}
}
