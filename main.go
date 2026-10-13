package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var version = "dev"

func main() {
	npuOnly := flag.Bool("n", false, "Only show the NPU load")
	npuBars := flag.Bool("b", false, "Show the NPU with bars instead of sparklines")
	rgaOn := flag.Bool("r", false, "Show the RGA load")
	darkMode := flag.Bool("d", false, "Enable dark mode")
	interval := flag.Float64("i", 1.0, "Refresh interval in seconds")
	showVer := flag.Bool("v", false, "Print version and exit")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: Show different NPU/CPU stats")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVer {
		fmt.Println("sbctop", version)
		return
	}
	if *interval <= 0 {
		die("interval must be > 0")
	}

	cap := detectCaps()
	kver := afterColon(readSafe(npuVerPath))
	rv := afterColon(readSafe(rgaVerPath))
	lib := libVersion()

	keys, restore := keyReader()
	termRestore = restore
	defer bye()
	fmt.Print("\033[?1049h\033[?25l")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	sigWinch := make(chan os.Signal, 1)
	signal.Notify(sigWinch, syscall.SIGWINCH)
	defer signal.Stop(sigWinch)

	var drivers []string
	if cap.npu {
		drivers = append(drivers, "librknnrt: "+lib, "Kmod: "+kver)
	}
	if cap.rga {
		drivers = append(drivers, "RGA: "+rv)
	}

	var samples [][]int
	var prev []string
	pw, ph := 0, 0
	bars := *npuBars // N/N toggles sparklines <-> bars at runtime
	for {
		var loads []int
		if cap.npu {
			loads = parseNPU(readOrDie(npuLoadPath))
			samples = append(samples, loads)
			if len(samples) > maxSamples {
				samples = samples[1:]
			}
		}
		w, h := termSize()
		resized := w != pw || h != ph
		if resized {
			pw, ph = w, h
			prev = nil // force a full repaint (see renderDiff)
		}

		var rNames []string
		var rLoads []float64
		if *rgaOn && cap.rga {
			rNames, rLoads = parseRGA(readOrDie(rgaLoadPath))
		}
		rgaBlock := func(w, h int) []string {
			maxes := make([]float64, len(rNames))
			for i := range maxes {
				maxes[i] = 100
			}
			return hbarsBlock(w, h, "RGA", rNames, rLoads, maxes, "%", *darkMode)
		}
		npuBlock := func(w, h int) []string {
			if bars {
				vals := make([]float64, len(loads))
				for i, v := range loads {
					vals[i] = float64(v)
				}
				return cpuBlock("NPU", w, h, vals, *darkMode, npuState(), nil)
			}
			return sparkRowsBlock(w, h, samples, *darkMode, npuState())
		}

		var frame []string
		if *npuOnly && cap.npu {
			if len(rNames) > 0 {
				rh := len(rNames) + 2
				frame = padTo(stackBlocks(npuBlock(w, h-1-rh), rgaBlock(w, rh)), h-1)
			} else {
				frame = padTo(npuBlock(w, h-1), h-1)
			}
		} else {
			cpus := cpuPercents()
			memU, memT, swpU, swpT := memSwap()
			ts := thermals()
			tLabels := make([]string, len(ts))
			tVals := make([]float64, len(ts))
			tMaxes := make([]float64, len(ts))
			for i, t := range ts {
				tLabels[i], tVals[i], tMaxes[i] = t.label, t.temp, t.high
			}
			var g gpuInfo
			var piLoad float64
			hasGPU := false
			if cap.gpu {
				g = readGPU()
				hasGPU = true
			}
			if cap.piGPU {
				piLoad = readPiGPU()
				hasGPU = true
			}

			topH, botH := (h-1)/2, (h-1)-(h-1)/2
			var topFns, botFns []func(w, h int) []string
			if cap.npu {
				topFns = append(topFns, npuBlock)
			}
			topFns = append(topFns, func(w, h int) []string {
				memH := 5
				if memH > h {
					memH = h
				}
				return stackBlocks(cpuBlock("CPU", w, h-memH, cpus, *darkMode, nil, cpuFreqs()),
					memBlock(w, memH, memU, memT, swpU, swpT, *darkMode))
			})
			botFns = append(botFns, func(w, h int) []string {
				return hbarsBlock(w, h, "Thermals", tLabels, tVals, tMaxes, "ºC", *darkMode)
			})
			if hasGPU {
				botFns = append(botFns, func(w, h int) []string {
					if cap.gpu {
						return gpuBlock(w, h, g, *darkMode)
					}
					return piGpuBlock(w, h, piLoad, *darkMode)
				})
			}
			if len(rNames) > 0 {
				botFns = append(botFns, rgaBlock)
			}
			frame = append(renderRow(topFns, w, topH), renderRow(botFns, w, botH)...)
		}
		frame = append(frame, versionLine(w, version, drivers))
		var sb strings.Builder
		if resized {
			sb.WriteString("\033[2J") // drop reflow/clip leftovers anywhere on screen
		}
		renderDiff(&sb, prev, frame)
		prev = frame
		_, _ = os.Stdout.Write([]byte(sb.String()))

		timer := time.NewTimer(time.Duration(*interval * float64(time.Second)))
		select {
		case k, ok := <-keys:
			timer.Stop()
			if !ok || k == 'q' || k == 'Q' || k == 27 {
				return
			}
			if k == 'n' || k == 'N' {
				bars = !bars
			}
		case <-sigWinch:
			timer.Stop()
			// Fall through to an immediate re-render; the size check above
			// turns it into a full repaint when the window actually changed.
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
