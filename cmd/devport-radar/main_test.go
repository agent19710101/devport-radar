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
