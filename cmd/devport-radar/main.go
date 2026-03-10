package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/agent19710101/devport-radar/pkg/radar"
)

type watchEvent struct {
	Type      string          `json:"type"`
	Port      int             `json:"port,omitempty"`
	Service   *radar.Service  `json:"service,omitempty"`
	Services  []radar.Service `json:"services,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

func main() {
	var (
		jsonOut    = flag.Bool("json", false, "Print JSON output")
		timeout    = flag.Duration("timeout", 900*time.Millisecond, "HTTP probe timeout")
		watch       = flag.Bool("watch", false, "Refresh continuously and show service deltas")
		intervalS   = flag.Int("interval", 5, "Watch refresh interval in seconds")
		watchDetect = flag.String("watch-detect", "port", "Change detection mode: port or port-process")
		ports       = flag.String("ports", "", "Optional port filter list/ranges (e.g. 3000,5432,8000-8010)")
		titleWidth  = flag.Int("title-width", 42, "Max title width for table output")
	)
	flag.Parse()

	filter, err := parsePortFilter(*ports)
	if err != nil {
		exitf("invalid --ports filter: %v", err)
	}

	detectMode, err := parseWatchDetectMode(*watchDetect)
	if err != nil {
		exitf("invalid --watch-detect: %v", err)
	}

	if *watch {
		watchLoop(*intervalS, *timeout, *jsonOut, filter, *titleWidth, detectMode)
		return
	}

	services, err := radar.Scan(context.Background(), *timeout)
	if err != nil {
		exitf("scan failed: %v", err)
	}
	services = filterServices(services, filter)
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

func watchLoop(intervalS int, timeout time.Duration, jsonOut bool, filter map[int]struct{}, titleWidth int, detectMode string) {
	if intervalS <= 0 {
		intervalS = 5
	}
	var prev map[string]radar.Service
	for {
		services, err := radar.Scan(context.Background(), timeout)
		if err != nil {
			exitf("scan failed: %v", err)
		}
		services = filterServices(services, filter)
		current := serviceIndex(services, detectMode)

		if jsonOut {
			if prev != nil {
				printDeltaJSON(prev, current)
			}
			printWatchSnapshotJSON(services)
		} else {
			if prev != nil {
				printDelta(prev, current)
			}
			printTable(services, titleWidth)
		}

		prev = current
		time.Sleep(time.Duration(intervalS) * time.Second)
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
	fmt.Fprintln(tw, "PORT\tPROCESS\tPID\tHTTP\tFINGERPRINT\tTITLE")
	for _, s := range services {
		httpStatus := "-"
		if s.HTTPStatus > 0 {
			httpStatus = intString(s.HTTPStatus)
		}
		title := compact(s.Title, titleWidth)
		if title == "" {
			title = "-"
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
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n", s.Port, proc, pid, httpStatus, fp, title)
	}
	_ = tw.Flush()
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

func filterServices(services []radar.Service, filter map[int]struct{}) []radar.Service {
	if len(filter) == 0 {
		return services
	}
	out := make([]radar.Service, 0, len(services))
	for _, s := range services {
		if _, ok := filter[s.Port]; ok {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
