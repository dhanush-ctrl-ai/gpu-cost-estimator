package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/gpu-cost/gpu-cost/collector"
)

func makeStats(name, team string, totalBytes uint64) collector.Stats {
	return collector.Stats{
		JobName:    name,
		Team:       team,
		TotalBytes: totalBytes,
		EventCount: 1,
		FirstSeen:  time.Now().Add(-time.Hour),
		LastSeen:   time.Now(),
	}
}

func TestCalculate_optimal_job(t *testing.T) {
	since := time.Now().Add(-time.Hour)
	stats := []collector.Stats{makeStats("llama", "ml", 500_000_000_000)}
	reports := Calculate(stats, PriceConfig{GPUType: "A100", PricePerHour: 3.20}, since)

	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	if reports[0].Efficiency != "optimal" {
		t.Fatalf("expected optimal, got %q", reports[0].Efficiency)
	}
	if reports[0].WastedCost != 0 {
		t.Fatalf("expected 0 wasted cost, got %f", reports[0].WastedCost)
	}
}

func TestCalculate_wasted_job(t *testing.T) {
	since := time.Now().Add(-time.Hour)
	stats := []collector.Stats{makeStats("tiny", "eng", 50_000_000)}
	reports := Calculate(stats, PriceConfig{GPUType: "A100", PricePerHour: 3.20}, since)

	if reports[0].Efficiency != "wasted" {
		t.Fatalf("expected wasted, got %q", reports[0].Efficiency)
	}
	if reports[0].WastedCost <= 0 {
		t.Fatalf("expected positive wasted cost, got %f", reports[0].WastedCost)
	}
}

func TestCalculate_cost_math_correct(t *testing.T) {
	// 1 hour at $3.20/hr = $3.20
	since := time.Now().Add(-time.Hour)
	stats := []collector.Stats{makeStats("job", "team", 1_000_000)}
	reports := Calculate(stats, PriceConfig{GPUType: "A100", PricePerHour: 3.20}, since)

	// Allow 1% tolerance for timing imprecision
	cost := reports[0].EstimatedCost
	if cost < 3.17 || cost > 3.23 {
		t.Fatalf("expected ~$3.20, got $%.4f", cost)
	}
}

func TestPrint_output_format(t *testing.T) {
	reports := []JobReport{
		{
			JobName:         "llama-finetune",
			Team:            "ml-core",
			TotalBytes:      500_000_000_000,
			DurationMinutes: 60,
			EstimatedCost:   3.20,
			Efficiency:      "optimal",
			WastedCost:      0,
		},
		{
			JobName:         "tiny-job",
			Team:            "eng",
			TotalBytes:      10_000_000,
			DurationMinutes: 60,
			EstimatedCost:   3.20,
			Efficiency:      "wasted",
			WastedCost:      3.20,
		},
	}
	config := PriceConfig{GPUType: "A100", PricePerHour: 3.20}

	var buf bytes.Buffer
	printTo(&buf, reports, config, time.Hour)
	out := buf.String()

	if !strings.Contains(out, "GPU COST ATTRIBUTION REPORT") {
		t.Error("missing header line")
	}
	if !strings.Contains(out, "TOTAL") {
		t.Error("missing TOTAL line")
	}
	if !strings.Contains(out, "WASTE ALERTS") {
		t.Error("missing waste alerts section")
	}
}
