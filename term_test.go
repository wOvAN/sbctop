package main

import (
	"bytes"
	"strings"
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

func TestRenderDiff(t *testing.T) {
	var sb strings.Builder
	// Unchanged lines are skipped, changed ones rewritten with EL clear.
	renderDiff(&sb, []string{"a", "b"}, []string{"a", "B"})
	if got, want := sb.String(), "\033[2;1HB\033[K"; got != want {
		t.Fatalf("diff = %q, want %q", got, want)
	}
	// Shrinking the frame clears leftover rows below it.
	sb.Reset()
	renderDiff(&sb, []string{"a", "b", "c"}, []string{"a"})
	if got, want := sb.String(), "\033[2;1H\033[K\033[3;1H\033[K"; got != want {
		t.Fatalf("shrink = %q, want %q", got, want)
	}
	// Nil prev (post-resize full repaint) rewrites every line.
	sb.Reset()
	renderDiff(&sb, nil, []string{"a", "b"})
	if got, want := sb.String(), "\033[1;1Ha\033[K\033[2;1Hb\033[K"; got != want {
		t.Fatalf("full = %q, want %q", got, want)
	}
}
