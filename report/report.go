package report

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gpu-cost/gpu-cost/collector"
)

type PriceConfig struct {
	GPUType      string
	PricePerHour float64
}

type JobReport struct {
	JobName         string
	Team            string
	TotalBytes      uint64
	DurationMinutes float64
	EstimatedCost   float64
	Efficiency      string
	WastedCost      float64
}

// Calculate computes cost reports for each job stat.
// Total GPU cost for the period is prorated across jobs by their byte share.
func Calculate(stats []collector.Stats, price PriceConfig, since time.Time) []JobReport {
	durationMinutes := time.Since(since).Minutes()
	totalCost := (durationMinutes / 60.0) * price.PricePerHour

	var totalBytes uint64
	for _, s := range stats {
		totalBytes += s.TotalBytes
	}

	reports := make([]JobReport, 0, len(stats))
	for _, s := range stats {
		fraction := 1.0
		if totalBytes > 0 {
			fraction = float64(s.TotalBytes) / float64(totalBytes)
		}
		jobCost := totalCost * fraction
		eff := efficiency(s.TotalBytes)
		wasted := 0.0
		if eff == "wasted" {
			wasted = jobCost
		}
		reports = append(reports, JobReport{
			JobName:         s.JobName,
			Team:            s.Team,
			TotalBytes:      s.TotalBytes,
			DurationMinutes: durationMinutes,
			EstimatedCost:   jobCost,
			Efficiency:      eff,
			WastedCost:      wasted,
		})
	}
	return reports
}

func efficiency(bytes uint64) string {
	switch {
	case bytes > 400_000_000_000:
		return "optimal"
	case bytes > 100_000_000_000:
		return "moderate"
	default:
		return "wasted"
	}
}

// Print writes the formatted report to stdout.
func Print(reports []JobReport, config PriceConfig, duration time.Duration) {
	printTo(os.Stdout, reports, config, duration)
}

func printTo(w io.Writer, reports []JobReport, config PriceConfig, duration time.Duration) {
	fmt.Fprintln(w, "GPU COST ATTRIBUTION REPORT")
	fmt.Fprintln(w, "════════════════════════════════════════════════════")
	fmt.Fprintf(w, "GPU Type: %s @ $%.2f/hr\n", config.GPUType, config.PricePerHour)
	fmt.Fprintf(w, "Monitoring Duration: %s\n", duration)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-20s %-12s %-10s %-8s %-10s\n", "JOB", "TEAM", "BYTES", "COST", "EFFICIENCY")
	fmt.Fprintln(w, "─────────────────────────────────────────────────────────────────")

	var totalBytes uint64
	var totalCost float64
	var wasted []JobReport

	for _, r := range reports {
		gb := float64(r.TotalBytes) / 1e9
		fmt.Fprintf(w, "%-20s %-12s %-10s %-8s %-10s\n",
			r.JobName,
			r.Team,
			fmt.Sprintf("%.1fGB", gb),
			fmt.Sprintf("$%.2f", r.EstimatedCost),
			r.Efficiency,
		)
		totalBytes += r.TotalBytes
		totalCost += r.EstimatedCost
		if r.Efficiency == "wasted" {
			wasted = append(wasted, r)
		}
	}

	fmt.Fprintln(w, "─────────────────────────────────────────────────────────────────")
	totalGB := float64(totalBytes) / 1e9
	fmt.Fprintf(w, "%-20s %-12s %-10s $%-7.2f\n", "TOTAL", "", fmt.Sprintf("%.1fGB", totalGB), totalCost)
	fmt.Fprintln(w)

	if len(wasted) == 0 {
		fmt.Fprintln(w, "✓ No waste detected")
		return
	}

	fmt.Fprintln(w, "⚠ WASTE ALERTS:")
	for _, r := range wasted {
		fmt.Fprintf(w, "  → %s\n", r.JobName)
		fmt.Fprintf(w, "     Wasted: $%.2f\n", r.WastedCost)
	}
}
