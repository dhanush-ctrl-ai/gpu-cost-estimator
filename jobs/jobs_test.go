package jobs

import (
	"os"
	"testing"
)

func TestRegister(t *testing.T) {
	r := NewRegistry()
	r.Register(1001, "llama", "ml-core")
	j, ok := r.Lookup(1001)
	if !ok {
		t.Fatal("expected job to be found")
	}
	if j.Name != "llama" || j.Team != "ml-core" || j.PID != 1001 {
		t.Fatalf("unexpected job: %+v", j)
	}
}

func TestLookup_found(t *testing.T) {
	r := NewRegistry()
	r.Register(42, "bert", "search")
	j, ok := r.Lookup(42)
	if !ok {
		t.Fatal("expected found")
	}
	if j.Name != "bert" {
		t.Fatalf("got name %q", j.Name)
	}
}

func TestLookup_notfound(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Lookup(999)
	if ok {
		t.Fatal("expected not found")
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("GPU_JOB_1", "pid:1001,name:llama-finetune,team:ml-core")
	os.Setenv("GPU_JOB_2", "pid:1002,name:bert-ablation,team:search")
	defer func() {
		os.Unsetenv("GPU_JOB_1")
		os.Unsetenv("GPU_JOB_2")
	}()

	r := LoadFromEnv()
	j1, ok := r.Lookup(1001)
	if !ok {
		t.Fatal("job 1001 not found")
	}
	if j1.Name != "llama-finetune" || j1.Team != "ml-core" {
		t.Fatalf("unexpected job1: %+v", j1)
	}
	j2, ok := r.Lookup(1002)
	if !ok {
		t.Fatal("job 1002 not found")
	}
	if j2.Name != "bert-ablation" || j2.Team != "search" {
		t.Fatalf("unexpected job2: %+v", j2)
	}
}

func TestAll(t *testing.T) {
	r := NewRegistry()
	r.Register(1, "a", "t1")
	r.Register(2, "b", "t2")
	r.Register(3, "c", "t3")
	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(all))
	}
}
