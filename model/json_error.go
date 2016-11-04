package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const padding = 50

// HumanReadableJSONError takes a generic reader that can seek
// (being passed to the JSON decoder) and the error that comes out
// of the unmarshalling and tries to return a human readable error
// that prints part of the offending payload.
func HumanReadableJSONError(r io.ReadSeeker, err error) string {
	var prettyerr bytes.Buffer
	switch err.(type) {
	case *json.SyntaxError:
		serr := err.(*json.SyntaxError)
		prettyerr.WriteString(fmt.Sprintf("json syntax error at offset:%d\n", serr.Offset))
		prettyerr.Write(tagInputWithOffset(r, serr.Offset))
	case *json.UnmarshalTypeError:
		serr := err.(*json.UnmarshalTypeError)
		prettyerr.WriteString(
			fmt.Sprintf("was expecting type %s and got type %s at offset:%d\n",
				serr.Type,
				serr.Value,
				serr.Offset,
			),
		)
		prettyerr.Write(tagInputWithOffset(r, serr.Offset))
	default:
		return err.Error()
	}

	return prettyerr.String()

}

func tagInputWithOffset(r io.ReadSeeker, offset int64) []byte {
	// we want to read up to <padding> chars more than the buffer
	start := offset - padding
	if start < 0 {
		start = 0
	}
	var errbuf bytes.Buffer
	r.Seek(start, io.SeekStart)
	n, err := io.CopyN(&errbuf, r, 2*padding)
	if err != nil && err != io.EOF {
		return nil
	}

	var resp bytes.Buffer
	resp.WriteString("error located at marker ---^:")
	resp.WriteRune('\n')
	resp.WriteString("    ")
	resp.Write(errbuf.Bytes())
	resp.WriteRune('\n')
	// pad to output a ---^ at the error
	errpos := offset - start
	for i := int64(0); i < errpos; i++ {
		resp.WriteRune(' ')
	}
	resp.WriteString("---^")
	return resp.Bytes()
}
