package main

import (
	"reflect"
	"testing"
)

func TestParseNPU(t *testing.T) {
	multi := "Core0:  12%\nCore1:  0%\n"
	if got := parseNPU(multi); !reflect.DeepEqual(got, []int{12, 0}) {
		t.Fatalf("multi: %v", got)
	}
	single := "NPU load:  42%"
	if got := parseNPU(single); !reflect.DeepEqual(got, []int{42}) {
		t.Fatalf("single: %v", got)
	}
}

func TestParseRGA(t *testing.T) {
	txt := `num of scheduler = 3
================== load ==================
scheduler[0]: rga3_core0
         load = 0%
-----------------------------------
scheduler[1]: rga3_core1
         load = 5%
-----------------------------------
`
	names, loads := parseRGA(txt)
	if !reflect.DeepEqual(names, []string{"rga3_core0", "rga3_core1"}) || !reflect.DeepEqual(loads, []float64{0, 5}) {
		t.Fatalf("got %v %v", names, loads)
	}
}

func TestParseGPUMem(t *testing.T) {
	txt := `<dev>                <pages>
mali0                      3072
  <kctx>                  <comm>               <pid>         <pages>
  0x7f0000                chrome               1234          2048
  0x7f0100                firefox              5678          1024
`
	total, procs := parseGPUMem(txt)
	if total != 12 { // 3072 pages x 4K = 12 MB
		t.Fatalf("total MB: %d", total)
	}
	if len(procs) != 2 {
		t.Fatalf("procs: %d", len(procs))
	}
	if procs[0].comm != "chrome" || procs[0].pid != 1234 || procs[0].memMB != 8 {
		t.Fatalf("top proc: %+v", procs[0])
	}
	if procs[1].comm != "firefox" || procs[1].memMB != 4 {
		t.Fatalf("second proc: %+v", procs[1])
	}
}

func TestParseTransStat(t *testing.T) {
	txt := `     From  :   To
           :1000000000 900000000 800000000   time(ms)
*1000000000:         0         0         1       223
  900000000:         0         0         0         0
  300000000:         2         0         0     36150
Total transition : 3
`
	freqs, ms := parseTransStat(txt)
	if !reflect.DeepEqual(freqs, []uint64{1000000000, 900000000, 300000000}) {
		t.Fatalf("freqs: %v", freqs)
	}
	if !reflect.DeepEqual(ms, []float64{223, 0, 36150}) {
		t.Fatalf("ms: %v", ms)
	}
}
