package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	npuLoadPath  = "/sys/kernel/debug/rknpu/load"
	npuVerPath   = "/sys/kernel/debug/rknpu/version"
	npuFreqPath  = "/sys/kernel/debug/rknpu/freq"
	npuVoltPath  = "/sys/kernel/debug/rknpu/volt"
	npuPowerPath = "/sys/kernel/debug/rknpu/power"
	rgaLoadPath  = "/sys/kernel/debug/rkrga/load"
	rgaVerPath   = "/sys/kernel/debug/rkrga/driver_version"
	libRknnPath  = "/usr/lib/librknnrt.so"
	maxSamples   = 100
)

// termRestore restores termios; set once the TUI starts. bye leaves the
// alternate screen with a clean, visible-cursor main screen on every exit path.
var termRestore = func() {}

func bye() {
	termRestore()
	fmt.Print("\033[0m\033[?25h\033[?1049l\033[2J\033[H")
}

func die(msg string) {
	bye()
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}

func readOrDie(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		die(fmt.Sprintf("Cannot read %s. Run with `sudo`", path))
	}
	return string(b)
}

func readFloat(path string) (float64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
}

var coreRe = regexp.MustCompile(`Core(\d+):\s*(\d+)%`)
var singleRe = regexp.MustCompile(`NPU load:\s+(\d+)%`)

func parseNPU(txt string) []int {
	var res []int
	if strings.Contains(txt, "Core0:") {
		for _, m := range coreRe.FindAllStringSubmatch(txt, -1) {
			i, _ := strconv.Atoi(m[1])
			v, _ := strconv.Atoi(m[2])
			for len(res) <= i {
				res = append(res, 0)
			}
			res[i] = v
		}
	} else {
		for _, m := range singleRe.FindAllStringSubmatch(txt, -1) {
			v, _ := strconv.Atoi(m[1])
			res = append(res, v)
		}
	}
	return res
}

func parseRGA(txt string) ([]string, []float64) {
	var names []string
	var loads []float64
	cur := ""
	for _, line := range strings.Split(txt, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "scheduler"):
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				cur = strings.TrimSpace(parts[1])
			}
		case strings.HasPrefix(line, "load ="):
			v := strings.TrimSuffix(strings.TrimSpace(strings.SplitN(line, "=", 2)[1]), "%")
			f, err := strconv.ParseFloat(v, 64)
			if err == nil {
				names = append(names, cur)
				loads = append(loads, f)
			}
		}
	}
	return names, loads
}

func afterColon(s string) string {
	if i := strings.Index(s, ":"); i >= 0 {
		return strings.TrimSpace(s[i+1:])
	}
	return strings.TrimSpace(s)
}

func npuState() string {
	freq, _ := readFloat(npuFreqPath)
	volt, _ := readFloat(npuVoltPath)
	power, _ := os.ReadFile(npuPowerPath)
	return fmt.Sprintf("NPU: freq %dMHz  volt %dmV  power %s",
		uint64(freq)/1e6, uint64(volt)/1000, strings.TrimSpace(string(power)))
}

type gpuProc struct {
	comm  string
	pid   int
	memMB uint64
}

type gpuInfo struct {
	load     float64
	util     float64
	ctx      int
	freq     uint64
	minF     uint64
	maxF     uint64
	gov      string
	memMB    uint64
	state    string
	procs    []gpuProc
	memFreq  uint64
	memMinF  uint64
	memMaxF  uint64
	memGov   string
	opp      []uint64
	dvfsFreq []uint64
	dvfsMs   []float64
}

var (
	prevBusy, prevIdle uint64
	gpuPrimed          bool
)

func readGPU() gpuInfo {
	g := gpuInfo{}
	dirs, _ := filepath.Glob("/sys/class/devfreq/*gpu*")
	if len(dirs) > 0 {
		d := dirs[0]
		if b, err := os.ReadFile(filepath.Join(d, "load")); err == nil {
			if i := strings.Index(string(b), "@"); i >= 0 {
				g.load, _ = strconv.ParseFloat(strings.TrimSpace(string(b[:i])), 64)
			}
		}
		if b, err := os.ReadFile(filepath.Join(d, "cur_freq")); err == nil {
			g.freq, _ = strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		}
		if b, err := os.ReadFile(filepath.Join(d, "min_freq")); err == nil {
			g.minF, _ = strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		}
		if b, err := os.ReadFile(filepath.Join(d, "max_freq")); err == nil {
			g.maxF, _ = strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		}
		if b, err := os.ReadFile(filepath.Join(d, "governor")); err == nil {
			g.gov = strings.TrimSpace(string(b))
		}
		if b, err := os.ReadFile(filepath.Join(d, "available_frequencies")); err == nil {
			for _, s := range strings.Fields(string(b)) {
				f, _ := strconv.ParseUint(s, 10, 64)
				g.opp = append(g.opp, f)
			}
		}
		if b, err := os.ReadFile(filepath.Join(d, "trans_stat")); err == nil {
			g.dvfsFreq, g.dvfsMs = parseTransStat(string(b))
		}
	}
	if dirs, _ := filepath.Glob("/sys/class/devfreq/*dmc*"); len(dirs) > 0 {
		d := dirs[0]
		if b, err := os.ReadFile(filepath.Join(d, "cur_freq")); err == nil {
			g.memFreq, _ = strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		}
		if b, err := os.ReadFile(filepath.Join(d, "min_freq")); err == nil {
			g.memMinF, _ = strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		}
		if b, err := os.ReadFile(filepath.Join(d, "max_freq")); err == nil {
			g.memMaxF, _ = strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		}
		if b, err := os.ReadFile(filepath.Join(d, "governor")); err == nil {
			g.memGov = strings.TrimSpace(string(b))
		}
	}
	if b, err := os.ReadFile("/sys/kernel/debug/mali0/scheduler_state"); err == nil {
		g.state = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile("/sys/kernel/debug/mali0/dvfs_utilization"); err == nil {
		f := strings.Fields(string(b))
		if len(f) >= 4 {
			busy, _ := strconv.ParseUint(f[1], 10, 64)
			idle, _ := strconv.ParseUint(f[3], 10, 64)
			db, di := busy-prevBusy, idle-prevIdle
			prevBusy, prevIdle = busy, idle
			// First frame has no baseline: counters are cumulative since boot.
			if gpuPrimed && db+di > 0 {
				g.util = float64(db) / float64(db+di) * 100
			}
			gpuPrimed = true
		}
	}
	if b, err := os.ReadFile("/sys/kernel/debug/mali0/active_groups"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "Ctx ") {
				g.ctx++
			}
		}
	}
	if mb, err := os.ReadFile("/sys/kernel/debug/mali0/gpu_memory"); err == nil {
		var total uint64
		total, g.procs = parseGPUMem(string(mb))
		g.memMB = total
	}
	return g
}

// parseGPUMem reads mali0 gpu_memory: a 2-field "<dev> <pages>" device line with
// the total, then 4-field "<dev> <comm> <pid> <pages>" per-process rows.
// Returns total MB and per-process MB sorted by size.
func parseGPUMem(txt string) (uint64, []gpuProc) {
	var total uint64
	var procs []gpuProc
	for _, line := range strings.Split(txt, "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[0] == "mali0" {
			if pages, err := strconv.ParseUint(f[1], 10, 64); err == nil {
				total = pages * 4096 / 1024 / 1024
			}
			continue
		}
		if len(f) < 4 {
			continue
		}
		pid, e1 := strconv.Atoi(f[2])
		pages, e2 := strconv.ParseUint(f[3], 10, 64)
		if e1 != nil || e2 != nil {
			continue
		}
		procs = append(procs, gpuProc{comm: f[1], pid: pid, memMB: pages * 4096 / 1024 / 1024})
	}
	sort.Slice(procs, func(i, j int) bool { return procs[i].memMB > procs[j].memMB })
	return total, procs
}

// parseTransStat reads a devfreq trans_stat transition matrix. Each data row is
// "[*]<from_freq>: <counts...> <time_ms>"; the trailing number is the ms spent in
// that frequency. Header ("From : To") and "Total transition" rows are skipped.
func parseTransStat(txt string) ([]uint64, []float64) {
	var freqs []uint64
	var ms []float64
	for _, line := range strings.Split(txt, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "*"))
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		f, err := strconv.ParseUint(strings.TrimSpace(line[:i]), 10, 64)
		if err != nil {
			continue
		}
		nums := strings.Fields(line[i+1:])
		if len(nums) == 0 {
			continue
		}
		t, err := strconv.ParseFloat(nums[len(nums)-1], 64)
		if err != nil {
			continue
		}
		freqs = append(freqs, f)
		ms = append(ms, t)
	}
	return freqs, ms
}

func libVersion() string {
	b, err := os.ReadFile(libRknnPath)
	if err != nil {
		return ""
	}
	const tag = "librknnrt version:"
	i := bytes.Index(b, []byte(tag))
	if i < 0 {
		return ""
	}
	j := i + len(tag)
	end := j
	for end < len(b) && b[end] >= 0x20 {
		end++
	}
	s := string(b[j:end])
	s = strings.ReplaceAll(s, "(", "\n")
	s = strings.ReplaceAll(s, ")", "")
	return strings.TrimSpace(s)
}

type cpuTimes struct{ total, idle uint64 }

var prevCPU []cpuTimes

func cpuPercents() []float64 {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil
	}
	var cur []cpuTimes
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "cpu") || strings.HasPrefix(line, "cpu ") {
			continue
		}
		var total, idle uint64
		for i, s := range strings.Fields(line)[1:] {
			v, _ := strconv.ParseUint(s, 10, 64)
			total += v
			if i == 3 || i == 4 {
				idle += v
			}
		}
		cur = append(cur, cpuTimes{total, idle})
	}
	res := make([]float64, len(cur))
	if len(prevCPU) == len(cur) {
		for i, c := range cur {
			dt := c.total - prevCPU[i].total
			di := c.idle - prevCPU[i].idle
			if dt > 0 {
				res[i] = float64(dt-di) / float64(dt) * 100
			}
		}
	}
	prevCPU = cur
	return res
}

func memSwap() (memU, memT, swpU, swpT uint64) {
	kv := map[string]uint64{}
	b, _ := os.ReadFile("/proc/meminfo")
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 {
			v, _ := strconv.ParseUint(f[1], 10, 64)
			kv[strings.TrimSuffix(f[0], ":")] = v
		}
	}
	memT = kv["MemTotal"] / 1024
	memU = (kv["MemTotal"] - kv["MemAvailable"]) / 1024
	swpT = kv["SwapTotal"] / 1024
	swpU = (kv["SwapTotal"] - kv["SwapFree"]) / 1024
	return
}

type thermal struct {
	label string
	temp  float64
	high  float64
}

func thermals() []thermal {
	var res []thermal
	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	for _, z := range zones {
		t, err := readFloat(filepath.Join(z, "temp"))
		if err != nil {
			continue
		}
		high, err := readFloat(filepath.Join(z, "critical_temp"))
		if err != nil {
			high, err = readFloat(filepath.Join(z, "trip_point_0_temp"))
		}
		if err != nil {
			high = 100000
		}
		label := filepath.Base(z)
		if b, err := os.ReadFile(filepath.Join(z, "type")); err == nil {
			label = strings.TrimSpace(string(b))
		}
		label = strings.TrimSuffix(label, "_thermal")
		res = append(res, thermal{label, math.Floor(t / 1000), math.Floor(high / 1000)})
	}
	return res
}
