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
	BufferReader() *bytes.Reader
	ContentType() string
}

type jsonDecoder struct {
	decoder     *json.Decoder
	buf         *bytes.Buffer
	slice       []byte
	contentType string
}

type msgpackDecoder struct {
	decoder     *codec.Decoder
	buf         *bytes.Buffer
	slice       []byte
	contentType string
}

func newJSONDecoder() *jsonDecoder {
	// sets the size of the buffer so that it usually doesn't need
	// to be expanded or reallocated
	buf := bytes.NewBuffer(make([]byte, 0, minBufferSize))
	return &jsonDecoder{
		buf:         buf,
		slice:       buf.Bytes(),
		decoder:     json.NewDecoder(buf),
		contentType: "application/json",
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

func (d *jsonDecoder) BufferReader() *bytes.Reader {
	return bytes.NewReader(d.slice)
}

func (d *jsonDecoder) ContentType() string {
	return d.contentType
}

func newMsgpackDecoder() *msgpackDecoder {
	// sets the size of the buffer so that it usually doesn't need
	// to be expanded or reallocated
	buf := bytes.NewBuffer(make([]byte, 0, minBufferSize))
	return &msgpackDecoder{
		buf:         buf,
		slice:       buf.Bytes(),
		decoder:     codec.NewDecoder(buf, &codec.MsgpackHandle{}),
		contentType: "application/msgpack",
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

func (d *msgpackDecoder) BufferReader() *bytes.Reader {
	return bytes.NewReader(d.slice)
}

func (d *msgpackDecoder) ContentType() string {
	return d.contentType
}

// DecoderPool is a pool meant to share buffers required to decode traces.
// It naively tries to cap the number of active encoders, but doesn't enforce
// the limit. To use a pool, you should Borrow() for a decoder and then
// Return() that decoder to the pool. Decoders in that pool should honor
// the ClientDecoder interface.
// For compatibility, the pool owns both JSON and Msgpack decoders so that
// we don't need multiple pools. Borrowing a decoder, means asking a decoder
// for the given content type
type DecoderPool struct {
	json    chan ClientDecoder
	msgpack chan ClientDecoder
}

func NewDecoderPool(size int) *DecoderPool {
	return &DecoderPool{
		json:    make(chan ClientDecoder, size),
		msgpack: make(chan ClientDecoder, size),
	}
}

func (p *DecoderPool) Borrow(contentType string) ClientDecoder {
	// select the right Decoder based on the given content-type header
	var decoder ClientDecoder

	switch contentType {
	case "application/msgpack":
		select {
		case decoder = <-p.msgpack:
		default:
			decoder = newMsgpackDecoder()
		}
	default:
		// if the client doesn't use a specific decoder, fallback to JSON
		select {
		case decoder = <-p.json:
		default:
			decoder = newJSONDecoder()
		}
	}

	return decoder
}

func (p *DecoderPool) Release(dec ClientDecoder) {
	switch dec.ContentType() {
	case "application/msgpack":
		select {
		case p.msgpack <- dec:
		default:
			// discard
		}
	default:
		select {
		case p.json <- dec:
		default:
			// discard
		}
	}
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
