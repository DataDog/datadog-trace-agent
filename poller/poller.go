package poller

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/DataDog/datadog-trace-agent/statsd"
	log "github.com/cihub/seelog"
)

// Poller polls Datadog for configuration updates
type Poller struct {
	interval    time.Duration
	persistPath string
	client      *http.Client
	endpoint    string
	updates     chan *config.ServerConfig
	apiKey      string

	index int64
}

var (
	defaultInterval = 10 * time.Second

	//TODO[aaditya]
	defaultEndpoint = "localhost:8090/config"
)

type pollingError struct {
	error

	tags []string
}

// NewDefaultConfigPoller initializes a new Poller with sane defaults
func NewDefaultConfigPoller(apiKey, persistPath string) *Poller {
	p := &Poller{
		defaultInterval, persistPath, http.DefaultClient,
		defaultEndpoint, make(chan *config.ServerConfig), apiKey, 0,
	}

	go p.run()
	return p
}

func (p *Poller) run() {
	// retrieve cached config on first boot
	conf, err := config.NewServerConfigFromFile(p.persistPath)
	if err != nil {
		log.Errorf("failed to read cached config at %v: %v. forcing reload...", p.persistPath, err)
		err = p.update()
		if err != nil {
			reportPollingError(&pollingError{err, []string{}})
		}
	} else {
		p.updates <- conf
		p.index = conf.ModifyIndex
	}

	for range time.Tick(p.interval) {
		err = p.update()
		if err != nil {
			reportPollingError(&pollingError{err, []string{}})
		}
	}
}

func (p *Poller) update() (err error) {
	req, err := http.NewRequest("GET", p.endpoint, nil)
	if err != nil {
		log.Errorf("failed to retrieve config from Datadog API: %v.", err)
		return
	}

	req.Header.Set("DD-Api-Key", p.apiKey)
	req.Header.Set("DD-Config-Modify-Index", strconv.FormatInt(p.index, 10))

	response, err := p.client.Do(req)
	if err != nil {
		tags := []string{"error:http_error"}
		return pollingError{err, tags}
	}

	defer response.Body.Close()

	tags := []string{"status_code:" + strconv.Itoa(response.StatusCode)}
	switch response.StatusCode {
	case http.StatusOK:
		reportPollingSuccess(tags)
	case http.StatusNotModified:
		reportPollingSuccess(tags)

		// not modified, exit early
		return
	default:
		if response.StatusCode >= 500 {
			tags := append(tags, "error:http_error")
			return pollingError{errors.New("config server error"), tags}
		}
	}

	var s config.ServerConfig
	err = json.NewDecoder(response.Body).Decode(&s)
	if err != nil {
		tags := []string{"error:decoding_error"}
		return pollingError{err, tags}
	}

	if s.ModifyIndex > p.index {
		p.updates <- &s
		err = p.persist(&s)
		if err != nil {
			log.Errorf("failed to persist config from server: %v", err)
			reportPersistFailed(err, []string{})
		}
	}

	p.index = s.ModifyIndex
	return nil
}

// Updates returns the channel to subscribe to for configuration updates
func (p *Poller) Updates() <-chan *config.ServerConfig {
	return p.updates
}

func (p *Poller) persist(conf *config.ServerConfig) error {
	if p.persistPath == "" {
		//persistence disabled
		log.Info("persist path unspecified: config not saved")
		return nil
	}

	var f *os.File
	var err error

	f, err = os.Create(p.persistPath)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := gob.NewEncoder(f)
	encoder.Encode(conf)

	return nil
}

func reportPollingError(err *pollingError) {
	log.Errorf("failed to retrieve config from server: %v", err)
	statsd.Client.Count("datadog.trace_agent.poller.poll.failed", 1, err.tags, 1.0)
}

func reportPollingSuccess(tags []string) {
	statsd.Client.Count("datadog.trace_agent.poller.poll.succeeded", 1, tags, 1.0)
	return
}

func reportPersistFailed(err error, tags []string) {
	log.Errorf("failed to persist config from server: %v", err)
	statsd.Client.Count("datadog.trace_agent.poller.persist.failed", 1, tags, 1.0)
	return
}
