package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"strconv"

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
		end REAL,
		duration REAL,
		type TEXT,
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
			`INSERT INTO span(trace_id, span_id, parent_id, start, end, duration, type, json_meta)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
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
				strconv.FormatFloat(s.End, 'f', 6, 64),
				strconv.FormatFloat(s.Duration, 'f', 6, 64),
				s.Type,
				jsonMeta,
			)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	log.Print("SqliteWriter started")
}
