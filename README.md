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
./raclette -writer=es
```

#### The Python lib

```
cd python
pip install -e ./
```

### Snippets

```
# Send a trace manually
curl "http://localhost:7777" -X POST -d '{"SpanID": 1234, "traceid": 46, "Type": "demo", "meta": {"client":"curl", "apache.version": "2.2.2"}}'

# Send a trace with the example
python python/example/custom_script.py
```

