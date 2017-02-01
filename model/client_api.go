package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/tinylib/msgp/msgp"
)

// average size of a buffer; because usually the payloads are huge,
// this value ensures that initialized buffers are big enough so that
// the resize operation is not usually called while reading the response.
const (
	minBufferSize = 512
	maxBufferSize = 1e7
)

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
	Cap() int
}

type jsonDecoder struct {
	decoder     *json.Decoder
	buf         *bytes.Buffer
	slice       []byte
	contentType string
}

type msgpackDecoder struct {
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

func (d *jsonDecoder) Cap() int {
	return d.buf.Cap()
}

func newMsgpackDecoder() *msgpackDecoder {
	// sets the size of the buffer so that it usually doesn't need
	// to be expanded or reallocated
	buf := bytes.NewBuffer(make([]byte, 0, minBufferSize))
	return &msgpackDecoder{
		buf:         buf,
		slice:       buf.Bytes(),
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
	switch t := v.(type) {
	case *Traces:
		return msgp.Decode(d.buf, t)
	case *ServicesMetadata:
		return msgp.Decode(d.buf, t)
	default:
		return errors.New("No implementation for this interface")
	}
}

func (d *msgpackDecoder) BufferReader() *bytes.Reader {
	return bytes.NewReader(d.slice)
}

func (d *msgpackDecoder) ContentType() string {
	return d.contentType
}

func (d *msgpackDecoder) Cap() int {
	return d.buf.Cap()
}

// DecoderPool is a pool meant to share buffers for traces and services decoding.
// It naively tries to cap the number of active encoders, but doesn't enforce
// that limit. To use a pool, you should Borrow() a decoder and then Release()
// that decoder in the pool. Borrowing and Releasing decoders are thread-safe
// operation since channels are used.
// Decoders in that pool should honor the ClientDecoder interface.
// For compatibility reason, the pool includes both JSON and Msgpack decoders so that
// the caller should not decide which ClientDecoder is needed.
type DecoderPool struct {
	json    chan ClientDecoder
	msgpack chan ClientDecoder
}

// NewDecoderPool initializes a new struct that includes two different pools,
// one for JSON decoders and one for Msgpack decoders.
func NewDecoderPool(size int) *DecoderPool {
	return &DecoderPool{
		json:    make(chan ClientDecoder, size),
		msgpack: make(chan ClientDecoder, size),
	}
}

// Borrow is used to borrow a ClientDecoder according to given contentType.
// this operation ensures that if the pool limit is reached, a new decoder
// is created so that the call is not blocking. This operation is thread-safe
// since channels are used.
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

// Release is used to give back a ClientDecoder to the pool. According to the
// given contentType, the ClientDecoder is sent into the right pool. If the pool
// limit is reached, the decoder is discarded so that the call is not blocking.
// This operation is thread-safe since channels are used.
func (p *DecoderPool) Release(dec ClientDecoder) {
	// dropping the decoder if it reaches the maxBufferSize
	if dec.Cap() > maxBufferSize {
		return
	}

	switch dec.ContentType() {
	case "application/msgpack":
		select {
		case p.msgpack <- dec:
		default:
			// discard the ClientDecoder
		}
	default:
		select {
		case p.json <- dec:
		default:
			// discard the ClientDecoder
		}
	}
}
