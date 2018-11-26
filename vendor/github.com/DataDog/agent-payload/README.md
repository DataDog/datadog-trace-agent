# agent-payload

Payload format description for communication between the Agent and the Datadog backend.

This repository includes the protocol-buffer IDL used by the agent6 to communicate with the Datadog backend.
Those payloads are only supported by the V2 API endpoints.

# Generated code

The Go, Python and Java code are already generated along the `.proto` file.

# Updates

You will need to install gogoprotobuf: github.com/gogo/protobuf

```
go get github.com/gogo/protobuf/proto
go get github.com/gogo/protobuf/protoc-gen-gofast
go get github.com/gogo/protobuf/protoc-gen-gogofast
go get github.com/gogo/protobuf/gogoproto
```

After updating the IDL you must:

- Regenerate the code: `rake codegen`
- Create a new tag with the updated version of the payload
