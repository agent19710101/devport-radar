package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/agent19710101/devport-radar/pkg/radar"
)

type watchEvent struct {
	Type      string          `json:"type"`
	Port      int             `json:"port,omitempty"`
	Service   *radar.Service  `json:"service,omitempty"`
	Services  []radar.Service `json:"services,omitempty"`
	Error     string          `json:"error,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

type scanFn func(context.Context, time.Duration) ([]radar.Service, error)
type emitFn func(prev, current map[string]radar.Service, services []radar.Service)

func main() {
	var (
		jsonOut     = flag.Bool("json", false, "Print JSON output")
		timeout     = flag.Duration("timeout", 900*time.Millisecond, "HTTP probe timeout")
		watch       = flag.Bool("watch", false, "Refresh continuously and show service deltas")
		intervalS   = flag.Int("interval", 5, "Watch refresh interval in seconds")
		watchDetect = flag.String("watch-detect", "port", "Change detection mode: port or port-process")
		watchStrict = flag.Bool("watch-strict", false, "Fail fast in watch mode on first scan error")
		ports       = flag.String("ports", "", "Optional port filter list/ranges (e.g. 3000,5432,8000-8010)")
		profile     = flag.String("profile", "", "Optional scan profile preset: agent, web, data")
		titleWidth  = flag.Int("title-width", 42, "Max title width for table output")
		onlyHTTP    = flag.Bool("only-http", false, "Show only services with successful HTTP probe response")
		noHTTPProbe = flag.Bool("no-http-probe", false, "Disable HTTP probing for faster port/process-only scans")
		tui         = flag.Bool("tui", false, "Render watch output as a minimal terminal dashboard")
		metricsAddr = flag.String("metrics-addr", "", "Serve Prometheus metrics at the given listen address (e.g. :9317)")
		metricsPath = flag.String("metrics-path", "/metrics", "HTTP path for Prometheus metrics endpoint")
		aliasesFile = flag.String("aliases-file", "", "Optional port alias file (format: <port>=<alias>)")
	)
	flag.Parse()

	filter, err := parsePortFilter(*ports)
	if err != nil {
		exitf("invalid --ports filter: %v", err)
	}
	profileFilter, err := parseProfilePreset(*profile)
	if err != nil {
		exitf("invalid --profile: %v", err)
	}
	filter = mergePortFilters(filter, profileFilter)

	detectMode, err := parseWatchDetectMode(*watchDetect)
	if err != nil {
		exitf("invalid --watch-detect: %v", err)
	}
	if err := validateFlagCombination(*watch, *jsonOut, *tui, *watchStrict, *watchDetect, *intervalS, *onlyHTTP, *noHTTPProbe); err != nil {
		exitf("invalid flag combination: %v", err)
	}

	aliases, err := loadAliases(*aliasesFile)
	if err != nil {
		exitf("load aliases: %v", err)
	}

	probeTimeout := resolveProbeTimeout(*timeout, *noHTTPProbe)

	ctx := context.Background()
	if strings.TrimSpace(*metricsAddr) != "" {
		if err := runMetricsServer(ctx, *metricsAddr, *metricsPath, probeTimeout, filter, *onlyHTTP, aliases); err != nil {
			exitf("metrics server failed: %v", err)
		}
		return
	}
	if *watch {
		watchCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := watchLoop(watchCtx, *intervalS, probeTimeout, *jsonOut, *tui, *watchStrict, filter, *onlyHTTP, *titleWidth, detectMode, radar.Scan, aliases, nil, nil); err != nil {
			exitf("watch failed: %v", err)
		}
		return
	}

	services, err := radar.Scan(ctx, probeTimeout)
	if err != nil {
		exitf("scan failed: %v", err)
	}
	services = filterServices(services, filter, *onlyHTTP)
	services = applyAliases(services, aliases)
	printServices(services, *jsonOut, *titleWidth)
}

func printServices(services []radar.Service, jsonOut bool, titleWidth int) {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(services); err != nil {
			exitf("encode json: %v", err)
		}
		return
	}
	printTable(services, titleWidth)
}

func watchLoop(
	ctx context.Context,
	intervalS int,
	timeout time.Duration,
	jsonOut bool,
	tui bool,
	watchStrict bool,
	filter map[int]struct{},
	onlyHTTP bool,
	titleWidth int,
	detectMode string,
	scan scanFn,
	aliases map[int]string,
	tick <-chan time.Time,
	emit emitFn,
) error {
	if intervalS <= 0 {
		intervalS = 5
	}
	if scan == nil {
		scan = radar.Scan
	}
	if emit == nil {
		emit = func(prev, current map[string]radar.Service, services []radar.Service) {
			if jsonOut {
				if prev != nil {
					printDeltaJSON(prev, current)
				}
				printWatchSnapshotJSON(services)
				return
			}
			if tui {
				fmt.Fprint(os.Stdout, renderDashboard(services, titleWidth, time.Now()))
				return
			}
			if prev != nil {
				printDelta(prev, current)
			}
			printTable(services, titleWidth)
		}
	}

	if err := ctx.Err(); err != nil {
		return nil
	}

	var ticker *time.Ticker
	if tick == nil {
		ticker = time.NewTicker(time.Duration(intervalS) * time.Second)
		defer ticker.Stop()
		tick = ticker.C
	}

	var prev map[string]radar.Service
	runOnce := func() error {
		services, err := scan(ctx, timeout)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return nil
			}
			if watchStrict {
				return fmt.Errorf("scan failed: %w", err)
			}
			if jsonOut {
				printWatchErrorJSON(err)
			} else {
				fmt.Fprintf(os.Stderr, "watch scan error: %v\n", err)
			}
			return nil
		}
		services = filterServices(services, filter, onlyHTTP)
		services = applyAliases(services, aliases)
		current := serviceIndex(services, detectMode)
		emit(prev, current, services)
		prev = current
		return nil
	}

	if err := runOnce(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick:
			if err := runOnce(); err != nil {
				return err
			}
		}
	}
}

func printDelta(prev, current map[string]radar.Service) {
	for _, ev := range buildDeltaEvents(prev, current) {
		if ev.Type == "appeared" {
			fmt.Printf("+ port %d appeared\n", ev.Port)
			continue
		}
		fmt.Printf("- port %d disappeared\n", ev.Port)
	}
}

func printDeltaJSON(prev, current map[string]radar.Service) {
	events := buildDeltaEvents(prev, current)
	for _, ev := range events {
		printJSON(ev)
	}
}

func printWatchSnapshotJSON(services []radar.Service) {
	printJSON(watchEvent{Type: "snapshot", Services: services, Timestamp: time.Now()})
}

func printWatchErrorJSON(err error) {
	printJSON(watchEvent{Type: "error", Error: err.Error(), Timestamp: time.Now()})
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(v); err != nil {
		exitf("encode json: %v", err)
	}
}

func buildDeltaEvents(prev, current map[string]radar.Service) []watchEvent {
	events := make([]watchEvent, 0)
	for k, s := range current {
		if _, ok := prev[k]; !ok {
			svc := s
			events = append(events, watchEvent{Type: "appeared", Port: s.Port, Service: &svc, Timestamp: time.Now()})
		}
	}
	for k, s := range prev {
		if _, ok := current[k]; !ok {
			svc := s
			events = append(events, watchEvent{Type: "disappeared", Port: s.Port, Service: &svc, Timestamp: time.Now()})
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Type == events[j].Type {
			if events[i].Port == events[j].Port {
				return events[i].Service.PID < events[j].Service.PID
			}
			return events[i].Port < events[j].Port
		}
		return events[i].Type < events[j].Type
	})
	return events
}

func printTable(services []radar.Service, titleWidth int) {
	fmt.Fprint(os.Stdout, renderTable(services, titleWidth))
}

func renderTable(services []radar.Service, titleWidth int) string {
	if len(services) == 0 {
		return "No listening TCP services detected.\n"
	}
	if titleWidth < 8 {
		titleWidth = 8
	}

	var b bytes.Buffer
	tw := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "PORT\tALIAS\tPROCESS\tPID\tHTTP\tFINGERPRINT\tTITLE")
	for _, s := range services {
		httpStatus := "-"
		if s.HTTPStatus > 0 {
			httpStatus = intString(s.HTTPStatus)
		}
		title := compact(s.Title, titleWidth)
		if title == "" {
			title = "-"
		}
		alias := strings.TrimSpace(s.Alias)
		if alias == "" {
			alias = "-"
		}
		proc := s.Process
		if proc == "" {
			proc = "-"
		}
		pid := "-"
		if s.PID > 0 {
			pid = intString(s.PID)
		}
		fp := s.Fingerprint
		if fp == "" || fp == "unknown" {
			fp = "-"
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n", s.Port, alias, proc, pid, httpStatus, fp, title)
	}
	_ = tw.Flush()
	return b.String()
}

func renderDashboard(services []radar.Service, titleWidth int, now time.Time) string {
	var b bytes.Buffer
	b.WriteString("\x1b[2J\x1b[H")
	fmt.Fprintf(&b, "devport-radar dashboard  |  %s\n", now.Format(time.RFC3339))
	fmt.Fprintf(&b, "services: %d\n\n", len(services))
	b.WriteString(renderTable(services, titleWidth))
	b.WriteString("\nPress Ctrl+C to stop.\n")
	return b.String()
}

func compact(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func intString(v int) string {
	return fmt.Sprintf("%d", v)
}

func resolveProbeTimeout(timeout time.Duration, noHTTPProbe bool) time.Duration {
	if noHTTPProbe {
		return 0
	}
	return timeout
}

func validateFlagCombination(watch, jsonOut, tui, watchStrict bool, watchDetect string, intervalS int, onlyHTTP, noHTTPProbe bool) error {
	if intervalS <= 0 {
		return errors.New("--interval must be > 0")
	}
	if tui && !watch {
		return errors.New("--tui requires --watch")
	}
	if watchStrict && !watch {
		return errors.New("--watch-strict requires --watch")
	}
	if jsonOut && tui {
		return errors.New("--tui cannot be combined with --json")
	}
	if onlyHTTP && noHTTPProbe {
		return errors.New("--only-http cannot be combined with --no-http-probe")
	}
	normalizedDetect := strings.TrimSpace(strings.ToLower(watchDetect))
	if !watch && normalizedDetect != "" && normalizedDetect != "port" {
		return errors.New("--watch-detect requires --watch")
	}
	return nil
}

func parseWatchDetectMode(raw string) (string, error) {
	mode := strings.TrimSpace(strings.ToLower(raw))
	switch mode {
	case "", "port":
		return "port", nil
	case "port-process":
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported mode %q (use port or port-process)", raw)
	}
}

func serviceIndex(services []radar.Service, detectMode string) map[string]radar.Service {
	idx := make(map[string]radar.Service, len(services))
	for _, s := range services {
		idx[serviceKey(s, detectMode)] = s
	}
	return idx
}

func serviceKey(s radar.Service, detectMode string) string {
	if detectMode == "port-process" {
		if s.PID > 0 {
			return fmt.Sprintf("%d/%d", s.Port, s.PID)
		}
		if s.Process != "" {
			return fmt.Sprintf("%d/%s", s.Port, s.Process)
		}
	}
	return intString(s.Port)
}

func parseProfilePreset(raw string) (map[int]struct{}, error) {
	preset := strings.TrimSpace(strings.ToLower(raw))
	if preset == "" {
		return nil, nil
	}

	toFilter := func(ports []int) map[int]struct{} {
		filter := make(map[int]struct{}, len(ports))
		for _, p := range ports {
			filter[p] = struct{}{}
		}
		return filter
	}

	switch preset {
	case "agent":
		return toFilter([]int{11434, 3000, 5173, 6333, 6379, 8080, 8000}), nil
	case "web":
		return toFilter([]int{3000, 5173, 8000, 8080, 8081, 8888}), nil
	case "data":
		return toFilter([]int{5432, 6379, 27017, 3306, 9200, 6333}), nil
	default:
		return nil, fmt.Errorf("unsupported preset %q (use agent, web, or data)", raw)
	}
}

func mergePortFilters(primary, secondary map[int]struct{}) map[int]struct{} {
	if len(primary) == 0 {
		return secondary
	}
	if len(secondary) == 0 {
		return primary
	}
	merged := make(map[int]struct{}, len(primary)+len(secondary))
	for p := range primary {
		merged[p] = struct{}{}
	}
	for p := range secondary {
		merged[p] = struct{}{}
	}
	return merged
}

func parsePortFilter(raw string) (map[int]struct{}, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	ports := map[int]struct{}{}
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errors.New("empty port segment")
		}
		if strings.Contains(part, "-") {
			bounds := strings.Split(part, "-")
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid range %q", part)
			}
			start, err := parseValidPort(bounds[0])
			if err != nil {
				return nil, err
			}
			end, err := parseValidPort(bounds[1])
			if err != nil {
				return nil, err
			}
			if end < start {
				return nil, fmt.Errorf("invalid range %q: end before start", part)
			}
			for p := start; p <= end; p++ {
				ports[p] = struct{}{}
			}
			continue
		}
		p, err := parseValidPort(part)
		if err != nil {
			return nil, err
		}
		ports[p] = struct{}{}
	}
	return ports, nil
}

func parseValidPort(raw string) (int, error) {
	p, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid port %q", raw)
	}
	if p < 1 || p > 65535 {
		return 0, fmt.Errorf("port out of range %d", p)
	}
	return p, nil
}

func filterServices(services []radar.Service, filter map[int]struct{}, onlyHTTP bool) []radar.Service {
	if len(filter) == 0 && !onlyHTTP {
		return services
	}
	out := make([]radar.Service, 0, len(services))
	for _, s := range services {
		if len(filter) > 0 {
			if _, ok := filter[s.Port]; !ok {
				continue
			}
		}
		if onlyHTTP && s.HTTPStatus <= 0 {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

func loadAliases(path string) (map[int]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	aliases := map[int]string{}
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("line %d: expected <port>=<alias>", lineNo)
		}
		p, err := parseValidPort(parts[0])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		alias := strings.TrimSpace(parts[1])
		if alias == "" {
			return nil, fmt.Errorf("line %d: alias cannot be empty", lineNo)
		}
		aliases[p] = alias
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return aliases, nil
}

func applyAliases(services []radar.Service, aliases map[int]string) []radar.Service {
	if len(aliases) == 0 {
		return services
	}
	out := make([]radar.Service, 0, len(services))
	for _, s := range services {
		if alias, ok := aliases[s.Port]; ok {
			s.Alias = alias
		}
		out = append(out, s)
	}
	return out
}

func runMetricsServer(ctx context.Context, addr, path string, timeout time.Duration, filter map[int]struct{}, onlyHTTP bool, aliases map[int]string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/metrics"
	}
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		services, err := radar.Scan(r.Context(), timeout)
		if err != nil {
			http.Error(w, fmt.Sprintf("scan failed: %v", err), http.StatusServiceUnavailable)
			return
		}
		services = filterServices(services, filter, onlyHTTP)
		services = applyAliases(services, aliases)
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(renderPrometheusMetrics(services)))
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(os.Stderr, "serving metrics on http://%s%s\n", addr, path)
	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func renderPrometheusMetrics(services []radar.Service) string {
	var b strings.Builder
	b.WriteString("# HELP devport_radar_services_total Number of detected services.\n")
	b.WriteString("# TYPE devport_radar_services_total gauge\n")
	b.WriteString(fmt.Sprintf("devport_radar_services_total %d\n", len(services)))
	b.WriteString("# HELP devport_radar_service_up Service detection marker (always 1 when present).\n")
	b.WriteString("# TYPE devport_radar_service_up gauge\n")
	b.WriteString("# HELP devport_radar_service_http_status Last successful HTTP status code observed for a service.\n")
	b.WriteString("# TYPE devport_radar_service_http_status gauge\n")
	for _, s := range services {
		labels := fmt.Sprintf("port=\"%d\",process=\"%s\",fingerprint=\"%s\",alias=\"%s\"", s.Port, esc(s.Process), esc(s.Fingerprint), esc(s.Alias))
		b.WriteString(fmt.Sprintf("devport_radar_service_up{%s} 1\n", labels))
		if s.HTTPStatus > 0 {
			b.WriteString(fmt.Sprintf("devport_radar_service_http_status{%s} %d\n", labels, s.HTTPStatus))
		}
	}
	return b.String()
}

func esc(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
