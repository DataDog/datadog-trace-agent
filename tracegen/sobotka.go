package main

// this file implements some "fake" services that are used in dogweb
// composing them will enable you to generate traces that look like the
// ones from different applications of dogweb (web, sobotka, ...)

func newFakeDogpoundGet() Service {
	return Service{
		Name: "dogpound.redis",
		ResourceMaker: func() string {
			return "mget"
		},
		DurationMaker: func() int64 {
			return GaussianDuration(0.001, 0.01, 0, 0)
		},
	}
}

func newFakeDogpoundSet() Service {
	return Service{
		Name: "dogpound.redis",
		ResourceMaker: func() string {
			return "mset"
		},
		DurationMaker: func() int64 {
			return GaussianDuration(0.00001, 0.01, 0, 0)
		},
	}
}

func newFakeSobotkaNormalize() Service {
	subs := []Service{newFakeDogpoundGet(), newFakeDogpoundSet(), newFakeDogpoundSet()}
	return Service{
		Name: "sobotka.transform",
		ResourceMaker: func() string {
			return "normalize"
		},
		DurationMaker: func() int64 {
			return GaussianDuration(0.03, 0.1, 1e7, 0)
		},
		SubServices: subs,
	}
}

func newFakeSobotkaTransform() Service {
	qs := []string{"agent_api", "metric_api"}
	return Service{
		Name: "sobotka.transform",
		ResourceMaker: func() string {
			return ChooseRandomString(qs)
		},
		DurationMaker: func() int64 {
			return GaussianDuration(0.05, 0.01, 0, 0)
		},
	}
}

func newFakeSobotka() Service {
	qs := []string{"metric_api", "metrics", "check_runs"}
	subs := []Service{newFakeSobotkaTransform(), newFakeSobotkaNormalize()}
	return Service{
		Name: "sobotka.process",
		ResourceMaker: func() string {
			return ChooseRandomString(qs)
		},
		DurationMaker: func() int64 {
			return GaussianDuration(0.30, 0.1, 1e8, 0)
		},
		SubServices: subs,
	}
}
