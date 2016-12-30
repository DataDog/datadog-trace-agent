package model

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/ugorji/go/codec"
)

// average size of a buffer
const minBufferSize = 512

// readAll reads from source until an error or EOF and writes to dest;
// if the dest buffer contains data, it is truncated
func readAll(source io.Reader, dest *bytes.Buffer) (err error) {
	// If the buffer overflows, we will get bytes.ErrTooLarge.
	// Return that as an error. Any other panic remains.
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		if panicErr, ok := e.(error); ok && panicErr == bytes.ErrTooLarge {
			err = panicErr
		} else {
			panic(e)
		}
	}()

	// empty the buffer and copy the source into it
	dest.Reset()
	_, err = dest.ReadFrom(source)
	return err
}

// ClientDecoder is the common interface that all decoders should honor
type ClientDecoder interface {
	Decode(body io.Reader, v interface{}) error
}

type jsonDecoder struct {
	decoder *json.Decoder
	buf     *bytes.Buffer
}

type msgpackDecoder struct {
	decoder *codec.Decoder
	buf     *bytes.Buffer
}

func newJSONDecoder() *jsonDecoder {
	// sets the size of the buffer so that it usually doesn't need
	// to be expanded or reallocated
	buf := bytes.NewBuffer(make([]byte, 0, minBufferSize))
	return &jsonDecoder{
		buf:     buf,
		decoder: json.NewDecoder(buf),
	}
}

func (d *jsonDecoder) Decode(body io.Reader, v interface{}) error {
	// read the response into the buffer
	err := readAll(body, d.buf)
	if err != nil {
		return err
	}

	// decode the payload to the given interface
	return d.decoder.Decode(v)
}

func newMsgpackDecoder() *msgpackDecoder {
	// sets the size of the buffer so that it usually doesn't need
	// to be expanded or reallocated
	buf := bytes.NewBuffer(make([]byte, 0, minBufferSize))
	return &msgpackDecoder{
		buf:     buf,
		decoder: codec.NewDecoder(buf, &codec.MsgpackHandle{}),
	}
}

func (d *msgpackDecoder) Decode(body io.Reader, v interface{}) error {
	// read the response into the buffer
	err := readAll(body, d.buf)
	if err != nil {
		return err
	}

	// decode the payload to the given interface
	return d.decoder.Decode(v)
}

// DecoderFromContentType returns a ClientDecoder depending on the contentType value
// orig. coming from a request header
func DecoderFromContentType(contentType string) ClientDecoder {
	// select the right Decoder based on the given content-type header
	switch contentType {
	case "application/msgpack":
		return newMsgpackDecoder()
	default:
		// if the client doesn't use a specific decoder, fallback to JSON
		return newJSONDecoder()
	}
}
