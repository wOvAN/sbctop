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
	npuBars := flag.Bool("b", false, "Show the NPU with bars instead of lines")
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

	var drivers []string
	if cap.npu {
		drivers = append(drivers, "librknnrt: "+lib, "Kmod: "+kver)
	}
	if cap.rga {
		drivers = append(drivers, "RGA: "+rv)
	}

	var samples [][]int
	var prev []string
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

		npuBlock := func(w, h int) []string {
			if *rgaOn && cap.rga {
				names, rloads := parseRGA(readOrDie(rgaLoadPath))
				labels := make([]string, len(loads)+len(names))
				vals := make([]float64, len(loads)+len(names))
				maxes := make([]float64, len(labels))
				for i, v := range loads {
					labels[i], vals[i], maxes[i] = fmt.Sprintf("NPU%d", i), float64(v), 100
				}
				for i, nm := range names {
					labels[len(loads)+i], vals[len(loads)+i], maxes[len(loads)+i] = nm, rloads[i], 100
				}
				return hbarsBlock(w, h, "NPU/RGA Load", labels, vals, maxes, "%", *darkMode)
			}
			if *npuBars {
				vals := make([]float64, len(loads))
				for i, v := range loads {
					vals[i] = float64(v)
				}
				return cpuBlock("NPU Load", w, h, vals, *darkMode, npuState())
			}
			return lineBlock(w, h, samples, npuState(), *darkMode)
		}

		var frame []string
		if *npuOnly && cap.npu {
			frame = padTo(npuBlock(w, h-1), h-1)
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
				return stackBlocks(cpuBlock("CPU", w, h-memH, cpus, *darkMode, ""),
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
			frame = append(renderRow(topFns, w, topH), renderRow(botFns, w, botH)...)
		}
		frame = append(frame, versionLine(w, version, drivers))
		var sb strings.Builder
		for i, l := range frame {
			if i < len(prev) && prev[i] == l {
				continue
			}
			fmt.Fprintf(&sb, "\033[%d;1H%s\033[K", i+1, l)
		}
		for i := len(frame); i < len(prev); i++ {
			fmt.Fprintf(&sb, "\033[%d;1H\033[K", i+1)
		}
		prev = frame
		_, _ = os.Stdout.Write([]byte(sb.String()))

		timer := time.NewTimer(time.Duration(*interval * float64(time.Second)))
		select {
		case k, ok := <-keys:
			timer.Stop()
			if !ok || k == 'q' || k == 'Q' || k == 27 {
				return
			}
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
