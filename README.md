### Make it work

#### The Agent

```
# Go dependencies
go get github.com/olivere/elastic
go get github.com/mattn/go-sqlite3

# Build the collector
go build

# Run it
./raclette
```

```
# If you want to use the ES writer, first create the ES index
curl -X PUT 'http://localhost:9200/raclette' -d @es_settings.json

# Then run with the proper option
./raclette -writers=es,sqlite
```


### The Agent UI

Simple web UI to see traces and spans. Spans need to be written in the SQLite DB.

```
pip install flask

python collector_web/server.py
```

#### The Python lib

Checkout `dogweb:dogtrace` to have access to the `dogtrace` library.

### Snippets

```
# Send a trace manually
curl "http://localhost:7777" -X POST -d '{"span_id": 1234, "trace_id": 46, "type": "demo", "meta": {"client":"curl", "apache.version": "2.2.2"}}'
```
