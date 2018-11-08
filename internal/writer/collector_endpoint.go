package writer

import (
	"context"
	"fmt"
	"log"
	"reflect"

	grpc "google.golang.org/grpc"
)

// CollectorEndpoint sends payloads to a Collector.
type CollectorEndpoint struct {
	conn *grpc.ClientConn
}

// NewCollectorEndpoint returns an initialized CollectorEndpoint, from a provided serverAddr
func NewCollectorEndpoint(serverAddr string) *CollectorEndpoint {
	var conn *grpc.ClientConn
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure(), grpc.WithCodec(rawProtoCodec{}))
	if err != nil {
		log.Fatalf("Unable to create grpc connection to collector: %s", err)
	}
	return &CollectorEndpoint{
		conn: conn,
	}
}

// write will send the serialized traces payload to the Datadog traces endpoint.
func (e *CollectorEndpoint) write(payload *payload) error {
	_, err := e.sendTraces(context.Background(), payload.bytes)
	return err
}

func (e *CollectorEndpoint) baseURL() string {
	return "TODO"
}

func (e *CollectorEndpoint) sendTraces(ctx context.Context, in []byte, opts ...grpc.CallOption) ([]byte, error) {
	var out []byte
	err := grpc.Invoke(ctx, "/receiver.CollectorService/SendTraces", in, &out, e.conn, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// This codec is a hack to allow the grpc endpoint to work with the []byte object used in the PayloadSender

type rawProtoCodec struct{}

func (rawProtoCodec) Marshal(v interface{}) ([]byte, error) {
	b, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf("failed to marshal: %v is not type of []byte", v)
	}
	return b, nil
}

func (rawProtoCodec) Unmarshal(data []byte, v interface{}) error {
	b, ok := v.(*[]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal: %v is of type %v when it should have type *[]byte", v, reflect.TypeOf(v))
	}
	*b = data
	return nil
}

func (rawProtoCodec) String() string {
	return "proto"
}
