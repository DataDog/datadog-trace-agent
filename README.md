### Make it work

```
# To use the ES writer, be sure ES runs on :9200 or :19200
go get github.com/olivere/elastic
go build
./raclette
```

### Snippets

```
# Send a trace manually
curl "http://localhost:7777" -X POST -d '{"SpanID": 1234, "traceid": 46, "Type": "demo", "meta": {"client":"curl", "apache.version": "2.2.2"}}'
```
