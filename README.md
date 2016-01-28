### Make it work

#### The Agent

```
# Run verifications & build the binaries
rake

# Run it
./raclette

# Run the trace generator
./generator
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
curl "http://localhost:7777/span" -X POST -d '{"span_id": 1234, "trace_id": 46, "type": "demo", "meta": {"client":"curl", "apache.version": "2.2.2"}}'
```
