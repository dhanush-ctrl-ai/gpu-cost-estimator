package collector

import (
	"testing"

	"github.com/gpu-cost/gpu-cost/jobs"
)

func makeRegistry() *jobs.Registry {
	r := jobs.NewRegistry()
	r.Register(1001, "llama", "ml")
	r.Register(1002, "bert", "search")
	return r
}

func TestCollector_accumulates_bytes(t *testing.T) {
	c := NewCollector(makeRegistry())
	c.RecordEvent(1001, 4_000_000_000)
	c.RecordEvent(1001, 4_000_000_000)

	stats := c.GetStats()
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	if stats[0].TotalBytes != 8_000_000_000 {
		t.Fatalf("expected 8GB, got %d", stats[0].TotalBytes)
	}
	if stats[0].EventCount != 2 {
		t.Fatalf("expected 2 events, got %d", stats[0].EventCount)
	}
}

func TestCollector_unknown_pid_ignored(t *testing.T) {
	c := NewCollector(makeRegistry())
	c.RecordEvent(9999, 1_000_000)

	stats := c.GetStats()
	if len(stats) != 0 {
		t.Fatalf("expected 0 stats for unknown pid, got %d", len(stats))
	}
}

func TestCollector_multiple_pids_tracked_separately(t *testing.T) {
	c := NewCollector(makeRegistry())
	c.RecordEvent(1001, 4_000_000_000)
	c.RecordEvent(1002, 200_000_000)

	stats := c.GetStats()
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(stats))
	}

	byName := make(map[string]Stats)
	for _, s := range stats {
		byName[s.JobName] = s
	}
	if byName["llama"].TotalBytes != 4_000_000_000 {
		t.Fatalf("llama bytes wrong: %d", byName["llama"].TotalBytes)
	}
	if byName["bert"].TotalBytes != 200_000_000 {
		t.Fatalf("bert bytes wrong: %d", byName["bert"].TotalBytes)
	}
}
