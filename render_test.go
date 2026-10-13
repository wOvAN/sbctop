package main

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	samples := [][]int{{10, 20}, {50, 60}, {90, 100}}
	sub := []kv{{"freq", "1000MHz"}, {"volt", "800mV"}, {"power", "off"}}
	block := sparkRowsBlock(60, 12, samples, true, sub)
	if len(block) != 12 {
		t.Fatalf("height: %d", len(block))
	}
	joined := strings.Join(block, "")
	hasSpark := strings.ContainsFunc(joined, func(r rune) bool {
		for _, s := range spark {
			if r == s {
				return true
			}
		}
		return false
	})
	if !strings.Contains(joined, "NPU") || !hasSpark {
		t.Fatalf("no sparkline plot rendered:\n%s", joined)
	}
	// Key/value separation: dimmed keys, bold values.
	for _, want := range []string{"\033[90mfreq", "\033[1m1000MHz", "\033[90mvolt", "\033[1m800mV"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("subtitle pair styling missing %q:\n%s", want, joined)
		}
	}
	hb := hbarsBlock(40, 5, "Thermals", []string{"soc"}, []float64{55}, []float64{85}, "ºC", false)
	if !strings.Contains(strings.Join(hb, ""), "55ºC") {
		t.Fatalf("thermal text missing:\n%s", strings.Join(hb, "\n"))
	}
}

func TestSparkRowsIdleCoreStaysVisible(t *testing.T) {
	// Regression: Core1/Core2 idle at 0 used to vanish — overlaid line plots
	// collapse coincident series into the last core's color. One row per core
	// must keep every series painted in its own color.
	samples := [][]int{{10, 0, 0}, {30, 0, 0}, {70, 0, 5}, {40, 0, 0}}
	joined := strings.Join(sparkRowsBlock(54, 10, samples, false, nil), "")
	for _, want := range []string{"Core0", "Core1", "Core2", "\033[32m", "\033[90mCore1"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q:\n%s", want, joined)
		}
	}
	if n := strings.Count(joined, "\033[32m"); n < 2 {
		t.Fatalf("Core1 green only %d time(s), row must be painted: %q", n, joined)
	}
	// Deterministic: frame diffing requires identical re-renders.
	again := strings.Join(sparkRowsBlock(54, 10, samples, false, nil), "")
	if joined != again {
		t.Fatal("non-deterministic render")
	}
}

func TestSparkRowsOverflow(t *testing.T) {
	samples := [][]int{{1, 2, 3, 4, 5, 6}}
	joined := strings.Join(sparkRowsBlock(40, 6, samples, false, []kv{{"freq", "1"}}), "")
	if !strings.Contains(joined, "Core0") || !strings.Contains(joined, "+") {
		t.Fatalf("expected shown cores + overflow marker:\n%s", joined)
	}
	if strings.Contains(joined, "Core5") {
		t.Fatalf("Core5 should overflow at h=6:\n%s", joined)
	}
}

func TestDVFSSpark(t *testing.T) {
	opp := []uint64{1000000000, 900000000, 300000000}
	freqs := []uint64{1000000000, 300000000}
	ms := []float64{0, 100}
	got := dvfsSpark(opp, freqs, ms)
	// Ascending: 300MHz has all the time (full bar), 900/1000MHz empty (lowest bar).
	if !strings.HasPrefix(got, "█▁▁") || !strings.HasSuffix(got, "300-1000M") {
		t.Fatalf("spark: %q", got)
	}
	if dvfsSpark(nil, nil, nil) != "n/a" {
		t.Fatal("empty spark should be n/a")
	}
}

func TestCPUBlockHtop(t *testing.T) {
	got := cpuBlock("CPU", 40, 12, []float64{0, 50, 100}, true, nil, nil)
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "▒") || strings.Contains(joined, "█") {
		t.Fatalf("htop uses '|' not blocks:\n%s", joined)
	}
	if !strings.Contains(joined, "0[") || !strings.Contains(joined, "100.0%") {
		t.Fatalf("missing htop bracket/pct:\n%s", joined)
	}
	if strings.Contains(joined, "M") {
		t.Fatalf("no freqs requested, no freq column expected:\n%s", joined)
	}
	got = cpuBlock("CPU", 40, 12, []float64{0, 50, 100}, true, nil, []int{408, 1800, 2400})
	joined = strings.Join(got, "\n")
	if !strings.Contains(joined, "2400M") {
		t.Fatalf("missing per-core freq:\n%s", joined)
	}
}

func TestCPUBlockTwoColumns(t *testing.T) {
	// 16 cores in a 8-row block: doesn't fit in 1 column (needs 16+2), so 2 columns of 8.
	vals := make([]float64, 16)
	for i := range vals {
		vals[i] = float64(i * 6)
	}
	got := cpuBlock("CPU", 80, 8, vals, true, nil, nil)
	joined := strings.Join(got, "\n")
	// Both core 0 and core 8 (second column) must appear.
	if !strings.Contains(joined, "0[") || !strings.Contains(joined, "8[") {
		t.Fatalf("2-column split missing core 8:\n%s", joined)
	}
}

func TestGPUBlockBorderIntact(t *testing.T) {
	g := gpuInfo{
		load: 50, freq: 1e9, minF: 3e8, maxF: 1e9, gov: "performance",
		state: "SUSPENDED", memMB: 1024, memFreq: 2112e6, memMinF: 528e6,
		memMaxF: 2112e6, memGov: "dmc_ondemand",
		opp: []uint64{1e9, 9e8, 3e8}, dvfsFreq: []uint64{3e8}, dvfsMs: []float64{100},
	}
	for _, l := range gpuBlock(40, 20, g, true) {
		if r := []rune(l); len(r) > 0 && r[0] == '│' && r[len(r)-1] != '│' {
			t.Fatalf("line lost right border: %q", l)
		}
	}
}
