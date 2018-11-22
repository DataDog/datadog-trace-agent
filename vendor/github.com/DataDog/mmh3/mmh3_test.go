package mmh3

import (
	"fmt"
	"testing"
)

func TestAll(t *testing.T) {
	s := []byte("hello")
	if Hash32(s) != 0x248bfa47 {
		t.Fatal("32bit hello")
	}
	if fmt.Sprintf("%x", Hash128(s)) != "029bbd41b3a7d8cb191dae486a901e5b" {
		t.Fatal("128bit hello")
	}
	s = []byte("Winter is coming")
	if Hash32(s) != 0x43617e8f {
		t.Fatal("32bit winter")
	}
	if fmt.Sprintf("%x", Hash128(s)) != "95eddc615d3b376c13fb0b0cead849c5" {
		t.Fatal("128bit winter")
	}
}

// Test the x64 optimized hash against Hash128
func Testx64(t *testing.T) {
	keys := []string{
		"hello",
		"Winter is coming",
	}

	for _, k := range keys {
		h128 := Hash128([]byte(k))
		h128x64 := Hash128x64([]byte(k))

		if string(h128) != string(h128x64) {
			t.Fatalf("Expected same hashes for %s, but got %x and %x", k, h128, h128x64)
		}
	}

}

func TestHashWriter128(t *testing.T) {
	s := []byte("hello")
	h := HashWriter128{}
	h.Write(s)
	res := h.Sum(nil)
	if fmt.Sprintf("%x", res) != "029bbd41b3a7d8cb191dae486a901e5b" {
		t.Fatal("128bit hello")
	}
	s = []byte("Winter is coming")
	h.Reset()
	h.Write(s)
	res = h.Sum(nil)
	if fmt.Sprintf("%x", res) != "95eddc615d3b376c13fb0b0cead849c5" {
		t.Fatal("128bit hello")
	}
	str := "Winter is coming"
	h.Reset()
	h.WriteString(str)
	res = h.Sum(nil)
	if fmt.Sprintf("%x", res) != "95eddc615d3b376c13fb0b0cead849c5" {
		t.Fatal("128bit hello")
	}

}
