package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/agent19710101/devport-radar/pkg/radar"
)

func main() {
	var (
		jsonOut = flag.Bool("json", false, "Print JSON output")
		timeout = flag.Duration("timeout", 900*time.Millisecond, "HTTP probe timeout")
	)
	flag.Parse()

	services, err := radar.Scan(context.Background(), *timeout)
	if err != nil {
		exitf("scan failed: %v", err)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(services); err != nil {
			exitf("encode json: %v", err)
		}
		return
	}

	printTable(services)
}

func printTable(services []radar.Service) {
	if len(services) == 0 {
		fmt.Println("No listening TCP services detected.")
		return
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "PORT\tPROCESS\tPID\tHTTP\tFINGERPRINT\tTITLE")
	for _, s := range services {
		httpStatus := "-"
		if s.HTTPStatus > 0 {
			httpStatus = strconv(s.HTTPStatus)
		}
		title := compact(s.Title, 42)
		if title == "" {
			title = "-"
		}
		proc := s.Process
		if proc == "" {
			proc = "-"
		}
		fp := s.Fingerprint
		if fp == "" {
			fp = "-"
		}
		fmt.Fprintf(tw, "%d\t%s\t%d\t%s\t%s\t%s\n", s.Port, proc, s.PID, httpStatus, fp, title)
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

func strconv(v int) string {
	return fmt.Sprintf("%d", v)
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
