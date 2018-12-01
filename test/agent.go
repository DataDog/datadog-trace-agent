package test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

type agentRunner struct {
	mu  sync.RWMutex // guards pid
	pid int          // agent pid, if running

	port    int         // agent port
	log     *safeBuffer // agent log
	ddURL   string      // backend address
	verbose bool
}

func newAgentRunner(ddURL string, verbose bool) *agentRunner {
	return &agentRunner{
		ddURL:   ddURL,
		log:     newSafeBuffer(),
		verbose: verbose,
	}
}

// Run runs the agent using a given yaml config. If an agent is already running,
// it will be killed.
func (s *agentRunner) Run(conf []byte) error {
	if _, err := exec.LookPath("trace-agent"); err != nil {
		return err
	}
	cfgPath, err := s.createConfigFile(conf)
	if err != nil {
		return fmt.Errorf("runner: error creating config: %v", err)
	}
	exit := s.runAgentConfig(cfgPath)
	for {
		select {
		case err := <-exit:
			return fmt.Errorf("runner: got %q, output was:\n----\n%s----", err, s.log.String())
		default:
			if strings.Contains(s.log.String(), "listening for traces at") {
				if s.verbose {
					log.Print("runner: agent started")
				}
				return nil
			}
		}
	}
}

// Log returns the tail of the agent log (up to 1M).
func (s *agentRunner) Log() string { return s.log.String() }

// PID returns the process ID of the trace-agent. If the trace-agent is not running
// as a child process of this program, it will be 0.
func (s *agentRunner) PID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pid
}

// Addr returns the address of the trace agent receiver.
func (s *agentRunner) Addr() string { return fmt.Sprintf("localhost:%d", s.port) }

// Kill stops a running trace-agent, if it was started by this process.
func (s *agentRunner) Kill() {
	pid := s.PID()
	if pid == 0 {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	if err := proc.Kill(); err != nil {
		if s.verbose {
			log.Print("couldn't kill running agent: ", err)
		}
	}
	proc.Wait()
}

func (s *agentRunner) runAgentConfig(path string) <-chan error {
	s.Kill()
	cmd := exec.Command("trace-agent", "-config", path)
	s.log.Reset()
	cmd.Stdout = s.log
	cmd.Stderr = ioutil.Discard
	cmd.Start()

	s.mu.Lock()
	s.pid = cmd.Process.Pid
	s.mu.Unlock()

	ch := make(chan error, 1) // don't block
	go func() {
		ch <- cmd.Wait()
		s.mu.Lock()
		s.pid = 0
		s.mu.Unlock()
		if s.verbose {
			log.Print("runner: agent stopped")
		}
	}()
	return ch
}

// createConfigFile creates a config file from the given config, altering the
// apm_config.apm_dd_url and log_level values and returns the full path.
func (s *agentRunner) createConfigFile(conf []byte) (string, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader(conf)); err != nil {
		return "", err
	}
	s.port = 8126
	if v.IsSet("apm_config.receiver_port") {
		s.port = v.GetInt("apm_config.receiver_port")
	}
	v.Set("apm_config.apm_dd_url", "http://"+s.ddURL)
	if !v.IsSet("api_key") {
		v.Set("api_key", "testing123")
	}
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
