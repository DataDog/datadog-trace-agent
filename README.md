### Make it work

#### The Agent

```
# To use the ES writer, be sure ES runs on :9200 or :19200
go get github.com/olivere/elastic
go build

# Create the ES index
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

./raclette
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
```
