# sbctop

Single-binary terminal monitor for single-board computers (Rockchip,
Raspberry Pi): NPU, RGA, GPU, CPU, memory and thermals. Zero dependencies
(Go stdlib only), hand-rolled ANSI TUI with a braille line plot.

## Boards

Tested on:

- **Rockchip RK3588** — full set: NPU, RGA, Mali GPU, CPU, memory, thermals.
- **Raspberry Pi 4** (BCM2711) — CPU, memory, thermals, VideoCore VI GPU
  (load + fixed clock).

Any Linux/aarch64 SBC works: CPU, memory and thermals are generic, and the
NPU / RGA / GPU blocks appear only when the board exposes them (Rockchip
`rknpu` / `rkrga` / `mali0`, Raspberry Pi `v3d`). Other Rockchip SoCs and
Raspberry Pi 3 are expected to work the same way.

## Install

On an SBC (Linux/aarch64):

```sh
curl -fsSL https://raw.githubusercontent.com/wOvAN/sbctop/main/install.sh | sudo bash
```

## Usage

```sh
sudo sbctop            # full dashboard
sudo sbctop -n         # NPU load only (line plot)
sudo sbctop -b         # NPU load as bars
sudo sbctop -r         # merge RGA load into the NPU block
sudo sbctop -d         # dark-mode colors
sudo sbctop -i 0.5     # refresh every 500 ms
sbctop -v              # print version
```

Quit with `q`, `Q`, `Esc` or Ctrl-C. `n` toggles the NPU block between
sparklines and bars.

Requires root: the data comes from debugfs/sysfs (Rockchip `rknpu` / `rkrga`
/ `mali0`, Raspberry Pi `v3d`) which is root-only. Off-device the binary
exits with "Cannot read ... Run with `sudo`" — that is expected (tested on
RK3588 and Raspberry Pi 4).

## Build

```sh
make          # lint + build sbctop-arm64
make test     # unit tests, run anywhere (pure parsers/renderers)
make install  # install to /usr/local/bin
```

## Screens

- **NPU** — one history sparkline per core, plus NPU
  frequency/voltage/power state; `n` (or `-b`) switches to bars, `-r` adds RGA cores.
- **Thermals** — one bar per thermal zone against its critical temperature.
- **CPU** — htop-style per-core bars, two columns when they do not fit.
- **Memory** — RAM and swap usage bars.
- **GPU** — backend-specific: Rockchip Mali (devfreq load/clock/governor,
  DDR clock, busy time, active contexts, total memory, top processes) or
  Raspberry Pi VideoCore (load + fixed clock).

## Releases

Pushing a `v*` tag builds the arm64 binary and publishes a GitHub Release
(`.github/workflows/release.yml`).
