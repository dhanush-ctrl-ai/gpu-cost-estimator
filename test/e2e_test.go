package test

import (
	"testing"
	"time"

	"github.com/gpu-cost/gpu-cost/collector"
	"github.com/gpu-cost/gpu-cost/jobs"
	"github.com/gpu-cost/gpu-cost/report"
)

func TestFullPipeline(t *testing.T) {
	// 1. Create registry with 2 fake jobs
	reg := jobs.NewRegistry()
	reg.Register(1001, "llama", "ml")
	reg.Register(1002, "bert", "search")

	// 2. Create collector
	col := collector.NewCollector(reg)

	// 3. Feed fake events (bypass real eBPF)
	col.RecordEvent(1001, 4_000_000_000)
	col.RecordEvent(1001, 4_000_000_000)
	col.RecordEvent(1002, 200_000_000)

	// 4. Get stats
	stats := col.GetStats()
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(stats))
	}

	// 5. Calculate reports (1 hour ago)
	since := time.Now().Add(-time.Hour)
	priceCfg := report.PriceConfig{
		GPUType:      "A100",
		PricePerHour: 3.20,
	}
	reports := report.Calculate(stats, priceCfg, since)

	// 6. Assertions
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}

	byName := make(map[string]report.JobReport)
	for _, r := range reports {
		byName[r.JobName] = r
	}

	llama, ok := byName["llama"]
	if !ok {
		t.Fatal("llama report missing")
	}
	bert, ok := byName["bert"]
	if !ok {
		t.Fatal("bert report missing")
	}

	if llama.EstimatedCost <= bert.EstimatedCost {
		t.Errorf("expected llama cost > bert cost: llama=%.4f bert=%.4f",
			llama.EstimatedCost, bert.EstimatedCost)
	}

	if bert.Efficiency != "wasted" {
		t.Errorf("expected bert efficiency=wasted, got %q", bert.Efficiency)
	}

	if llama.TotalBytes != 8_000_000_000 {
		t.Errorf("expected llama totalBytes=8GB, got %d", llama.TotalBytes)
	}

	totalWasted := 0.0
	for _, r := range reports {
		totalWasted += r.WastedCost
	}
	if totalWasted <= 0 {
		t.Error("expected total wasted cost > 0")
	}

	// 7. Call Print — must not panic
	report.Print(reports, priceCfg, time.Hour)
}
