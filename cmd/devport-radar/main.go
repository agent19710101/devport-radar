package main

import (
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

func main() {
	var (
		jsonOut    = flag.Bool("json", false, "Print JSON output")
		timeout    = flag.Duration("timeout", 900*time.Millisecond, "HTTP probe timeout")
		watch      = flag.Bool("watch", false, "Refresh continuously and show service deltas")
		intervalS  = flag.Int("interval", 5, "Watch refresh interval in seconds")
		ports      = flag.String("ports", "", "Optional port filter list/ranges (e.g. 3000,5432,8000-8010)")
		titleWidth = flag.Int("title-width", 42, "Max title width for table output")
	)
	flag.Parse()

	filter, err := parsePortFilter(*ports)
	if err != nil {
		exitf("invalid --ports filter: %v", err)
	}

	if *watch {
		watchLoop(*intervalS, *timeout, *jsonOut, filter, *titleWidth)
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

func watchLoop(intervalS int, timeout time.Duration, jsonOut bool, filter map[int]struct{}, titleWidth int) {
	if intervalS <= 0 {
		intervalS = 5
	}
	var prev map[int]radar.Service
	for {
		services, err := radar.Scan(context.Background(), timeout)
		if err != nil {
			exitf("scan failed: %v", err)
		}
		services = filterServices(services, filter)
		current := map[int]radar.Service{}
		for _, s := range services {
			current[s.Port] = s
		}
		if prev != nil {
			printDelta(prev, current)
		}
		printServices(services, jsonOut, titleWidth)
		prev = current
		time.Sleep(time.Duration(intervalS) * time.Second)
	}
}

func printDelta(prev, current map[int]radar.Service) {
	for p := range current {
		if _, ok := prev[p]; !ok {
			fmt.Printf("+ port %d appeared\n", p)
		}
	}
	for p := range prev {
		if _, ok := current[p]; !ok {
			fmt.Printf("- port %d disappeared\n", p)
		}
	}
}

func printTable(services []radar.Service, titleWidth int) {
	if len(services) == 0 {
		fmt.Println("No listening TCP services detected.")
		return
	}
	if titleWidth < 8 {
		titleWidth = 8
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
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
