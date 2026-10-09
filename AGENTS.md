# AGENTS.md

`rkmon` — single-binary terminal monitor for Rockchip SBCs (NPU / RGA / Mali GPU / CPU / thermals).
Module `rkmon`, Go 1.27, **zero dependencies** (stdlib only, no `go.sum`). The TUI is hand-rolled
ANSI + termios; do not add a TUI library.

## Commands

```
make                                             # lint + build (rkmon-arm64, the committed binary)
make test                                        # all tests, no hardware needed
go test -run TestParseNPU                        # single test
make fmt vet lint clean                          # the rest
```

## Layout (one `package main`, 4 files)

- `main.go` — flags, main loop, frame diffing (only changed lines are rewritten).
- `collect.go` — all sysfs/procfs reads + pure parsers (`parseNPU`, `parseRGA`, `parseGPUMem`, `parseTransStat`).
- `render.go` — pure renderers returning `[]string` lines: `canvas` (braille line plot), `vbarsBlock`, `hbarsBlock`, `cpuBlock`, `memBlock`, `gpuBlock`.
- `term.go` — layout combinators (`sideBySide`, `stackBlocks`, `padTo`), terminal size, raw-key reader.

## Gotchas

- Running the binary needs a Rockchip board with debugfs **and root** (`/sys/kernel/debug/rknpu`,
  `rkrga`, `mali0`). Off-device it exits 2 with "Cannot read ... Run with `sudo`" — expected, not a bug.
- Keep parsers pure (`string` in, values out) and renderers pure (`w,h` + values in, lines out).
  That is why the suite runs on any machine: tests pass fixture strings, never touch `/sys`.
- Measure display width with `vlen()`, never `len()`/`len([]rune)` — lines carry ANSI color codes.
- `termSize()` returns `cols-1` on purpose: writing the last column trips the terminal's pending-wrap
  flag and breaks box borders. Frame diffing requires renderers to be deterministic.
