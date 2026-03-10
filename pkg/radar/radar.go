package radar

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	Port        int       `json:"port"`
	Protocol    string    `json:"protocol"`
	Process     string    `json:"process"`
	PID         int       `json:"pid"`
	Bind        string    `json:"bind"`
	HTTPStatus  int       `json:"http_status,omitempty"`
	Title       string    `json:"title,omitempty"`
	Server      string    `json:"server,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	ScannedAt   time.Time `json:"scanned_at"`
}

var procPattern = regexp.MustCompile(`\("([^\"]+)",pid=(\d+)`)
var titlePattern = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

func Scan(ctx context.Context, timeout time.Duration) ([]Service, error) {
	cmd := exec.CommandContext(ctx, "ss", "-ltnpH")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run ss: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	uniq := map[string]Service{}
	now := time.Now()

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		svc, ok := parseSSLine(line)
		if !ok {
			continue
		}
		svc.ScannedAt = now
		probeHTTP(ctx, &svc, timeout)
		key := fmt.Sprintf("%d:%d", svc.Port, svc.PID)
		prev, exists := uniq[key]
		if !exists || score(svc) > score(prev) {
			uniq[key] = svc
		}
	}

	services := make([]Service, 0, len(uniq))
	for _, svc := range uniq {
		services = append(services, svc)
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Port < services[j].Port })
	return services, nil
}

func parseSSLine(line string) (Service, bool) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return Service{}, false
	}
	local := fields[3]
	port := parsePort(local)
	if port == 0 {
		return Service{}, false
	}
	svc := Service{Port: port, Protocol: "tcp", Bind: local}

	m := procPattern.FindStringSubmatch(line)
	if len(m) == 3 {
		svc.Process = m[1]
		pid, _ := strconv.Atoi(m[2])
		svc.PID = pid
	}
	return svc, true
}

func parsePort(local string) int {
	if strings.HasPrefix(local, "[") {
		h, p, err := net.SplitHostPort(local)
		if err == nil && h != "" {
			port, _ := strconv.Atoi(p)
			return port
		}
	}
	idx := strings.LastIndex(local, ":")
	if idx == -1 || idx == len(local)-1 {
		return 0
	}
	port, _ := strconv.Atoi(local[idx+1:])
	return port
}

func probeHTTP(ctx context.Context, svc *Service, timeout time.Duration) {
	if svc.Port == 0 {
		return
	}

	client := &http.Client{Timeout: timeout}
	for _, url := range probeTargets(svc.Bind, svc.Port) {
		reqCtx, cancel := context.WithTimeout(ctx, timeout)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
		if err != nil {
			cancel()
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			continue
		}

		svc.HTTPStatus = resp.StatusCode
		svc.Server = resp.Header.Get("Server")

		buf := make([]byte, 4096)
		n, _ := bufio.NewReader(resp.Body).Read(buf)
		_ = resp.Body.Close()
		cancel()

		chunk := strings.TrimSpace(string(bytes.TrimSpace(buf[:n])))
		if chunk != "" {
			if t := extractTitle(chunk); t != "" {
				svc.Title = t
			}
		}
		svc.Fingerprint = inferFingerprint(*svc)
		return
	}
}

func probeTargets(bind string, port int) []string {
	hosts := make([]string, 0, 4)
	seen := make(map[string]struct{})
	addHost := func(h string) {
		h = strings.TrimSpace(h)
		if h == "" {
			return
		}
		if _, ok := seen[h]; ok {
			return
		}
		seen[h] = struct{}{}
		hosts = append(hosts, h)
	}

	host := bindHost(bind)
	switch host {
	case "", "*", "0.0.0.0", "::", "[::]":
		// wildcard/unknown bind; rely on local loopbacks.
	default:
		addHost(host)
	}

	addHost("127.0.0.1")
	addHost("localhost")
	addHost("::1")

	urls := make([]string, 0, len(hosts))
	for _, h := range hosts {
		urls = append(urls, fmt.Sprintf("http://%s", net.JoinHostPort(h, strconv.Itoa(port))))
	}
	return urls
}

func bindHost(bind string) string {
	host, _, err := net.SplitHostPort(bind)
	if err == nil {
		return strings.Trim(host, "[]")
	}
	idx := strings.LastIndex(bind, ":")
	if idx <= 0 {
		return ""
	}
	return strings.Trim(bind[:idx], "[]")
}

func extractTitle(body string) string {
	m := titlePattern.FindStringSubmatch(body)
	if len(m) != 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func inferFingerprint(s Service) string {
	text := strings.ToLower(strings.Join([]string{s.Process, s.Server, s.Title}, " "))
	switch {
	case strings.Contains(text, "vite"):
		return "vite-dev-server"
	case strings.Contains(text, "next"):
		return "nextjs"
	case strings.Contains(text, "node"):
		return "node-web"
	case strings.Contains(text, "python") || strings.Contains(text, "uvicorn"):
		return "python-web"
	case strings.Contains(text, "go"):
		return "go-service"
	case s.HTTPStatus > 0:
		return "http-service"
	default:
		return "unknown"
	}
}

func score(s Service) int {
	sc := 0
	if s.PID > 0 {
		sc += 1
	}
	if s.Process != "" {
		sc += 1
	}
	if s.HTTPStatus > 0 {
		sc += 2
	}
	if s.Title != "" {
		sc += 1
	}
	return sc
}
