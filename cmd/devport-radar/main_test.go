package main

import (
	"testing"

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

func TestFilterServices(t *testing.T) {
	services := []radar.Service{{Port: 8080}, {Port: 3000}, {Port: 5432}}
	filter, err := parsePortFilter("3000,5432")
	if err != nil {
		t.Fatalf("parsePortFilter error: %v", err)
	}
	got := filterServices(services, filter)
	if len(got) != 2 {
		t.Fatalf("len(filterServices()) = %d, want 2", len(got))
	}
	if got[0].Port != 3000 || got[1].Port != 5432 {
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
