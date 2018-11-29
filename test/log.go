package test

import "log"

func (s *Runner) fatal(v ...interface{}) {
	if s.T != nil {
		s.T.Fatal(v...)
	} else {
		log.Fatal(v...)
	}
}

func (s *Runner) print(v ...interface{}) {
	if s.T != nil {
		s.T.Log(v...)
	} else {
		log.Println(v...)
	}
}

func (s *Runner) printf(fmt string, v ...interface{}) {
	if s.T != nil {
		s.T.Logf(fmt, v...)
	} else {
		log.Printf(fmt, v...)
	}
}

func (s *Runner) fatalf(fmt string, v ...interface{}) {
	if s.T != nil {
		s.T.Fatalf(fmt, v...)
	} else {
		log.Fatalf(fmt, v...)
	}
}
