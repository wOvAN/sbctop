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
		fmt.Println("rkmon", version)
		return
	}
	if *interval <= 0 {
		die("interval must be > 0")
	}

	readOrDie(npuLoadPath)
	kver := afterColon(readOrDie(npuVerPath))
	rv := afterColon(readOrDie(rgaVerPath))
	lib := libVersion()

	keys, restore := keyReader()
	termRestore = restore
	defer bye()
	fmt.Print("\033[?1049h\033[?25l")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var samples [][]int
	var prev []string
	for {
		loads := parseNPU(readOrDie(npuLoadPath))
		samples = append(samples, loads)
		if len(samples) > maxSamples {
			samples = samples[1:]
		}
		w, h := termSize()
		var frame []string
		if *npuOnly {
			frame = padTo(lineBlock(w, h-1, samples, npuState(), *darkMode), h-1)
		} else {
			topH, botH := (h-1)/2, (h-1)-(h-1)/2
			lw := w / 2
			rw := w - lw - 1
			var npuBlock []string
			if *rgaOn {
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
				npuBlock = hbarsBlock(lw, topH, "NPU/RGA Load", labels, vals, maxes, "%", *darkMode)
			} else if *npuBars {
				vals := make([]float64, len(loads))
				for i, v := range loads {
					vals[i] = float64(v)
				}
				npuBlock = cpuBlock("NPU Load", lw, topH, vals, *darkMode, npuState())
			} else {
				npuBlock = lineBlock(lw, topH, samples, npuState(), *darkMode)
			}
			ts := thermals()
			tLabels := make([]string, len(ts))
			tVals := make([]float64, len(ts))
			tMaxes := make([]float64, len(ts))
			for i, t := range ts {
				tLabels[i], tVals[i], tMaxes[i] = t.label, t.temp, t.high
			}
			thermalBlock := hbarsBlock(lw, botH, "Thermals", tLabels, tVals, tMaxes, "ºC", *darkMode)

			memU, memT, swpU, swpT := memSwap()
			g := readGPU()

			cpus := cpuPercents()
			memH := 5
			if memH > topH {
				memH = topH
			}
			cpuH := topH - memH
			cpuB := cpuBlock("CPU", rw, cpuH, cpus, *darkMode, "")
			memB := memBlock(rw, memH, memU, memT, swpU, swpT, *darkMode)
			topRight := stackBlocks(cpuB, memB)

			gpuB := gpuBlock(rw, botH, g, *darkMode)

			frame = append(sideBySide(padTo(npuBlock, topH), padTo(topRight, topH)),
				sideBySide(padTo(thermalBlock, botH), padTo(gpuB, botH))...)
		}
		frame = append(frame, versionLine(w, version, lib, kver, rv))
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
