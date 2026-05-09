#pragma once

struct gpu_event {
    __u32 pid;
    __u64 bytes;
    __u64 timestamp;
};
