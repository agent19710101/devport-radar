package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agent19710101/devport-radar/pkg/radar"
)

func TestParsePortFilter(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []int
		wantErr bool
	}{
		{name: "empty", raw: "", want: nil},
		{name: "single and range", raw: "3000,5432,8000-8002", want: []int{3000, 5432, 8000, 8001, 8002}},
		{name: "invalid range", raw: "9000-8000", wantErr: true},
		{name: "invalid port", raw: "70000", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePortFilter(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePortFilter() error = %v", err)
			}
			for _, p := range tc.want {
				if _, ok := got[p]; !ok {
					t.Fatalf("expected port %d in filter", p)
				}
			}
		})
	}
}

func TestParseProfilePreset(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []int
		wantErr bool
	}{
		{name: "empty", raw: "", want: nil},
		{name: "agent", raw: "agent", want: []int{11434, 3000, 5173}},
		{name: "web", raw: "web", want: []int{3000, 8080}},
		{name: "data", raw: "data", want: []int{5432, 6379}},
		{name: "invalid", raw: "other", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseProfilePreset(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseProfilePreset() error = %v", err)
			}
			for _, p := range tc.want {
				if _, ok := got[p]; !ok {
					t.Fatalf("expected port %d in profile", p)
				}
			}
		})
	}
}

func TestMergePortFilters(t *testing.T) {
	a := map[int]struct{}{3000: {}, 5432: {}}
	b := map[int]struct{}{8080: {}, 5432: {}}
	got := mergePortFilters(a, b)
	for _, p := range []int{3000, 5432, 8080} {
		if _, ok := got[p]; !ok {
			t.Fatalf("missing merged port %d", p)
		}
	}
}

func TestFilterServices(t *testing.T) {
	services := []radar.Service{{Port: 8080}, {Port: 3000}, {Port: 5432}}
	filter, err := parsePortFilter("3000,5432")
	if err != nil {
		t.Fatalf("parsePortFilter error: %v", err)
	}
	got := filterServices(services, filter, false)
	if len(got) != 2 {
		t.Fatalf("len(filterServices()) = %d, want 2", len(got))
	}
	if got[0].Port != 3000 || got[1].Port != 5432 {
		t.Fatalf("unexpected order/ports: %+v", got)
	}
}

func TestFilterServicesOnlyHTTP(t *testing.T) {
	services := []radar.Service{
		{Port: 8080, HTTPStatus: 200},
		{Port: 3000},
		{Port: 5432, HTTPStatus: 404},
	}
	got := filterServices(services, nil, true)
	if len(got) != 2 {
		t.Fatalf("len(filterServices()) = %d, want 2", len(got))
	}
	if got[0].Port != 5432 || got[1].Port != 8080 {
		t.Fatalf("unexpected order/ports: %+v", got)
	}
}

func TestBuildDeltaEvents(t *testing.T) {
	prev := map[string]radar.Service{
		"3000": {Port: 3000, Process: "node"},
		"5432": {Port: 5432, Process: "postgres"},
	}
	current := map[string]radar.Service{
		"3000": {Port: 3000, Process: "node"},
		"8080": {Port: 8080, Process: "go-app"},
	}

	events := buildDeltaEvents(prev, current)
	if len(events) != 2 {
		t.Fatalf("len(buildDeltaEvents()) = %d, want 2", len(events))
	}
	if events[0].Type != "appeared" || events[0].Port != 8080 {
		t.Fatalf("first event = %+v, want appeared 8080", events[0])
	}
	if events[1].Type != "disappeared" || events[1].Port != 5432 {
		t.Fatalf("second event = %+v, want disappeared 5432", events[1])
	}
	if events[0].Service == nil || events[0].Service.Process != "go-app" {
		t.Fatalf("appeared service mismatch: %+v", events[0].Service)
	}
}

func TestResolveProbeTimeout(t *testing.T) {
	base := 900 * time.Millisecond
	if got := resolveProbeTimeout(base, false); got != base {
		t.Fatalf("resolveProbeTimeout() = %v, want %v", got, base)
	}
	if got := resolveProbeTimeout(base, true); got != 0 {
		t.Fatalf("resolveProbeTimeout() = %v, want 0", got)
	}
}

func TestValidateFlagCombination(t *testing.T) {
	tests := []struct {
		name      string
		watch     bool
		jsonOut   bool
		tui       bool
		strict    bool
		detect    string
		intervalS int
		onlyHTTP  bool
		noProbe   bool
		wantErr   bool
	}{
		{name: "valid one shot", intervalS: 5},
		{name: "valid watch tui", watch: true, tui: true, intervalS: 5},
		{name: "invalid interval", intervalS: 0, wantErr: true},
		{name: "tui requires watch", tui: true, intervalS: 5, wantErr: true},
		{name: "strict requires watch", strict: true, intervalS: 5, wantErr: true},
		{name: "json conflicts with tui", watch: true, jsonOut: true, tui: true, intervalS: 5, wantErr: true},
		{name: "only-http conflicts with no-http-probe", onlyHTTP: true, noProbe: true, intervalS: 5, wantErr: true},
		{name: "watch-detect requires watch", detect: "port-process", intervalS: 5, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFlagCombination(tc.watch, tc.jsonOut, tc.tui, tc.strict, tc.detect, tc.intervalS, tc.onlyHTTP, tc.noProbe)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("validateFlagCombination() error = %v", err)
			}
		})
	}
}

func TestParseWatchDetectMode(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "default", raw: "", want: "port"},
		{name: "port", raw: "port", want: "port"},
		{name: "port process", raw: "port-process", want: "port-process"},
		{name: "invalid", raw: "pid", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseWatchDetectMode(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseWatchDetectMode() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("parseWatchDetectMode() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestServiceKeyPortProcess(t *testing.T) {
	tests := []struct {
		name string
		svc  radar.Service
		want string
	}{
		{name: "prefer pid", svc: radar.Service{Port: 8080, PID: 123, Process: "node"}, want: "8080/123"},
		{name: "fallback process", svc: radar.Service{Port: 8080, Process: "node"}, want: "8080/node"},
		{name: "fallback port", svc: radar.Service{Port: 8080}, want: "8080"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := serviceKey(tc.svc, "port-process")
			if got != tc.want {
				t.Fatalf("serviceKey() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderDashboard(t *testing.T) {
	ts := time.Date(2026, 3, 10, 9, 30, 0, 0, time.UTC)
	services := []radar.Service{{Port: 8080, Process: "dev-server", PID: 4321, HTTPStatus: 200, Fingerprint: "http-service", Title: "Dashboard"}}

	got := renderDashboard(services, 42, ts)
	for _, want := range []string{"\x1b[2J\x1b[H", "devport-radar dashboard", ts.Format(time.RFC3339), "services: 1", "Press Ctrl+C to stop."} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderDashboard() missing %q in output: %s", want, got)
		}
	}
}

func TestRenderTableGolden(t *testing.T) {
	services := []radar.Service{
		{Port: 3000, Process: "node", PID: 9123, HTTPStatus: 200, Fingerprint: "vite", Title: "Frontend Dashboard"},
		{Port: 5432, Process: "postgres", PID: 1102, Fingerprint: "postgresql"},
		{Port: 8080, HTTPStatus: 404, Fingerprint: "unknown", Title: "A very long service title that should be truncated"},
	}

	got := renderTable(services, 20)
	goldenPath := filepath.Join("testdata", "table.golden")
	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := string(wantBytes)
	if got != want {
		t.Fatalf("renderTable() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestWatchLoopCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticks := make(chan time.Time)
	scans := 0
	emits := 0

	scan := func(context.Context, time.Duration) ([]radar.Service, error) {
		scans++
		return []radar.Service{{Port: 8080, PID: scans, Process: "app"}}, nil
	}
	emit := func(_, _ map[string]radar.Service, _ []radar.Service) {
		emits++
		if emits == 2 {
			cancel()
		}
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- watchLoop(ctx, 5, time.Second, false, false, false, nil, false, 42, "port", scan, nil, ticks, emit)
	}()

	ticks <- time.Now()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("watchLoop() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watchLoop did not stop after cancellation")
	}

	if scans != 2 {
		t.Fatalf("scan count = %d, want 2", scans)
	}
	if emits != 2 {
		t.Fatalf("emit count = %d, want 2", emits)
	}
}

func TestWatchLoopContinuesAfterTransientScanError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticks := make(chan time.Time)
	scans := 0
	emits := 0

	scan := func(context.Context, time.Duration) ([]radar.Service, error) {
		scans++
		if scans == 2 {
			return nil, context.DeadlineExceeded
		}
		return []radar.Service{{Port: 8080, PID: scans, Process: "app"}}, nil
	}
	emit := func(_, _ map[string]radar.Service, _ []radar.Service) {
		emits++
		if emits == 2 {
			cancel()
		}
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- watchLoop(ctx, 5, time.Second, false, false, false, nil, false, 42, "port", scan, nil, ticks, emit)
	}()

	ticks <- time.Now()
	ticks <- time.Now()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("watchLoop() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watchLoop did not stop after cancellation")
	}

	if scans != 3 {
		t.Fatalf("scan count = %d, want 3", scans)
	}
	if emits != 2 {
		t.Fatalf("emit count = %d, want 2", emits)
	}
}

func TestWatchLoopStrictModePropagatesScanError(t *testing.T) {
	scan := func(context.Context, time.Duration) ([]radar.Service, error) {
		return nil, context.DeadlineExceeded
	}
	err := watchLoop(context.Background(), 5, time.Second, false, false, true, nil, false, 42, "port", scan, nil, make(chan time.Time), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestJSONSnapshotContractGolden(t *testing.T) {
	services := []radar.Service{{
		Port:        8080,
		Protocol:    "tcp",
		Bind:        "127.0.0.1",
		Process:     "dev-server",
		PID:         4321,
		HTTPStatus:  200,
		Server:      "Caddy",
		Title:       "Dashboard",
		Fingerprint: "http-service",
		ScannedAt:   time.Date(2026, 3, 10, 7, 30, 0, 0, time.UTC),
	}}

	got := captureStdout(t, func() { printServices(services, true, 42) })
	want := readGolden(t, filepath.Join("testdata", "services.json.golden"))
	if got != want {
		t.Fatalf("json contract mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestWatchNDJSONContractGolden(t *testing.T) {
	ts := time.Date(2026, 3, 10, 7, 30, 0, 0, time.UTC)
	svc := radar.Service{Port: 8080, Process: "dev-server", PID: 4321}

	got := captureStdout(t, func() {
		printJSON(watchEvent{Type: "appeared", Port: 8080, Service: &svc, Timestamp: ts})
		printJSON(watchEvent{Type: "snapshot", Services: []radar.Service{svc}, Timestamp: ts})
		printJSON(watchEvent{Type: "error", Error: "scan timeout", Timestamp: ts})
	})
	want := readGolden(t, filepath.Join("testdata", "watch-events.ndjson.golden"))
	if got != want {
		t.Fatalf("ndjson contract mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("invalid ndjson line %q: %v", line, err)
		}
		if _, ok := obj["type"]; !ok {
			t.Fatalf("event missing type: %q", line)
		}
		if _, ok := obj["timestamp"]; !ok {
			t.Fatalf("event missing timestamp: %q", line)
		}
	}
}

func TestApplyAliases(t *testing.T) {
	services := []radar.Service{{Port: 3000, Process: "web"}, {Port: 5432, Process: "postgres"}}
	got := applyAliases(services, map[int]string{3000: "frontend"})
	if got[0].Alias != "frontend" {
		t.Fatalf("alias not applied: %+v", got[0])
	}
	if got[1].Alias != "" {
		t.Fatalf("unexpected alias on unmatched service: %+v", got[1])
	}
}

func TestLoadAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "aliases.txt")
	if err := os.WriteFile(path, []byte("# comment\n3000=frontend\n5432=postgres-db\n"), 0o644); err != nil {
		t.Fatalf("write aliases file: %v", err)
	}
	aliases, err := loadAliases(path)
	if err != nil {
		t.Fatalf("loadAliases() error = %v", err)
	}
	if aliases[3000] != "frontend" || aliases[5432] != "postgres-db" {
		t.Fatalf("unexpected aliases: %+v", aliases)
	}
}

func TestRenderPrometheusMetrics(t *testing.T) {
	metrics := renderPrometheusMetrics([]radar.Service{{Port: 3000, Process: "web", Fingerprint: "vite-dev-server", Alias: "frontend", HTTPStatus: 200}})
	for _, want := range []string{"# TYPE devport_radar_service_http_status gauge", "devport_radar_services_total 1", "devport_radar_service_up{port=\"3000\"", "alias=\"frontend\"", "devport_radar_service_http_status{"} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("renderPrometheusMetrics() missing %q in %s", want, metrics)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	return buf.String()
}

func readGolden(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	return string(b)
}
