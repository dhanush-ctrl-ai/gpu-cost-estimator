package collector

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/cilium/ebpf/ringbuf"
	"github.com/gpu-cost/gpu-cost/jobs"
)

type Stats struct {
	JobName    string
	Team       string
	TotalBytes uint64
	EventCount uint64
	FirstSeen  time.Time
	LastSeen   time.Time
}

type Collector struct {
	mu       sync.RWMutex
	stats    map[uint32]*Stats
	registry *jobs.Registry
}

func NewCollector(registry *jobs.Registry) *Collector {
	return &Collector{
		stats:    make(map[uint32]*Stats),
		registry: registry,
	}
}

// bpfObjects is the interface we need from the generated eBPF objects.
type bpfObjectsIface interface {
	GetMap(name string) ringbufMap
}

type ringbufMap interface{}

// gpuEvent mirrors the C struct layout.
type gpuEvent struct {
	PID       uint32
	_         [4]byte // padding to align u64
	Bytes     uint64
	Timestamp uint64
}

// Start opens the ring buffer from bpfObjects and begins reading events.
// bpfObjects must have a field or method exposing the "events" ringbuf map.
// We accept interface{} so callers can pass the generated ebpf.Objects struct
// without a compile-time import of generated code.
func (c *Collector) Start(ctx context.Context, bpfObjects interface{}) error {
	// bpfObjects must be a pointer to a struct with an "Events" field of type *ebpf.Map.
	// Use reflection-free approach: require caller to pass *ringbuf.Reader directly
	// via a wrapper, OR accept *ringbuf.Reader as bpfObjects directly for testability.
	var rd *ringbuf.Reader
	switch v := bpfObjects.(type) {
	case *ringbuf.Reader:
		rd = v
	case RingbufOpener:
		var err error
		rd, err = v.OpenRingbuf()
		if err != nil {
			return fmt.Errorf("open ring buffer: %w", err)
		}
	default:
		return fmt.Errorf("bpfObjects must implement RingbufOpener or be *ringbuf.Reader")
	}

	go c.readLoop(ctx, rd)
	return nil
}

// RingbufOpener is implemented by types that can open the eBPF ring buffer.
type RingbufOpener interface {
	OpenRingbuf() (*ringbuf.Reader, error)
}

func (c *Collector) readLoop(ctx context.Context, rd *ringbuf.Reader) {
	defer rd.Close()

	done := ctx.Done()
	for {
		select {
		case <-done:
			return
		default:
		}

		record, err := rd.Read()
		if err != nil {
			select {
			case <-done:
				return
			default:
				continue
			}
		}
		c.processRecord(record.RawSample)
	}
}

func (c *Collector) processRecord(raw []byte) {
	if len(raw) < int(unsafe.Sizeof(gpuEvent{})) {
		return
	}
	var evt gpuEvent
	if err := binary.Read(bytes.NewReader(raw), binary.LittleEndian, &evt); err != nil {
		return
	}
	c.RecordEvent(evt.PID, evt.Bytes)
}

// RecordEvent is exported so tests can inject fake events without real eBPF.
func (c *Collector) RecordEvent(pid uint32, bytes uint64) {
	job, ok := c.registry.Lookup(pid)
	if !ok {
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	s, exists := c.stats[pid]
	if !exists {
		s = &Stats{
			JobName:   job.Name,
			Team:      job.Team,
			FirstSeen: now,
		}
		c.stats[pid] = s
	}
	s.TotalBytes += bytes
	s.EventCount++
	s.LastSeen = now
}

// GetStats returns a copy of all accumulated stats.
func (c *Collector) GetStats() []Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Stats, 0, len(c.stats))
	for _, s := range c.stats {
		cp := *s
		out = append(out, cp)
	}
	return out
}

// Reset clears all accumulated stats.
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats = make(map[uint32]*Stats)
}
