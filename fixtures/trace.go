package fixtures

import (
	"math/rand"

	"github.com/DataDog/datadog-trace-agent/model"
)

/* tree structure
   from 1 to 10 levels

   always one root
   each level has at most 100 spans
*/

func genNextLevel(prevLevel []model.Span) []model.Span {
	var spans []model.Span
	numSpans := rand.Intn(100) + 1

	// the spans have to be "nested" in the previous level
	// choose randomly spans from prev level
	chosenSpans := rand.Perm(len(prevLevel))
	// cap to a random number > 1
	maxParentSpans := rand.Intn(len(prevLevel))
	if maxParentSpans == 0 {
		maxParentSpans = 1
	}
	chosenSpans = chosenSpans[:maxParentSpans]

	// now choose a random amount of spans per chosen span
	// total needs to be numSpans
	for i, prevIdx := range chosenSpans {
		prev := prevLevel[prevIdx]

		var childSpans int
		value := numSpans - (len(chosenSpans) - i)
		if i == len(chosenSpans)-1 || value < 1 {
			childSpans = numSpans
		} else {
			childSpans = rand.Intn(value)
		}
		numSpans -= childSpans

		timeLeft := prev.Duration

		// create the spans
		curSpans := make([]model.Span, 0, childSpans)
		for j := 0; j < childSpans && timeLeft > 0; j++ {
			news := RandomSpan()
			news.TraceID = prev.TraceID
			news.ParentID = prev.SpanID

			// distribute durations in prev span
			// random start
			randStart := rand.Int63n(timeLeft)
			news.Start = prev.Start + randStart
			// random duration
			timeLeft -= randStart
			news.Duration = rand.Int63n(timeLeft)
			timeLeft -= news.Duration

			curSpans = append(curSpans, news)
		}

		spans = append(spans, curSpans...)
	}

	return spans
}

func RandomTrace() model.Trace {
	t := model.Trace{RandomSpan()}

	prevLevel := t
	maxDepth := rand.Intn(10)

	for i := 0; i < maxDepth; i++ {
		if len(prevLevel) > 0 {
			prevLevel = genNextLevel(prevLevel)
			t = append(t, prevLevel...)
		}
	}

	return t
}

func SingleSpanTrace() model.Trace {
	var t model.Trace
	t = append(t, RandomSpan())
	return t
}

func FuzzTrace() model.Trace {
	return model.Trace{}
}
