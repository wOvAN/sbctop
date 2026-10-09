package main

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	samples := [][]int{{10, 20}, {50, 60}, {90, 100}}
	block := lineBlock(60, 12, samples, "NPU: 1000MHz 800mV off", true)
	if len(block) != 12 {
		t.Fatalf("height: %d", len(block))
	}
	joined := strings.Join(block, "")
	braille := strings.ContainsFunc(joined, func(r rune) bool { return r >= 0x2800 && r <= 0x28ff })
	if !strings.Contains(joined, "NPU Load") || !braille {
		t.Fatalf("no braille plot rendered:\n%s", joined)
	}
	vb := vbarsBlock(40, 8, "CPU Load per core", []float64{50, 100}, true)
	if !strings.Contains(strings.Join(vb, ""), "█") {
		t.Fatal("no bars rendered")
	}
	hb := hbarsBlock(40, 5, "Thermals", []string{"soc"}, []float64{55}, []float64{85}, "ºC", false)
	if !strings.Contains(strings.Join(hb, ""), "55ºC") {
		t.Fatalf("thermal text missing:\n%s", strings.Join(hb, "\n"))
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
	got := cpuBlock(40, 12, []float64{0, 50, 100}, true)
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "▒") || strings.Contains(joined, "█") {
		t.Fatalf("htop uses '|' not blocks:\n%s", joined)
	}
	if !strings.Contains(joined, "0[") || !strings.Contains(joined, "100.0%") {
		t.Fatalf("missing htop bracket/pct:\n%s", joined)
	}
}

func TestCPUBlockTwoColumns(t *testing.T) {
	// 16 cores in a 8-row block: doesn't fit in 1 column (needs 16+2), so 2 columns of 8.
	vals := make([]float64, 16)
	for i := range vals {
		vals[i] = float64(i * 6)
	}
	got := cpuBlock(80, 8, vals, true)
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
