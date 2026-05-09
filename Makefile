BINARY   = gpu-cost
PROBE_SRC = probe/probe.c
PROBE_OBJ = probe/probe.o

.PHONY: all build build-probe test run-monitor fake-workload clean help

all: build-probe build

build-probe: ## Compile the eBPF probe with clang
	clang -O2 -target bpf -c $(PROBE_SRC) -o $(PROBE_OBJ)

build: ## Build the Go binary
	go build -o $(BINARY) .

test: ## Run all Go tests
	go test ./... -v -count=1

run-monitor: ## Run the GPU cost monitor (requires sudo and probe.o)
	sudo GPU_PRICE_PER_HOUR=3.20 \
	     GPU_TYPE=A100 \
	     REPORT_INTERVAL_SEC=10 \
	     ./$(BINARY) monitor

fake-workload: ## Run the fake GPU workload simulator
	python3 scripts/fake_workload.py

clean: ## Remove built artifacts
	rm -f $(BINARY) $(PROBE_OBJ)

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'
