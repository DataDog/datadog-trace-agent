package test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

// RunAgent runs the agent using a given yaml config. If an agent is already running,
// it will be killed.
func (s *Runner) RunAgent(conf []byte) error {
	if atomic.LoadUint64(&s.started) == 0 {
		return errors.New("runner: server not started (call Start first)")
	}
	cfgPath, err := s.createConfigFile(conf)
	if err != nil {
		log.Fatalf("runner: error creating config: %v", err)
	}
	exit := s.runAgentConf(cfgPath)
	for {
		select {
		case err := <-exit:
			return fmt.Errorf("runner: got %q, output was:\n----\n%s----\n", err, s.log.String())
		default:
			if strings.Contains(s.log.String(), "listening for traces at") {
				log.Println("runner: agent started")
				return nil
			}
		}
	}
}

// Logtail returns up to 1MB of tail from the running trace-agent log.
func (s *Runner) Logtail() string { return s.log.String() }

// StopAgent stops a running trace-agent.
func (s *Runner) StopAgent() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.pid == 0 {
		return
	}
	proc, err := os.FindProcess(s.pid)
	if err != nil {
		return
	}
	if err := proc.Kill(); err != nil {
		log.Println("couldn't kill running agent: ", err)
	}
	proc.Wait()
	s.log.Reset()
}

func (s *Runner) runAgentConf(path string) <-chan error {
	s.StopAgent()
	cmd := exec.Command("trace-agent", "-config", path)
	s.log.Reset()
	cmd.Stdout = s.log
	cmd.Stderr = ioutil.Discard
	cmd.Start()

	s.mu.Lock()
	s.pid = cmd.Process.Pid
	s.mu.Unlock()

	// We use a buffered channel because we only try receiving on it while
	// waiting for a start and shouldn't block afterwards; if it fails, it's
	// fine because posting to the agent will also fail.
	ch := make(chan error, 1)
	go func() {
		ch <- cmd.Wait()
		s.mu.Lock()
		s.pid = 0
		s.mu.Unlock()
		log.Println("runner: agent stopped")
	}()
	return ch
}

// createConfigFile creates a config file from the given config, altering the
// apm_config.apm_dd_url and log_level values and returns the full path.
func (s *Runner) createConfigFile(conf []byte) (string, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader(conf)); err != nil {
		return "", err
	}
	s.agentPort = 8126
	if v.IsSet("apm_config.receiver_port") {
		s.agentPort = v.GetInt("apm_config.receiver_port")
	}
	v.Set("apm_config.apm_dd_url", "http://"+s.srv.Addr)
	if !v.IsSet("apm_config.trace_writer.flush_period_seconds") {
		v.Set("apm_config.trace_writer.flush_period_seconds", 1)
	}
	v.Set("log_level", "info")
	out, err := yaml.Marshal(v.AllSettings())
	if err != nil {
		return "", err
	}
	dir, err := ioutil.TempDir("", "agent-conf-")
	if err != nil {
		return "", err
	}
	f, err := os.Create(filepath.Join(dir, "datadog.yaml"))
	if err != nil {
		return "", err
	}
	if _, err := f.Write(out); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return f.Name(), nil
}
