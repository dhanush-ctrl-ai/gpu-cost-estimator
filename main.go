package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/gpu-cost/gpu-cost/collector"
	"github.com/gpu-cost/gpu-cost/jobs"
	"github.com/gpu-cost/gpu-cost/report"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: gpu-cost <monitor|report>")
		os.Exit(1)
	}

	cfg := loadConfig()

	switch os.Args[1] {
	case "monitor":
		if err := runMonitor(cfg, false); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "report":
		if err := runMonitor(cfg, true); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(1)
	}
}

type config struct {
	pricePerHour   float64
	gpuType        string
	reportInterval time.Duration
}

func loadConfig() config {
	cfg := config{
		pricePerHour:   3.20,
		gpuType:        "A100",
		reportInterval: 30 * time.Second,
	}
	if v := os.Getenv("GPU_PRICE_PER_HOUR"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid GPU_PRICE_PER_HOUR %q: %v\n", v, err)
			os.Exit(1)
		}
		cfg.pricePerHour = f
	}
	if v := os.Getenv("GPU_TYPE"); v != "" {
		cfg.gpuType = v
	}
	if v := os.Getenv("REPORT_INTERVAL_SEC"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid REPORT_INTERVAL_SEC %q: %v\n", v, err)
			os.Exit(1)
		}
		cfg.reportInterval = time.Duration(n) * time.Second
	}
	return cfg
}

func runMonitor(cfg config, once bool) error {
	fmt.Println("Starting GPU Cost Monitor...")
	fmt.Println("Attach jobs using environment variables:")
	fmt.Println("  GPU_JOB_1=pid:XXXX,name:my-job,team:my-team")

	probeObj := "probe/probe.o"
	if _, err := os.Stat(probeObj); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("Error: probe.o not found.\nCompile first: clang -O2 -target bpf -c probe/probe.c -o probe/probe.o")
	}

	spec, err := ebpf.LoadCollectionSpec(probeObj)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("Error: permission denied.\neBPF requires root. Run with sudo.")
		}
		return fmt.Errorf("load collection spec: %w", err)
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("Error: permission denied.\neBPF requires root. Run with sudo.")
		}
		return fmt.Errorf("new collection: %w", err)
	}
	defer coll.Close()

	eventsMap, ok := coll.Maps["events"]
	if !ok {
		return fmt.Errorf("events map not found in probe.o")
	}

	rd, err := ringbuf.NewReader(eventsMap)
	if err != nil {
		return fmt.Errorf("create ring buffer reader: %w", err)
	}

	// Attach uprobe to malloc in libc
	ex, err := link.OpenExecutable(libcPath())
	if err != nil {
		return fmt.Errorf("open libc: %w", err)
	}

	prog, ok := coll.Programs["uprobe_malloc"]
	if !ok {
		return fmt.Errorf("uprobe_malloc program not found in probe.o")
	}

	uprobeLink, err := ex.Uprobe("malloc", prog, nil)
	if err != nil {
		return fmt.Errorf("attach uprobe: %w", err)
	}
	defer uprobeLink.Close()

	reg := jobs.LoadFromEnv()
	col := collector.NewCollector(reg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := col.Start(ctx, rd); err != nil {
		return fmt.Errorf("start collector: %w", err)
	}

	priceCfg := report.PriceConfig{
		GPUType:      cfg.gpuType,
		PricePerHour: cfg.pricePerHour,
	}
	since := time.Now()

	if once {
		time.Sleep(time.Second)
		stats := col.GetStats()
		reports := report.Calculate(stats, priceCfg, since)
		report.Print(reports, priceCfg, time.Since(since))
		return nil
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(cfg.reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sig:
			cancel()
			fmt.Println("\nShutting down...")
			stats := col.GetStats()
			reports := report.Calculate(stats, priceCfg, since)
			report.Print(reports, priceCfg, time.Since(since))
			return nil
		case <-ticker.C:
			stats := col.GetStats()
			reports := report.Calculate(stats, priceCfg, since)
			report.Print(reports, priceCfg, time.Since(since))
		}
	}
}

func libcPath() string {
	// Common libc paths; try each.
	candidates := []string{
		"/lib/x86_64-linux-gnu/libc.so.6",
		"/lib/aarch64-linux-gnu/libc.so.6",
		"/usr/lib/libc.so.6",
		"/lib/libc.so.6",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "/lib/x86_64-linux-gnu/libc.so.6"
}

