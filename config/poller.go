package config

import (
	"encoding/gob"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	log "github.com/cihub/seelog"
)

type Poller struct {
	interval    time.Duration
	persistPath string
	client      *http.Client
	endpoint    string
	updates     chan *ServerConfig
	apiKey      string

	index int64
}

var (
	defaultInterval = 10 * time.Second

	//TODO[aaditya]
	defaultEndpoint = "localhost:8090/config"
)

func NewDefaultConfigPoller(apiKey, persistPath string) *Poller {
	p := &Poller{
		defaultInterval, persistPath, http.DefaultClient,
		defaultEndpoint, make(chan *ServerConfig), apiKey, 0,
	}

	go p.Run()
	return p
}

func (p *Poller) Run() {
	// retrieve cached config on first boot
	conf, err := NewServerConfigFromFile(p.persistPath)
	if err != nil {
		log.Errorf("failed to read cached config at %v: %v. forcing reload...", p.persistPath, err)
		p.update()
	} else {
		p.updates <- conf
		p.index = conf.ModifyIndex
	}

	for range time.Tick(p.interval) {
		p.update()
	}
}

func (p *Poller) update() {
	req, err := http.NewRequest("GET", p.endpoint, nil)
	if err != nil {
		log.Errorf("failed to retrieve config from Datadog API: %v.", err)
		return
	}

	req.Header.Set("DD-Api-Key", p.apiKey)
	req.Header.Set("DD-Config-Modify-Index", strconv.FormatInt(p.index, 10))

	response, err := p.client.Do(req)
	defer response.Body.Close()

	if err != nil {
		// TODO[aaditya]
		tags := []string{"error:http_error", "status_code:" + strconv.Itoa(response.StatusCode)}
		reportPollingError(response, tags)
		return
	}

	tags := []string{"status_code:" + strconv.Itoa(response.StatusCode)}
	switch v := response.StatusCode; v {
	case http.StatusOK:
		reportPollingSuccess(response, tags)
	case http.StatusNotModified:
		reportPollingSuccess(response, tags)

		// not modified, exit early
		return
	default:
		if v >= 500 {
			tags := []string{"error:http_error", "status_code:" + strconv.Itoa(v)}
			reportPollingError(response, tags)
		}

	}

	var s ServerConfig
	err = json.NewDecoder(response.Body).Decode(s)
	if err != nil {
		tags := []string{"error:decoding_error"}
		reportPollingError(response, tags)
	}

	if s.ModifyIndex > p.index {
		p.updates <- &s
		p.persist(&s)
	}

	p.index = s.ModifyIndex
}

func (p *Poller) Updates() chan *ServerConfig {
	return p.updates
}

func (p *Poller) persist(conf *ServerConfig) error {
	var f *os.File
	var err error

	// TODO[aaditya] need something OS-aware here
	if p.persistPath == "" {
		f, err = ioutil.TempFile("", "trace_agent_config")
	} else {
		f, err = os.Create(p.persistPath)
	}
	defer f.Close()

	if err != nil {
		reportPersistFailed(err)
		return err
	}

	encoder := gob.NewEncoder(f)
	encoder.Encode(conf)

	return nil
}

func reportPollingError(r *http.Response, tags []string) {
	//TODO[aaditya]
	return
}

func reportPollingSuccess(r *http.Response, tags []string) {
	//TODO[aaditya]
	return
}

func reportPersistFailed(err error) {
	//TODO[aaditya]
	return
}
