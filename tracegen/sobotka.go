package main

func NewFakeDogpoundGet() Service {
	return Service{
		Name: "dogpound.redis",
		ResourceMaker: func() string {
			return "mget"
		},
		DurationMaker: func() float64 {
			return GaussianDuration(0.001, 0.01, 0, 0)
		},
	}
}

func NewFakeDogpoundSet() Service {
	return Service{
		Name: "dogpound.redis",
		ResourceMaker: func() string {
			return "mset"
		},
		DurationMaker: func() float64 {
			return GaussianDuration(0.00001, 0.01, 0, 0)
		},
	}
}

func NewFakeSobotkaNormalize() Service {
	subs := []Service{NewFakeDogpoundGet(), NewFakeDogpoundSet(), NewFakeDogpoundSet()}
	return Service{
		Name: "sobotka.transform",
		ResourceMaker: func() string {
			return "normalize"
		},
		DurationMaker: func() float64 {
			return GaussianDuration(0.03, 0.1, 0.01, 0)
		},
		SubServices: subs,
	}
}

func NewFakeSobotkaTransform() Service {
	qs := []string{"agent_api", "metric_api"}
	return Service{
		Name: "sobotka.transform",
		ResourceMaker: func() string {
			return ChooseRandomString(qs)
		},
		DurationMaker: func() float64 {
			return GaussianDuration(0.05, 0.01, 0, 0)
		},
	}
}

func NewFakeSobotka() Service {
	qs := []string{"metric_api", "metrics", "check_runs"}
	subs := []Service{NewFakeSobotkaTransform(), NewFakeSobotkaNormalize()}
	return Service{
		Name: "sobotka.process",
		ResourceMaker: func() string {
			return ChooseRandomString(qs)
		},
		DurationMaker: func() float64 {
			return GaussianDuration(0.30, 0.1, 0.1, 0)
		},
		SubServices: subs,
	}
}
