package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/olivere/elastic"
)

type Writer interface {
	Init(chan Span)
	Start()
}

type EsWriter struct {
	in chan Span
	es *elastic.Client
}

func NewEsWriter() *EsWriter {
	return &EsWriter{}
}

func (w *EsWriter) Init(in chan Span) {
	w.in = in
	client, err := elastic.NewClient(
		elastic.SetURL("http://localhost:9200", "http://localhost:19200"),
		elastic.SetSniff(false), // personal-chef ES isn't in cluster mode
	)
	if err != nil {
		log.Fatal(err)
	}

	w.es = client
}

func (w *EsWriter) Start() {
	go func() {
		for s := range w.in {
			_, err := w.es.Index().Index("raclette").Type("span").BodyJson(s).Do()
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	log.Print("EsWriter started")
}

type SqliteWriter struct {
	in chan Span
	db *sql.DB
}

func NewSqliteWriter() *SqliteWriter {
	return &SqliteWriter{}
}

func (w *SqliteWriter) Init(in chan Span) {
	w.in = in
	db, err := sql.Open("sqlite3", "./db.sqlite3")
	if err != nil {
		log.Fatal(err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS span (
		trace_id INTEGER,
		span_id INTEGER,
		parent_id INTEGER,
		start REAL,
		duration REAL,
		sample_size REAL,
		type TEXT,
		resource TEXT,
		json_meta TEXT
	)`

	_, err = db.Exec(schema)
	if err != nil {
		log.Fatal(err)
	}

	w.db = db
}

func (w *SqliteWriter) Start() {
	go func() {
		query, err := w.db.Prepare(
			`INSERT INTO span(trace_id, span_id, parent_id, start, duration, sample_size, type, resource, json_meta)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		)
		if err != nil {
			log.Fatal(err)
		}
		for s := range w.in {
			jsonMeta, _ := json.Marshal(s.Meta)

			_, err := query.Exec(
				strconv.FormatUint(uint64(s.TraceID), 10),
				strconv.FormatUint(uint64(s.SpanID), 10),
				strconv.FormatUint(uint64(s.ParentID), 10),
				strconv.FormatFloat(s.Start, 'f', 6, 64),
				strconv.FormatFloat(s.Duration, 'f', 6, 64),
				strconv.FormatUint(uint64(s.SampleSize), 10),
				s.Type,
				s.Resource,
				jsonMeta,
			)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	log.Print("SqliteWriter started")
}

type CollectorPayload struct {
	ApiKey string `json:"api_key"`
	Spans  []Span `json:"spans"`
}

type APIWriter struct {
	in         chan Span
	spanBuffer []Span
}

func NewAPIWriter() *APIWriter {
	return &APIWriter{}
}

func (w *APIWriter) Init(in chan Span) {
	w.in = in
	w.spanBuffer = []Span{}
}

func (w *APIWriter) Start() {
	go func() {
		for s := range w.in {
			w.spanBuffer = append(w.spanBuffer, s)
		}
	}()

	go w.PeriodicFlush()

	log.Print("APIWriter started")
}

func (w *APIWriter) PeriodicFlush() {
	c := time.NewTicker(3 * time.Second).C
	for _ = range c {
		w.Flush()
	}
}

func (w *APIWriter) Flush() {
	spans := w.spanBuffer
	if len(spans) == 0 {
		log.Print("Nothing to flush")
		return
	}
	w.spanBuffer = []Span{}
	log.Printf("Flush collector to the API, %d spans", len(spans))

	payload := CollectorPayload{
		ApiKey: "424242",
		Spans:  spans,
	}

	url := "http://localhost:8012/api/v0.1/collector"

	jsonStr, err := json.Marshal(payload)
	if err != nil {
		log.Fatal(err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
}
