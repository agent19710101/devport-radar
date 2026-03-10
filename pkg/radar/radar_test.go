package radar

import (
	"slices"
	"testing"
)

func TestParsePort(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{in: "127.0.0.1:8080", want: 8080},
		{in: "*:5432", want: 5432},
		{in: "[::1]:3000", want: 3000},
		{in: "invalid", want: 0},
	}
	for _, tc := range tests {
		if got := parsePort(tc.in); got != tc.want {
			t.Fatalf("parsePort(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseSSLine(t *testing.T) {
	line := "LISTEN 0 4096 127.0.0.1:3000 0.0.0.0:* users:((\"node\",pid=12345,fd=20))"
	svc, ok := parseSSLine(line)
	if !ok {
		t.Fatalf("parseSSLine should parse valid input")
	}
	if svc.Port != 3000 || svc.PID != 12345 || svc.Process != "node" {
		t.Fatalf("unexpected service: %+v", svc)
	}
}

func TestParseLsofLine(t *testing.T) {
	line := "node 12345 user 22u IPv4 0x1234 0t0 TCP 127.0.0.1:3000 (LISTEN)"
	svc, ok := parseLsofLine(line)
	if !ok {
		t.Fatalf("parseLsofLine should parse valid input")
	}
	if svc.Port != 3000 || svc.PID != 12345 || svc.Process != "node" {
		t.Fatalf("unexpected service: %+v", svc)
	}
}

func TestParseLsofOutputSkipsHeaderAndInvalidLines(t *testing.T) {
	out := []byte("COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME\n" +
		"node 12345 user 22u IPv4 0x1234 0t0 TCP 127.0.0.1:3000 (LISTEN)\n" +
		"bad-line\n")
	services := parseLsofOutput(out)
	if len(services) != 1 {
		t.Fatalf("parseLsofOutput parsed %d services, want 1", len(services))
	}
	if services[0].Port != 3000 {
		t.Fatalf("first service port = %d, want 3000", services[0].Port)
	}
}

func TestInferFingerprint(t *testing.T) {
	tests := []struct {
		name string
		s    Service
		want string
	}{
		{name: "next", s: Service{Title: "Next App"}, want: "nextjs"},
		{name: "go", s: Service{Server: "go-http"}, want: "go-service"},
		{name: "fallback", s: Service{HTTPStatus: 200}, want: "http-service"},
	}
	for _, tc := range tests {
		if got := inferFingerprint(tc.s); got != tc.want {
			t.Fatalf("inferFingerprint(%s) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestProbeHTTPSkipsWhenTimeoutDisabled(t *testing.T) {
	svc := Service{Port: 3000, Bind: "127.0.0.1:3000"}
	probeHTTP(t.Context(), &svc, 0)
	if svc.HTTPStatus != 0 || svc.Title != "" || svc.Server != "" || svc.Fingerprint != "" {
		t.Fatalf("probeHTTP should skip mutation when timeout disabled, got %+v", svc)
	}
}

func TestProbeTargetsPrefersBindHost(t *testing.T) {
	targets := probeTargets("192.168.1.9:7777", 7777)
	if len(targets) == 0 {
		t.Fatalf("expected targets")
	}
	if targets[0] != "http://192.168.1.9:7777" {
		t.Fatalf("first target = %q, want bind host target", targets[0])
	}
}

func TestProbeTargetsIncludeLoopbacksForWildcardBind(t *testing.T) {
	targets := probeTargets("*:5432", 5432)
	want := []string{
		"http://127.0.0.1:5432",
		"http://localhost:5432",
		"http://[::1]:5432",
	}
	for _, w := range want {
		if !slices.Contains(targets, w) {
			t.Fatalf("targets %v missing %q", targets, w)
		}
	}
}
