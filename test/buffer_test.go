package test

import "testing"

func TestSafeBuffer(t *testing.T) {
	sb := newSafeBufferWithSize(10)
	check := func(contents string) {
		if sb.String() != contents {
			t.Fatalf("expected %q, found %q", contents, sb.String())
		}
	}

	sb.Write([]byte("12345"))
	check("12345")

	sb.Write([]byte("67"))
	check("1234567")

	sb.Write([]byte("123456"))
	check("4567123456")

	sb.Write([]byte("789"))
	check("7123456789")

	sb.Write([]byte("abcdefg"))
	check("789abcdefg")

	sb.Write([]byte("abcdefghij"))
	check("abcdefghij")

	sb.Write([]byte("abcdefghijklmnop"))
	check("ghijklmnop")
}
