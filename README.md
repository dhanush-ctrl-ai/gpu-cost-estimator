# GPU Cost Attribution Tool

A GPU cost attribution tool that uses eBPF uprobes to track memory allocation
per process and map it to ML training jobs, generating real-time cost reports.

## How It Works

1. An eBPF program attaches to `malloc` in libc via a uprobe
2. Each allocation event (PID + bytes) is streamed to userspace via a ring buffer
3. PIDs are matched against a job registry populated from environment variables
4. Cost is calculated based on GPU hourly rate and allocation activity

## Prerequisites

```bash
sudo apt install clang llvm libbpf-dev
```

## Setup

**Step 1: Install dependencies**
```bash
sudo apt install clang llvm libbpf-dev
```

**Step 2: Compile the eBPF probe**
```bash
make build-probe
```

**Step 3: Build the binary**
```bash
make build
```

**Step 4: Run the fake workload in terminal 1**
```bash
make fake-workload
```

**Step 5: Set the env vars printed by the fake workload**
```bash
export GPU_JOB_1=pid:XXXX,name:llama-finetune,team:ml-core
export GPU_JOB_2=pid:XXXX,name:bert-ablation,team:search
export GPU_JOB_3=pid:XXXX,name:rec-model,team:recsys
```

**Step 6: Run the monitor in terminal 2**
```bash
make run-monitor
```

## Configuration

| Environment Variable   | Default | Description                        |
|------------------------|---------|------------------------------------|
| `GPU_PRICE_PER_HOUR`   | `3.20`  | Hourly GPU cost in USD             |
| `GPU_TYPE`             | `A100`  | GPU model label shown in reports   |
| `REPORT_INTERVAL_SEC`  | `30`    | Seconds between report prints      |
| `GPU_JOB_N`            | —       | Job registration (see format below)|

Job format: `pid:XXXX,name:my-job,team:my-team`

## CLI

```
gpu-cost monitor   # continuous monitoring loop
gpu-cost report    # print one report and exit
```

## Running Tests

```bash
make test
```

## Project Structure

```
gpu-cost/
├── probe/           # eBPF C code
├── collector/       # ring buffer reader + stats accumulator
├── report/          # cost calculator + terminal printer
├── jobs/            # PID → job metadata registry
├── scripts/         # fake workload simulator
├── test/            # end-to-end integration test
└── main.go          # CLI entry point
```
