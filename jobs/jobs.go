package jobs

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Job struct {
	Name      string
	Team      string
	StartTime time.Time
	PID       uint32
}

type Registry struct {
	mu   sync.RWMutex
	jobs map[uint32]Job
}

func NewRegistry() *Registry {
	return &Registry{
		jobs: make(map[uint32]Job),
	}
}

func (r *Registry) Register(pid uint32, name, team string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[pid] = Job{
		Name:      name,
		Team:      team,
		StartTime: time.Now(),
		PID:       pid,
	}
}

func (r *Registry) Lookup(pid uint32) (Job, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	j, ok := r.jobs[pid]
	return j, ok
}

func (r *Registry) All() []Job {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Job, 0, len(r.jobs))
	for _, j := range r.jobs {
		out = append(out, j)
	}
	return out
}

func (r *Registry) Unregister(pid uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.jobs, pid)
}

// LoadFromEnv reads GPU_JOB_N env vars of the form:
//
//	GPU_JOB_1=pid:1001,name:llama-finetune,team:ml-core
func LoadFromEnv() *Registry {
	reg := NewRegistry()
	for i := 1; ; i++ {
		val := os.Getenv(fmt.Sprintf("GPU_JOB_%d", i))
		if val == "" {
			break
		}
		pid, name, team, err := parseJobEnv(val)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: invalid GPU_JOB_%d: %v\n", i, err)
			continue
		}
		reg.Register(pid, name, team)
	}
	return reg
}

func parseJobEnv(s string) (pid uint32, name, team string, err error) {
	parts := strings.Split(s, ",")
	fields := make(map[string]string)
	for _, p := range parts {
		kv := strings.SplitN(p, ":", 2)
		if len(kv) != 2 {
			return 0, "", "", fmt.Errorf("bad field %q", p)
		}
		fields[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	pidStr, ok := fields["pid"]
	if !ok {
		return 0, "", "", fmt.Errorf("missing pid")
	}
	p, err := strconv.ParseUint(pidStr, 10, 32)
	if err != nil {
		return 0, "", "", fmt.Errorf("bad pid %q: %w", pidStr, err)
	}
	name, ok = fields["name"]
	if !ok {
		return 0, "", "", fmt.Errorf("missing name")
	}
	team, ok = fields["team"]
	if !ok {
		return 0, "", "", fmt.Errorf("missing team")
	}
	return uint32(p), name, team, nil
}
