package main

import (
	"bytes"
	"testing"
)

func TestSplitKeys(t *testing.T) {
	cases := []struct {
		in   string
		want []byte
	}{
		{"q", []byte{'q'}},
		{"\x1b", []byte{0x1b}},         // lone Esc stays
		{"\x1b[A", nil},                // arrow up dropped
		{"\x1b[B", nil},                // arrow down dropped
		{"\x1b[64;10;10M", nil},        // mouse scroll dropped
		{"\x1bOP", nil},                // SS3 function key dropped
		{"a\x1b[Bq", []byte{'a', 'q'}}, // sequence between plain keys
	}
	for _, c := range cases {
		if got := splitKeys([]byte(c.in)); !bytes.Equal(got, c.want) {
			t.Errorf("splitKeys(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
