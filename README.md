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
curl -XPOST localhost:9200/raclette -d '{
    "mappings" : {
        "span" : {
            "properties" : {
                "trace_id" : { "type" : "double" },
                "span_id" : { "type" : "long" },
                "parent_id" : { "type" : "long" },
                "type" : { "type" : "string" },
                "start" : { "type" : "float" },
                "end" : { "type" : "float" },
                "duration" : { "type" : "float" }
            }
        }
    }
}'
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

```
cd python
pip install -e ./
```

### Snippets

```
# Send a trace manually
curl "http://localhost:7777" -X POST -d '{"span_id": 1234, "trace_id": 46, "type": "demo", "meta": {"client":"curl", "apache.version": "2.2.2"}}'

# Send a trace with the example
python python/example/custom_script.py
```
