package model

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimitedReader(t *testing.T) {
	buf := bytes.NewBufferString("foobar")
	r := ioutil.NopCloser(buf)
	lr := NewLimitedReader(r, 3)

	tmp := make([]byte, 1)
	n, err := lr.Read(tmp)
	assert.Nil(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, []byte("f"), tmp)

	tmp = make([]byte, 4)
	n, err = lr.Read(tmp)
	assert.Nil(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte("oo\x00\x00"), tmp)

	tmp = make([]byte, 1)
	n, err = lr.Read(tmp)
	assert.Equal(t, ErrLimitedReaderLimitReached, err)
}
