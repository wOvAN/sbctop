package main

import (
	"os"
	"strings"
	"syscall"
	"unsafe"
)

func versionLine(w int, ver string, drivers []string) string {
	s := "sbctop: " + ver
	for _, d := range drivers {
		if d != "" {
			s += "  " + strings.ReplaceAll(d, "\n", " ")
		}
	}
	if r := []rune(s); len(r) > w {
		return string(r[:max(w-1, 0)]) + "…"
	}
	return s
}

// vlen counts printable runes, skipping ANSI CSI escape sequences.
func vlen(s string) int {
	n, esc := 0, false
	for _, r := range s {
		switch {
		case r == 0x1b:
			esc = true
		case esc && r == 'm':
			esc = false
		case !esc:
			n++
		}
	}
	return n
}

func sideBySide(a, b []string) []string {
	n := max(len(a), len(b))
	a, b = padTo(a, n), padTo(b, n)
	wa := 0
	for _, l := range a {
		if vlen(l) > wa {
			wa = vlen(l)
		}
	}
	out := make([]string, n)
	for i := range out {
		out[i] = a[i] + strings.Repeat(" ", wa-vlen(a[i])+1) + b[i]
	}
	return out
}

func padTo(b []string, n int) []string {
	for len(b) < n {
		b = append(b, "")
	}
	return b[:n]
}

func padToWidth(s string, w int) string {
	if d := w - vlen(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func stackBlocks(a, b []string) []string {
	mw := 0
	for _, l := range a {
		if vlen(l) > mw {
			mw = vlen(l)
		}
	}
	for _, l := range b {
		if vlen(l) > mw {
			mw = vlen(l)
		}
	}
	out := make([]string, 0, len(a)+len(b))
	for _, l := range a {
		out = append(out, padToWidth(l, mw))
	}
	for _, l := range b {
		out = append(out, padToWidth(l, mw))
	}
	return out
}

// renderRow lays out blocks in one row, splitting the width evenly (the last
// block takes the remainder) and joining them with a 1-column gap. A single
// block spans the full width, which is how missing quadrants degrade.
func renderRow(fns []func(w, h int) []string, w, h int) []string {
	n := len(fns)
	if n == 0 {
		return nil
	}
	gap := 1
	each := (w - gap*(n-1)) / n
	var out []string
	for i, fn := range fns {
		wi := each
		if i == n-1 {
			wi = w - gap*(n-1) - each*(n-1)
		}
		part := padTo(fn(wi, h), h)
		if out == nil {
			out = part
		} else {
			out = sideBySide(out, part)
		}
	}
	return out
}

func termSize() (int, int) {
	ws := struct{ row, col, x, y uint16 }{}
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, os.Stdin.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	if e != 0 || ws.col < 2 || ws.row < 2 {
		return 80, 24
	}
	// Reserve the last column: writing to the final column sets the terminal's
	// pending-wrap flag, which pushes the right border onto the next line.
	return int(ws.col) - 1, int(ws.row)
}

func ioctl(fd, req uintptr, t *syscall.Termios) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(unsafe.Pointer(t)))
	if e != 0 {
		return e
	}
	return nil
}

func keyReader() (<-chan byte, func()) {
	fd := os.Stdin.Fd()
	var old syscall.Termios
	if ioctl(fd, syscall.TCGETS, &old) != nil {
		return nil, func() {}
	}
	raw := old
	raw.Lflag &^= syscall.ECHO | syscall.ICANON
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if ioctl(fd, syscall.TCSETS, &raw) != nil {
		return nil, func() {}
	}
	ch := make(chan byte, 8)
	go func() {
		for {
			b := make([]byte, 1)
			n, err := os.Stdin.Read(b)
			if n != 1 || err != nil {
				close(ch)
				return
			}
			if b[0] != 0x1b {
				ch <- b[0]
				continue
			}
			// ESC may open a CSI/SS3 sequence (arrows, fn keys, mouse). Drain
			// any bytes already queued, then drop the whole sequence or keep a
			// lone Esc as the quit key.
			buf := []byte{b[0]}
			setNonblock(fd, true)
			for {
				c, ok := readByte(fd)
				if !ok {
					break
				}
				buf = append(buf, c)
			}
			setNonblock(fd, false)
			for _, k := range splitKeys(buf) {
				ch <- k
			}
		}
	}()
	return ch, func() { _ = ioctl(fd, syscall.TCSETS, &old) }
}

// readByte reads one byte from stdin.
func readByte(fd uintptr) (byte, bool) {
	b := make([]byte, 1)
	n, err := os.Stdin.Read(b)
	return b[0], n == 1 && err == nil
}

// setNonblock toggles O_NONBLOCK on fd so a read can probe for queued input
// without blocking (syscall.FcntlInt was removed from the stdlib in Go 1.27).
func setNonblock(fd uintptr, on bool) {
	fl, _, _ := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFL, 0)
	if on {
		fl |= syscall.O_NONBLOCK
	} else {
		fl &^= syscall.O_NONBLOCK
	}
	_, _, _ = syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFL, fl)
}

// splitKeys turns a raw byte stream into logical keys: plain bytes pass
// through; an ESC that opens a CSI (ESC [) or SS3 (ESC O) sequence is dropped
// whole (consumed up to its final byte, 0x40-0x7e); a lone ESC is kept.
func splitKeys(data []byte) []byte {
	var out []byte
	for i := 0; i < len(data); {
		c := data[i]
		if c != 0x1b || i+1 >= len(data) || (data[i+1] != '[' && data[i+1] != 'O') {
			out = append(out, c)
			i++
			continue
		}
		i += 2
		for i < len(data) && (data[i] < 0x40 || data[i] > 0x7e) {
			i++
		}
		i++
	}
	return out
}
