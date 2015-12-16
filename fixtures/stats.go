package fixtures

import "github.com/DataDog/raclette/model"

var defaultAggregators = []string{"layer", "resource"}

// RandomStatsBucket returns a bucket made from n random spans, useful to run benchmarks and tests
func RandomStatsBucket(n int) model.StatsBucket {
	sb := model.NewStatsBucket(0, 1e9)
	for i := 0; i < n; i++ {
		sb.HandleSpan(RandomSpan(), defaultAggregators)
	}
	return sb
}

// TestStatsBucket returns a fixed stats bucket to be used in unit tests
func TestStatsBucket() model.StatsBucket {
	sb := model.NewStatsBucket(0, 1e9)
	sb.HandleSpan(TestSpan(), defaultAggregators)
	return sb
}

func StatsBucketWithSpans(s []model.Span) model.StatsBucket {
	sb := model.NewStatsBucket(0, 1e9)
	for _, s := range s {
		sb.HandleSpan(s, defaultAggregators)
	}
	return sb
}
