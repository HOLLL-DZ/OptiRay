package main

import "testing"

func TestRouterMetricsParsing(t *testing.T) {
	s, e := parseRouterCPU("cpu 10 2 3 40 5 6 7 8 90 80\n")
	if e != nil || s.total != 81 || s.idle != 45 {
		t.Fatalf("cpu %+v %v", s, e)
	}
	total, used, e := parseRouterMemory("MemTotal: 1000 kB\nMemAvailable: 400 kB\n")
	if e != nil || total != 1000 || used != 600 {
		t.Fatalf("memory %d %d %v", total, used, e)
	}
	_, used, e = parseRouterMemory("MemTotal: 1000 kB\nMemFree: 100 kB\nBuffers: 200 kB\nCached: 300 kB\n")
	if e != nil || used != 400 {
		t.Fatal("fallback", used, e)
	}
	if _, e := parseRouterCPU("invalid"); e == nil {
		t.Fatal("invalid cpu accepted")
	}
}

func TestKeeneticModelFallback(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"vendor: Keenetic\n model: Giga (KN-1010)\n hw_id: KN-1010\n device: Giga\n", "Giga (KN-1010)"},
		{"device: Giga\nhw_id: KN-1010\n", "Giga (KN-1010)"},
		{"hw_id: KN-1010\n", "KN-1010"},
		{"vendor: Keenetic\n", ""},
	} {
		if got := parseKeeneticModel(tc.input); got != tc.want {
			t.Fatalf("model=%q want=%q", got, tc.want)
		}
	}
}
