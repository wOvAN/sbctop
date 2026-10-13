package main

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

var seriesCols = [5]int{34, 32, 33, 36, 35}

// seriesCol cycles 5 base hues over 8 slots; the second half (dark mode: the
// first half) renders them bright (+60).
func seriesCol(k int, dark bool) int {
	c := seriesCols[k%5]
	if dark != (k >= 5) {
		c += 60
	}
	return c
}

func colorFor(pct float64, dark bool) int {
	c := 32
	if pct >= 60 {
		c = 33
	}
	if pct >= 85 {
		c = 31
	}
	if dark {
		c += 60
	}
	return c
}

type canvas struct {
	w, h int
	grid [][]rune
	cols [][]int
}

func newCanvas(w, h int) *canvas {
	c := &canvas{w: w, h: h, grid: make([][]rune, h), cols: make([][]int, h)}
	for y := 0; y < h; y++ {
		c.grid[y] = make([]rune, w)
		c.cols[y] = make([]int, w)
		for x := 0; x < w; x++ {
			c.grid[y][x] = ' '
			c.cols[y][x] = -1
		}
	}
	return c
}

func (c *canvas) set(x, y int, r rune, col int) {
	if x < 0 || x >= c.w || y < 0 || y >= c.h {
		return
	}
	c.grid[y][x] = r
	c.cols[y][x] = col
}

func (c *canvas) text(x, y int, s string, col int) {
	for i, r := range []rune(s) {
		if x+i >= c.w-1 {
			break
		}
		c.set(x+i, y, r, col)
	}
}

// kv pairs a dimmed key with its value: {"freq", "1000MHz"}.
type kv struct{ k, v string }

// kvLine draws head plus key/value pairs separated by a two-space gap,
// dimming keys (90) and bolding values (1) so pairs scan without reading.
// Separators use the default color so they stay trimmable trailing space.
func (c *canvas) kvLine(x, y int, head string, pairs []kv) {
	if head != "" {
		c.text(x, y, head+" ", 1)
		x += len([]rune(head)) + 1
	}
	for i, p := range pairs {
		if i > 0 {
			c.text(x, y, "  ", -1)
			x += 2
		}
		c.text(x, y, p.k+" ", 90)
		x += len([]rune(p.k)) + 1
		c.text(x, y, p.v, 1)
		x += len([]rune(p.v))
	}
}

func (c *canvas) box(title string) {
	for x := 1; x < c.w-1; x++ {
		c.set(x, 0, '─', -1)
		c.set(x, c.h-1, '─', -1)
	}
	for y := 0; y < c.h; y++ {
		c.set(0, y, '│', -1)
		c.set(c.w-1, y, '│', -1)
	}
	c.set(0, 0, '┌', -1)
	c.set(c.w-1, 0, '┐', -1)
	c.set(0, c.h-1, '└', -1)
	c.set(c.w-1, c.h-1, '┘', -1)
	if title != "" {
		c.text(center(title, c.w), 0, title, 1)
	}
}

func (c *canvas) lines() []string {
	out := make([]string, c.h)
	for y := 0; y < c.h; y++ {
		var sb strings.Builder
		cur := -2
		for x := 0; x < c.w; x++ {
			col := c.cols[y][x]
			if col != cur {
				if cur >= 0 {
					sb.WriteString("\033[0m")
				}
				if col >= 0 {
					fmt.Fprintf(&sb, "\033[%dm", col)
				}
				cur = col
			}
			sb.WriteRune(c.grid[y][x])
		}
		if cur >= 0 {
			sb.WriteString("\033[0m")
		}
		out[y] = strings.TrimRight(sb.String(), " ")
	}
	return out
}

func center(s string, w int) int { return (w - len([]rune(s))) / 2 }

var spark = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// dvfsSpark renders the DVFS duty-cycle as a one-line sparkline: one bar per OPP
// frequency (ascending), height = share of total time spent at that frequency.
func dvfsSpark(opp, freqs []uint64, ms []float64) string {
	fs := opp
	if len(fs) == 0 {
		fs = freqs
	}
	if len(fs) == 0 {
		return "n/a"
	}
	fs = append([]uint64(nil), fs...)
	sort.Slice(fs, func(i, j int) bool { return fs[i] < fs[j] })
	share := map[uint64]float64{}
	var tot float64
	for i, f := range freqs {
		share[f] += ms[i]
		tot += ms[i]
	}
	var sb strings.Builder
	for _, f := range fs {
		lvl := 0
		if tot > 0 {
			lvl = int(math.Round(share[f] / tot * float64(len(spark)-1)))
		}
		sb.WriteRune(spark[lvl])
	}
	return fmt.Sprintf("%s %d-%dM", sb.String(), fs[0]/1e6, fs[len(fs)-1]/1e6)
}

// sparkRowsBlock renders one history sparkline per NPU core: "Core0  42% ▂▅▇…".
// Every core stays visible no matter how the values overlap (unlike overlaid
// line plots, where coincident series collapse into one color). Histories are
// downsampled with a per-bucket max so rendering is deterministic. Cores that
// don't fit get a "+M" overflow label, the cpuBlock way.
func sparkRowsBlock(w, h int, samples [][]int, dark bool, sub []kv) []string {
	c := newCanvas(w, h)
	c.box("NPU")
	if len(samples) == 0 || len(samples[0]) == 0 || w < 16 || h < 4 {
		return c.lines()
	}
	nCore := len(samples[0])
	hists := make([][]int, nCore)
	for _, s := range samples {
		for k := 0; k < nCore && k < len(s); k++ {
			hists[k] = append(hists[k], s[k])
		}
	}
	rowsAvail := h - 2
	if len(sub) > 0 {
		rowsAvail--
	}
	shown := min(nCore, rowsAvail)
	more := ""
	if shown < nCore {
		shown = max(rowsAvail-1, 0)
		more = fmt.Sprintf("+%d", nCore-shown)
	}
	lw := len(fmt.Sprintf("Core%d", nCore-1))
	for i := 0; i < shown; i++ {
		y := 1 + i
		col := seriesCol(i, dark)
		last := 0
		if len(hists[i]) > 0 {
			last = hists[i][len(hists[i])-1]
		}
		c.text(1, y, fmt.Sprintf("Core%-*d", lw-4, i), 90)
		c.text(1+lw+1, y, fmt.Sprintf("%3d%%", last), col)
		sx := 1 + lw + 1 + 4 + 1
		for x := sx; x < w-1; x++ {
			lvl := 0
			if len(hists[i]) > 0 {
				avail := w - 1 - sx
				a := (x - sx) * len(hists[i]) / avail
				bb := (x - sx + 1) * len(hists[i]) / avail
				if bb <= a {
					bb = a + 1
				}
				mx := 0
				for _, v := range hists[i][a:bb] {
					mx = max(mx, v)
				}
				lvl = mx * (len(spark) - 1) / 100
			}
			c.set(x, y, spark[lvl], col)
		}
	}
	if more != "" {
		c.text(1, 1+shown, more, -1)
	}
	if len(sub) > 0 {
		c.kvLine(1, h-2, "NPU:", sub)
	}
	return c.lines()
}

func drawHBars(c *canvas, labels, texts []string, vals, maxes []float64, dark bool) int {
	lw := 12
	bx := lw + 2
	bw := c.w - 2 - bx - 13
	if bw < 5 {
		bw = 5
	}
	for i := range labels {
		pct := 0.0
		if maxes[i] > 0 {
			pct = vals[i] / maxes[i] * 100
		}
		n := int(math.Round(pct / 100 * float64(bw)))
		lbl := labels[i]
		if r := []rune(lbl); len(r) > lw {
			lbl = string(r[:lw])
		}
		c.text(1, 1+i, lbl, -1)
		col := colorFor(pct, dark)
		for j := 0; j < n; j++ {
			c.set(bx+j, 1+i, '█', col)
		}
		for j := n; j < bw; j++ {
			c.set(bx+j, 1+i, '▒', 90)
		}
		c.text(bx+bw+1, 1+i, texts[i], -1)
	}
	ty := 1 + len(labels)
	c.text(bx, ty, "0", -1)
	c.text(bx+bw/2-1, ty, "50", -1)
	c.text(bx+bw-2, ty, "100", -1)
	return ty
}

func hbarsBlock(w, h int, title string, labels []string, vals, maxes []float64, unit string, dark bool) []string {
	texts := make([]string, len(labels))
	for i := range texts {
		texts[i] = fmt.Sprintf("%.0f%s", vals[i], unit)
	}
	c := newCanvas(w, h)
	c.box(title)
	drawHBars(c, labels, texts, vals, maxes, dark)
	return c.lines()
}

// drawCore renders one htop-style core row "N[<fill |><space><pct>]" at (x, y)
// within a column of width colW.
func drawCore(c *canvas, x, y, num int, numW, colW int, v float64, dark bool) {
	c.text(x, y, fmt.Sprintf("%*d", numW, num), -1)
	bx := x + numW
	c.set(bx, y, '[', -1)
	pct := fmt.Sprintf("%.1f%%", v)
	fillArea := colW - numW - 2 - len(pct)
	if fillArea < 1 {
		fillArea = 1
	}
	nf := int(math.Round(float64(fillArea) * v / 100))
	col := colorFor(v, dark)
	for j := 0; j < fillArea; j++ {
		if j < nf {
			c.set(bx+1+j, y, '|', col)
		} else {
			c.set(bx+1+j, y, ' ', -1)
		}
	}
	c.text(bx+1+fillArea, y, pct, col)
	c.set(bx+1+fillArea+len(pct), y, ']', -1)
}

// cpuBlock renders per-core load as htop-style bars "N[||||   42.0%]".
// Fill is '|', empty is space, the percentage sits inside the bracket. If the
// core count exceeds the available rows, cores split into two columns (htop way).
// sub, when non-empty, is drawn on the last inner row as "NPU:" key/value pairs.
func cpuBlock(title string, w, h int, vals []float64, dark bool, sub []kv) []string {
	c := newCanvas(w, h)
	c.box(title)
	n := len(vals)
	if n == 0 || w < 16 || h < 4 {
		return c.lines()
	}
	rowsAvail := h - 3 // last inner row is kept for the "+N" overflow label
	cols := 1
	if n > rowsAvail {
		cols = 2
	}
	perCol := (n + cols - 1) / cols
	if perCol > rowsAvail {
		perCol = rowsAvail
	}
	numW := max(len(strconv.Itoa(n-1)), 1)
	gap := 1
	colW := (w - 2 - gap*(cols-1)) / cols
	if colW < numW+8 {
		colW = numW + 8
	}
	for i := 0; i < perCol; i++ {
		for col := 0; col < cols; col++ {
			idx := col*perCol + i
			if idx >= n {
				break
			}
			x := 1 + col*(colW+gap)
			drawCore(c, x, 1+i, idx, numW, colW, vals[idx], dark)
		}
	}
	if shown := cols * perCol; shown < n {
		c.text(1, 1+perCol, fmt.Sprintf("+%d", n-shown), -1)
	}
	if len(sub) > 0 {
		c.kvLine(2, h-2, "NPU:", sub)
	}
	return c.lines()
}

func fmtSize(mb uint64) string {
	switch {
	case mb >= 1024*1024:
		return fmt.Sprintf("%.1fT", float64(mb)/1024/1024)
	case mb >= 1024:
		return fmt.Sprintf("%.1fG", float64(mb)/1024)
	default:
		return fmt.Sprintf("%dM", mb)
	}
}

func fmtMemPair(u, t uint64) string {
	switch {
	case t >= 1024*1024:
		return fmt.Sprintf("%.1f/%.1fT", float64(u)/1024/1024, float64(t)/1024/1024)
	case t >= 1024:
		return fmt.Sprintf("%.1f/%.1fG", float64(u)/1024, float64(t)/1024)
	default:
		return fmt.Sprintf("%d/%dM", u, t)
	}
}

func memBlock(w, h int, memU, memT, swpU, swpT uint64, dark bool) []string {
	c := newCanvas(w, h)
	c.box("Memory")
	drawHBars(c,
		[]string{"Memory", "Swap"},
		[]string{fmtMemPair(memU, memT), fmtMemPair(swpU, swpT)},
		[]float64{float64(memU), float64(swpU)},
		[]float64{float64(memT), float64(swpT)},
		dark)
	return c.lines()
}

func gpuBlock(w, h int, g gpuInfo, dark bool) []string {
	c := newCanvas(w, h)
	c.box("GPU Mali0")
	ty := drawHBars(c, []string{"Load"}, []string{fmt.Sprintf("%.0f%%", g.load)}, []float64{g.load}, []float64{100}, dark)
	y := ty + 1
	c.kvLine(2, y, "", []kv{
		{"Clock", fmt.Sprintf("%dMHz (%d-%d)", g.freq/1e6, g.minF/1e6, g.maxF/1e6)},
		{"gov", g.gov},
	})
	y++
	c.kvLine(2, y, "", []kv{
		{"State", g.state},
		{"busy", fmt.Sprintf("%.0f%%", g.util)},
		{"ctx", fmt.Sprintf("%d", g.ctx)},
	})
	y++
	c.kvLine(2, y, "", []kv{{"Mem", fmtSize(g.memMB)}})
	y++
	c.kvLine(2, y, "", []kv{
		{"DDR", fmt.Sprintf("%dMHz (%d-%d)", g.memFreq/1e6, g.memMinF/1e6, g.memMaxF/1e6)},
		{"gov", g.memGov},
	})
	y++
	c.kvLine(2, y, "", []kv{{"DVFS", dvfsSpark(g.opp, g.dvfsFreq, g.dvfsMs)}})
	y++
	for x := 1; x < w-1; x++ {
		c.set(x, y, '─', 90)
	}
	y++
	c.text(2, y, "Processes:", 1)
	for i := 0; i < len(g.procs) && i < 3; i++ {
		p := g.procs[i]
		if p.memMB > 0 {
			y++
			c.text(2, y, fmt.Sprintf("  %s(%d) %s", p.comm, p.pid, fmtSize(p.memMB)), -1)
		}
	}
	return c.lines()
}

// piGpuBlock renders the Raspberry Pi VideoCore GPU: a load bar (from v3d
// gpu_stats) plus the fixed core clock label. No per-process memory or DVFS —
// the VC GPU exposes neither.
func piGpuBlock(w, h int, load float64, dark bool) []string {
	c := newCanvas(w, h)
	c.box("GPU")
	ty := drawHBars(c, []string{"Load"}, []string{fmt.Sprintf("%.0f%%", load)}, []float64{load}, []float64{100}, dark)
	c.kvLine(2, ty+1, "", []kv{{"VideoCore VI", "500MHz"}})
	return c.lines()
}
