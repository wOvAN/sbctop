# rkmon

Single-binary terminal monitor for Rockchip SBCs: NPU, RGA, Mali GPU, CPU,
memory and thermals. Zero dependencies (Go stdlib only), hand-rolled ANSI
TUI with a braille line plot.

## Install

On a Rockchip board (Linux/aarch64):

```sh
curl -fsSL https://raw.githubusercontent.com/wOvAN/rkmon/main/install.sh | sudo bash
```

## Usage

```sh
sudo rkmon            # full dashboard
sudo rkmon -n         # NPU load only (line plot)
sudo rkmon -b         # NPU load as bars
sudo rkmon -r         # merge RGA load into the NPU block
sudo rkmon -d         # dark-mode colors
sudo rkmon -i 0.5     # refresh every 500 ms
rkmon -v              # print version
```

Quit with `q`, `Q`, `Esc` or Ctrl-C.

Requires root: the data comes from debugfs (`/sys/kernel/debug/rknpu`,
`rkrga`, `mali0`) which is root-only. Off-device the binary exits with
"Cannot read ... Run with `sudo`" — that is expected, it needs a Rockchip
SoC (tested on RK3588).

## Build

```sh
make          # lint + build rkmon-arm64
make test     # unit tests, run anywhere (pure parsers/renderers)
make install  # install to /usr/local/bin
```

## Screens

- **NPU Load** — braille line plot of per-core load history, plus NPU
  frequency/voltage/power state; `-b` switches to bars, `-r` adds RGA cores.
- **Thermals** — one bar per thermal zone against its critical temperature.
- **CPU** — htop-style per-core bars, two columns when they do not fit.
- **Memory** — RAM and swap usage bars.
- **GPU** — devfreq load/clock/governor, DDR clock, Mali busy time, active
  contexts, total GPU memory and the top memory-consuming processes.

## Releases

Pushing a `v*` tag builds the arm64 binary and publishes a GitHub Release
(`.github/workflows/release.yml`).
