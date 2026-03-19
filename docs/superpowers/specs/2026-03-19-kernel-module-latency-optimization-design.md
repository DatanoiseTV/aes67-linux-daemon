# Ravenna ALSA Kernel Module Latency Optimization

**Date:** 2026-03-19
**Status:** Draft
**Scope:** `3rdparty/ravenna-alsa-lkm/` — ALSA driver, timer, RTP layer

## Problem Statement

The Ravenna ALSA kernel module currently hardcodes a minimum period size of 512 frames (`DEFAULT_NADAC_TICFRAMESIZE`), resulting in a minimum latency of ~21ms at 48kHz (2 periods x 512 frames). This is far above what AES67 specifies (1ms mandatory, 125µs minimum) and what professional audio applications require.

Additional issues include sample-by-sample format conversion in the hot path, tasklet-based interrupt handling with unpredictable latency, spinlock contention on the PCM pointer path, and multiple unnecessary buffer copies.

## Goals

- Support AES67-compliant packet times: 125µs, 250µs, 333µs, 1ms (mandatory), 4ms
- Support Ravenna-native 64-frame packets (1.33ms)
- Default to AES67 mandatory 1ms (48 frames @ 48kHz)
- Work reliably on both standard `PREEMPT=full` and `PREEMPT_RT` kernels
- Target kernel 5.15+
- Minimize hot-path CPU overhead to sustain low periods under load

## Non-Goals

- DSD optimization (existing DSD paths preserved, not optimized)
- Userspace daemon changes (beyond jitter buffer configuration)
- Network stack optimization (e.g., XDP, AF_XDP)

## Architecture Overview

```
+------------------+     +-------------------+     +------------------+
| ALSA Userspace   |     | ALSA PCM Driver   |     | Ravenna Manager  |
| (JACK/Pipewire)  |<--->| (audio_driver.c)  |<--->| (C++ manager)    |
+------------------+     +-------------------+     +------------------+
                               |       ^                    |
                               v       |                    v
                          +-------------------+     +------------------+
                          | DMA Ring Buffer   |     | RTP Audio Stream |
                          | (ALSA-managed)    |     | (network I/O)    |
                          +-------------------+     +------------------+
                                                           |
                          +-------------------+            v
                          | kthread Worker    |     +------------------+
                          | (SCHED_FIFO)      |<----| hrtimer          |
                          | (module_timer.c)  |     | (hard mode)      |
                          +-------------------+     +------------------+
```

## Phase 1: AES67-Compliant Period Sizes

### Current State

- `DEFAULT_NADAC_TICFRAMESIZE = 512` hardcoded in `MergingRAVENNACommon.h:74`
- `MR_ALSA_NB_FRAMES_PER_PERIOD_AT_1FS` set to this value in `audio_driver.c:59`
- `g_supported_period_sizes[] = {512, 1024, 2048, 4096}` in `audio_driver.c:1429`
- `mr_alsa_audio_hw_rule_period_size_by_rate()` forces `t.min = t.max` — single value per rate
- The Ravenna manager already supports smaller TIC frame sizes (48, 64) via `tic_frame_size_at_1fs` config, but the ALSA driver ignores this

### Changes

#### 1.1 Remove hardcoded period size

Replace `MR_ALSA_NB_FRAMES_PER_PERIOD_AT_1FS` usage with dynamic values from the Ravenna manager's `get_min_interrupts_frame_size()` / `get_max_interrupts_frame_size()`.

#### 1.2 AES67-compliant period size list

Replace `g_supported_period_sizes[]` with:

```c
static unsigned int g_supported_period_sizes[] = {
    6, 12, 16, 48, 64, 128, 192, 384, 512
};
```

At 48kHz these correspond to:
| Frames | Packet Time | Standard |
|--------|------------|----------|
| 6      | 125µs      | AES67 optional |
| 12     | 250µs      | AES67 optional |
| 16     | 333µs      | AES67 optional |
| 48     | 1ms        | AES67 mandatory |
| 64     | 1.33ms     | Ravenna native |
| 128    | 2.67ms     | Ravenna extended |
| 192    | 4ms        | AES67 optional |
| 384    | 8ms        | Ravenna extended |
| 512    | 10.67ms    | Legacy (NADAC) |

Filter at runtime: only expose sizes between `get_min_interrupts_frame_size()` and `get_max_interrupts_frame_size()`.

#### 1.3 Rewrite hw_rule_period_size_by_rate

Change from forcing `t.min = t.max` (single value) to exposing a range:

```c
// For rate <= 48kHz: period_size in [minPTPFrameSize, maxPTPFrameSize]
// For rate <= 96kHz: period_size in [minPTPFrameSize*2, maxPTPFrameSize*2]
// etc.
```

This lets ALSA negotiate the period size with the application instead of forcing one value.

#### 1.4 Update hardware descriptors (both playback and capture)

```c
// Both mr_alsa_audio_pcm_hardware_playback and mr_alsa_audio_pcm_hardware_capture:
.period_bytes_min = 6 * 1 * 2,  // 6 frames, 1 channel, 16-bit minimum
.periods_min = 2,
```

#### 1.5 Module parameter

```c
static int tic_frame_size = 48;  // AES67 default
module_param(tic_frame_size, int, 0444);
MODULE_PARM_DESC(tic_frame_size, "TIC frame size at 1FS (6,12,16,48,64,128,192)");
```

#### 1.6 Initial timer base period

In `init_clock_timer()`, compute the initial `base_period_` from the module parameter:

```c
// For 48 frames @ 48kHz: base_period = 1,000,000 ns (1ms)
// For 6 frames @ 48kHz:  base_period = 125,000 ns (125µs)
base_period_ = (tic_frame_size * 1000000000ULL) / DEFAULT_SAMPLERATE;
```

Currently hardcoded to 1.33ms (`1333333` ns). Phase 4.1 handles runtime updates when sample rate changes.

### Files Modified

- `common/MergingRAVENNACommon.h` — remove `DEFAULT_NADAC_TICFRAMESIZE` or make it a fallback
- `driver/audio_driver.c` — period size list, hw rules, hw descriptor, module param
- `driver/module_timer.c` — dynamic base period

#### 1.7 Reliability expectations by period size

| Period Size | Packet Time | Kernel Requirement |
|-------------|------------|--------------------|
| 6 frames    | 125µs      | PREEMPT_RT + CPU isolation (`isolcpus`) |
| 12 frames   | 250µs      | PREEMPT_RT + CPU isolation |
| 16 frames   | 333µs      | PREEMPT_RT recommended |
| 48 frames   | 1ms        | Standard PREEMPT=full (general-purpose target) |
| 64 frames   | 1.33ms     | Standard PREEMPT=full |
| 128+ frames | 2.67ms+    | Any kernel |

#### 1.8 Backward compatibility / rollback

Loading the module with `tic_frame_size=512` restores the original NADAC behavior. The default (48) is a conservative AES67-compliant value that should work on all supported hardware.

## Phase 2: Hot Path Optimization

### Current State

- Playback de-interleave (`audio_driver.c:1209-1265`): per-sample loop with `switch(nb_logical_bits)` and `if(dsdmode == 0)` inside the inner loop
- Capture interleave (`audio_driver.c:1126-1173`): uses `MTConvert*` functions (e.g., `MTConvertMappedInt32ToInt32LEInterleave`) which are better but still process generically
- Ring buffer wrap check on every sample (`audio_driver.c:1255-1265`). Note: `stepOut = strideOut * nb_playback_interrupts_per_period` (line 1198), so in DSD mode the stride is larger — but DSD optimization is a non-goal

### Changes

#### 2.1 Pre-select conversion function at prepare time

Add function pointers to `mr_alsa_audio_chip`:

```c
typedef void (*copy_fn_t)(struct mr_alsa_audio_chip *chip,
                          void *dst, const void *src,
                          unsigned int channels, unsigned int frames);

copy_fn_t playback_copy_fn;
copy_fn_t capture_copy_fn;
```

Set during `mr_alsa_audio_pcm_prepare()` based on format:

```c
switch (nb_logical_bits) {
case 32:
    chip->playback_copy_fn = playback_copy_s32le;
    break;
case 24:
    chip->playback_copy_fn = (stride == 3) ?
        playback_copy_s24_3le : playback_copy_s24le;
    break;
case 16:
    chip->playback_copy_fn = playback_copy_s16le;
    break;
}
```

#### 2.2 S32_LE fast path (most common case)

When format is S32_LE and Ravenna internal format is also S32, the de-interleave becomes a strided copy. For each channel:

```c
static void playback_copy_s32le(struct mr_alsa_audio_chip *chip,
                                void *dst_base, const void *src,
                                unsigned int channels, unsigned int frames)
{
    unsigned int ch;
    const int32_t *in = src;
    uint32_t ring_size = MR_ALSA_RINGBUFFER_NB_FRAMES;
    uint32_t pos = chip->playback_buffer_pos;

    for (ch = 0; ch < channels; ch++) {
        int32_t *out = (int32_t *)(chip->playback_buffer + ch * ring_size * 4);
        uint32_t frames_before_wrap = ring_size - pos;
        uint32_t first = min(frames, frames_before_wrap);
        unsigned int f;

        /* Fast block: no wrap */
        for (f = 0; f < first; f++)
            out[pos + f] = in[f * channels + ch];

        /* Wrap portion */
        for (f = first; f < frames; f++)
            out[f - first] = in[f * channels + ch];
    }
}
```

Further optimization: for the common case of `frames < ring_size - pos` (no wrap), the branch is eliminated entirely.

#### 2.3 Wrap handling outside inner loop

Pre-compute `frames_before_wrap = ring_buffer_size - current_pos`. If `frames <= frames_before_wrap`, skip the wrap check entirely. Otherwise, split into two memcpy/strided-copy operations.

### Files Modified

- `driver/audio_driver.c` — new copy functions, function pointer setup in prepare, interrupt handler calls function pointer

## Phase 3: Interrupt Model Upgrade

### Current State

- Kernel < 5.0: uses upstream `tasklet_hrtimer` (removed in 5.0)
- Kernel 5.0-6.14: reimplements `tasklet_hrtimer` locally (`module_timer.c:38-87`)
- Kernel >= 6.15: uses `hrtimer_setup()` with `HRTIMER_MODE_ABS_SOFT`
- Tasklet runs in softirq context — unbounded latency, can be delayed by other softirqs
- The `HRTIMER_MODE_ABS_SOFT` path runs in ksoftirqd context — even worse for latency
- Busy-loop in `timer_callback` when `period == 0` (`module_timer.c:107-133`)

### Changes

#### 3.1 kthread worker model

Replace tasklet/softirq with a dedicated kthread:

```c
static struct kthread_worker *audio_worker;
static struct kthread_work audio_work;

static int audio_cpu_affinity = -1;  // -1 = no pinning
module_param(audio_cpu_affinity, int, 0444);
MODULE_PARM_DESC(audio_cpu_affinity, "CPU core to pin audio thread to (-1 = auto)");
```

Init:

```c
int init_clock_timer(void)
{
    if (audio_cpu_affinity >= 0) {
        /* Create worker already pinned to the target CPU */
        audio_worker = kthread_create_worker_on_cpu(audio_cpu_affinity, 0, "ravenna-audio");
    } else {
        audio_worker = kthread_create_worker(0, "ravenna-audio");
    }
    if (IS_ERR(audio_worker))
        return PTR_ERR(audio_worker);

    /* sched_set_fifo() available since kernel 5.9 — sets SCHED_FIFO
     * with a default priority of MAX_RT_PRIO/2. Preferred over raw
     * sched_setscheduler() for forward compatibility. */
    sched_set_fifo(audio_worker->task);

    kthread_init_work(&audio_work, audio_work_fn);

#ifdef CONFIG_PREEMPT_RT
    /* On PREEMPT_RT, HRTIMER_MODE_ABS runs in softirq-kthread context,
     * which is safe for kthread_queue_work(). _HARD would run in true
     * hard IRQ context where kthread_queue_work() is NOT safe. */
    hrtimer_init(&my_hrtimer_, CLOCK_MONOTONIC, HRTIMER_MODE_ABS);
#else
    /* On non-RT kernels, _HARD gives the most deterministic wakeup */
    hrtimer_init(&my_hrtimer_, CLOCK_MONOTONIC, HRTIMER_MODE_ABS_HARD);
#endif
    my_hrtimer_.function = timer_callback;

    return 0;
}
```

Timer callback queues work instead of running directly:

```c
static enum hrtimer_restart timer_callback(struct hrtimer *timer)
{
    kthread_queue_work(audio_worker, &audio_work);
    hrtimer_forward_now(timer, ns_to_ktime(base_period_));
    return HRTIMER_RESTART;
}

#define MAX_CATCHUP_ITERATIONS 4

static void audio_work_fn(struct kthread_work *work)
{
    uint64_t next_wakeup, now;
    int iterations = 0;

    /* The Ravenna manager's t_clock_timer() may request immediate
     * re-processing (next_wakeup <= now) when it is behind. We honor
     * this but cap iterations to prevent unbounded spinning. */
    do {
        t_clock_timer(&next_wakeup);
        get_clock_time(&now);
    } while (now >= next_wakeup && ++iterations < MAX_CATCHUP_ITERATIONS);
}
```

#### 3.2 Use HRTIMER_MODE_ABS_HARD (non-RT) or HRTIMER_MODE_ABS (RT)

On `PREEMPT_RT` kernels, `HRTIMER_MODE_ABS_HARD` runs the callback in true hard IRQ context, where `kthread_queue_work()` is **unsafe** (it takes a raw_spinlock internally). Use `HRTIMER_MODE_ABS` on RT kernels — this runs in a softirq-kthread context that is still deterministic on RT.

On standard kernels, `HRTIMER_MODE_ABS_HARD` gives the most deterministic wakeup. The `#ifdef CONFIG_PREEMPT_RT` guard in Section 3.1 handles this.

For kernel 6.15+, use `hrtimer_setup()` with the appropriate mode instead of the current `_SOFT`.

#### 3.3 Bounded catch-up loop

The current `do { ... } while (period == 0)` loop in `timer_callback` (lines 107-133) is not a pure busy-wait — it re-calls `t_clock_timer()` which performs actual audio processing when the manager is behind. Simply removing it would cause audio dropouts.

Replace with bounded catch-up in the kthread work function (see `audio_work_fn` in Section 3.1): iterate up to `MAX_CATCHUP_ITERATIONS` (4) times when the manager requests immediate re-processing. If still behind after 4 iterations, let the next hrtimer tick handle it rather than spinning.

#### 3.4 Graceful degradation

- On `PREEMPT_RT` kernels: kthread with SCHED_FIFO gives near-hard-realtime guarantees
- On standard `PREEMPT=full`: kthread with SCHED_FIFO still gives better latency than tasklets, with typical worst-case jitter < 100µs
- Module parameters allow tuning priority and CPU affinity per deployment

### Files Modified

- `driver/module_timer.c` — complete rewrite
- `driver/module_timer.h` — updated API if needed

## Phase 4: RTP & Jitter Buffer Improvements

### Current State

- Jitter buffer offset queried once at prepare time via `get_input_jitter_buffer_offset()`
- Base timer period hardcoded to 1.33ms in `module_timer.c:153`
- Data path: network packet → RTP jitter buffer → Ravenna ring buffer → DMA buffer → userspace

### Changes

#### 4.1 Dynamic timer base period

Compute from TIC frame size and sample rate:

```c
void update_base_period(uint32_t tic_frame_size, uint32_t sample_rate)
{
    uint64_t period_ns = ((uint64_t)tic_frame_size * 1000000000ULL) / sample_rate;
    set_base_period(period_ns);
}
```

Called when sample rate or TIC frame size changes.

#### 4.2 Adaptive jitter buffer depth

Add a new callback to `alsa_ops`:

```c
int (*set_jitter_buffer_depth)(void* ravenna_peer, uint32_t depth_in_frames);
```

Set proportional to packet time with configurable safety factor:

```c
static int jitter_buffer_multiplier = 3;  // 3x packet time
module_param(jitter_buffer_multiplier, int, 0644);  // writable at runtime

// In prepare:
uint32_t jitter_depth = ptp_frame_size * jitter_buffer_multiplier;
chip->mr_alsa_audio_ops->set_jitter_buffer_depth(chip->ravenna_peer, jitter_depth);
```

#### 4.3 Reduce capture buffer copies

Currently the capture path copies from the Ravenna ring buffer (non-interleaved S32) to the DMA buffer (interleaved, user format). For S32_LE format, investigate having `ProcessRTPAudioPacket()` write directly into per-channel regions of the DMA buffer, eliminating the intermediate Ravenna ring buffer copy for capture.

This is the riskiest change and should be done last, with careful A/B testing.

### Files Modified

- `driver/module_timer.c` — dynamic base period
- `driver/audio_driver.h` — new `alsa_ops` callback
- `driver/audio_driver.c` — jitter buffer setup in prepare
- `driver/RTP_audio_stream.c` — potential direct-write optimization

## Phase 5: Lock Reduction

### Current State

- `mr_alsa_audio_pcm_pointer()` takes `spin_lock(&chip->lock)` — the same global lock used by the interrupt handler
- The interrupt handler holds `chip->lock` for the entire duration of processing (format conversion + buffer copy)
- `chip->playback_lock` and `chip->capture_lock` exist in the struct but are only used for Ravenna ring buffer lock/unlock callbacks, not in the interrupt path

### Changes

#### 5.1 Atomic pointer tracking

Replace:
```c
uint32_t dma_playback_offset;
uint32_t dma_capture_offset;
```

With:
```c
atomic_t dma_playback_offset;
atomic_t dma_capture_offset;
```

Interrupt handler uses `atomic_set()`, pointer callback uses `atomic_read()`. No lock needed.

#### 5.2 Lock-free pointer callback

```c
static snd_pcm_uframes_t mr_alsa_audio_pcm_pointer(struct snd_pcm_substream *alsa_sub)
{
    struct mr_alsa_audio_chip *chip = snd_pcm_substream_chip(alsa_sub);

    if (alsa_sub->stream == SNDRV_PCM_STREAM_PLAYBACK) {
        struct snd_pcm_runtime *runtime = alsa_sub->runtime;
        unsigned long bytes_to_frame_factor = runtime->channels * chip->current_alsa_playback_stride;
        return atomic_read(&chip->dma_playback_offset) / bytes_to_frame_factor;
    } else {
        struct snd_pcm_runtime *runtime = alsa_sub->runtime;
        unsigned long bytes_to_frame_factor = runtime->channels * chip->current_alsa_capture_stride;
        return atomic_read(&chip->dma_capture_offset) / bytes_to_frame_factor;
    }
}
```

#### 5.3 Split interrupt lock scope

In `mr_alsa_audio_pcm_interrupt()`, replace `chip->lock` with direction-specific locks and minimize lock scope:

```c
if (direction == 1 && chip->capture_substream != NULL) {
    spin_lock(&chip->capture_lock);
    /* ... capture processing ... */
    spin_unlock(&chip->capture_lock);
    snd_pcm_period_elapsed(chip->capture_substream);  // outside lock
}
```

This allows playback and capture interrupts to process concurrently and prevents pointer queries from blocking on audio processing.

### Files Modified

- `driver/audio_driver.c` — atomic offsets, lock-free pointer, split lock scope

## Implementation Order

| Phase | Risk | Latency Impact | Dependencies |
|-------|------|---------------|--------------|
| 1. Period sizes | Low | **Massive** (21ms → 2ms) | None |
| 2. Hot path | Low | Medium (CPU headroom) | None |
| 3. Interrupt model | Medium | High (jitter reduction) | None |
| 4. RTP/jitter | Medium | Medium (stability at low periods) | Phase 1 |
| 5. Lock reduction | Low | Medium (jitter reduction) | None |

Phases 1, 2, 3, and 5 are independent and can be developed in parallel.
Phase 4 depends on Phase 1 (needs small periods to be meaningful).

## Testing Strategy

- **Functional:** ALSA `aplay`/`arecord` at each supported period size, verify no xruns
- **Latency:** `cyclictest` for interrupt jitter, JACK's xrun counter for end-to-end
- **Stress:** 64-channel playback+capture at 48 frames/period under CPU load (`stress-ng`)
- **Compatibility:** Test on kernel 5.15 (Ubuntu 22.04), 6.8 (Ubuntu 24.04), 6.15+ (latest), and PREEMPT_RT variants
- **AES67 interop:** Verify 1ms packet time works with third-party AES67 devices
