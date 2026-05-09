import ctypes
import time
import os
import multiprocessing
import sys


def simulate_job(job_name, team, alloc_size_gb, interval_sec, steps):
    pid = os.getpid()
    print(f"JOB STARTED: {job_name} | team={team} | pid={pid}", flush=True)

    buf = (ctypes.c_char * int(alloc_size_gb * 1_000_000_000))()

    for n in range(1, steps + 1):
        time.sleep(interval_sec)
        print(f"  [{job_name}] step {n}/{steps}", flush=True)

    print(f"JOB DONE: {job_name}", flush=True)
    return pid


def start_and_announce(job_name, team, alloc_size_gb, interval_sec, steps, pid_queue):
    pid_queue.put((job_name, team, os.getpid()))
    simulate_job(job_name, team, alloc_size_gb, interval_sec, steps)


if __name__ == "__main__":
    jobs = [
        ("llama-finetune", "ml-core",  4,   0.5, 20),
        ("bert-ablation",  "search",   0.2, 1.0, 10),
        ("rec-model",      "recsys",   2,   0.3, 30),
    ]

    pid_queue = multiprocessing.Queue()
    processes = []

    for job_name, team, size, interval, steps in jobs:
        p = multiprocessing.Process(
            target=start_and_announce,
            args=(job_name, team, size, interval, steps, pid_queue),
        )
        p.start()
        processes.append(p)

    # Collect PIDs (one per process, in start order)
    pids = {}
    for _ in jobs:
        job_name, team, pid = pid_queue.get()
        pids[job_name] = (pid, team)

    print("\nSet these env vars before running gpu-cost:", flush=True)
    env_idx = 1
    for job_name, team, _, _, _ in jobs:
        pid, _ = pids[job_name]
        print(f"  GPU_JOB_{env_idx}=pid:{pid},name:{job_name},team:{team}", flush=True)
        env_idx += 1
    print(flush=True)

    for p in processes:
        p.join()
