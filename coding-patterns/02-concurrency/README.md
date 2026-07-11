# Concurrency Patterns

Leverage goroutines and channels to build parallel, streaming, and event-driven systems.

```
02-concurrency/
├── worker-pool/         # Bounded goroutine pool for parallel task processing
├── fan-out-fan-in/      # Distribute work across goroutines, merge results
├── pipeline/            # Composable channel stages
├── or-channel/          # Wait on any of N done channels
├── tee-channel/         # Split one stream into two
└── fan-in/              # Merge multiple channels into one
```

---

## Worker Pool

**File:** [`worker-pool/worker-pool.go`](worker-pool/worker-pool.go)

### What It Does

Spins up a fixed number of goroutines (workers) that read from a shared input channel and write to a shared output channel.

```
          ┌─────────┐
          │ jobs ch │
          └────┬────┘
               │
     ┌─────────┼─────────┐
     ▼         ▼         ▼
  ┌─────┐  ┌─────┐  ┌─────┐
  │ w-1 │  │ w-2 │  │ w-3 │   ← fixed pool (3 workers)
  └──┬──┘  └──┬──┘  └──┬──┘
     │        │        │
     └────────┼────────┘
              ▼
          ┌─────────┐
          │results ch│
          └─────────┘
```

- Workers are started before sending jobs.
- `close(jobs)` signals workers to exit their `range` loops.
- A separate goroutine waits for all workers (`wg.Wait()`), then closes the results channel.

### Key Implementation Details

- `sync.WaitGroup` to track when all workers finish.
- Workers use `range` over the jobs channel — automatically exits when channel is closed and drained.
- Results channel is closed by a monitoring goroutine after `wg.Wait()`.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Processing a large batch of independent tasks | Single or few tasks (just use `go fn()`) |
| CPU-bound work that benefits from bounded parallelism | I/O-bound work with different isolation needs (use bulkhead) |
| Tasks where order doesn't matter | Tasks requiring ordered processing (use pipeline or single goroutine) |

### Real-World Scenarios

**Image thumbnailer.** 10,000 images need thumbnails. A pool of 4 workers pulls image paths from a channel, reads from disk, resizes, writes to disk, and sends the result. Memory usage stays flat at 4 image buffers. Without a pool, 10,000 goroutines would OOM the process.

**Email sender.** Marketing campaign sends 50,000 emails. A pool of 20 workers pulls recipient records, renders the template, and sends via SMTP. The SMTP server sees at most 20 concurrent connections, staying within its connection limit.

---

## Fan-Out / Fan-In

**File:** [`fan-out-fan-in/fan-out-fan-in.go`](fan-out-fan-in/fan-out-fan-in.go)

### What It Does

- **Fan-Out:** one input channel is read by multiple goroutines, each processing independently.
- **Fan-In:** multiple output channels are merged into a single channel using `sync.WaitGroup`.

```
                   ┌───────┐  ─► square() ─► ┐
     generate() ──►│ split │                  ├──► fanIn() ─► results
                   └───────┘  ─► square() ─► ┘
```

Because Go channels are first-class values, the same function can serve both as a producer and consumer — the pattern is the wiring.

### Key Implementation Details

- Fan-out works because multiple goroutines can `select` or `range` on the same channel — each value goes to exactly one consumer.
- Fan-in uses a `sync.WaitGroup` per source channel and a shared output channel.
- The fan-in goroutine closes the output only after all source channels are exhausted.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Data-parallel CPU-bound computations | Sequential processing where output depends on input order |
| Independent operations on each item | Operations that share mutable state per item |
| Load-balancing across heterogeneous workers | Exactly-once delivery scenarios |

### Real-World Scenarios

**Image processing pipeline.** 100 images need 4 operations: resize, watermark, compress, upload. Fan-out to 4 resize workers, each resizes a different image. Fan-in the results (4 streams) into one channel for the watermark stage. On 8 CPU cores, this is ~4x faster than sequential.

**Log processing.** A log file with 1M lines fans out to 4 parsers. Each parser processes lines independently, extracting structured fields. Fan-in merges the parsed events into a single channel for batch writing to Elasticsearch.

---

## Pipeline

**File:** [`pipeline/pipeline.go`](pipeline/pipeline.go)

### What It Does

Chains independent processing stages via channels. Each stage is a `func(<-chan T) <-chan U`. The `Pipeline()` composer wires them together left-to-right.

```go
p := Pipeline(lowercase, splitWords, filterLongWords)
out := p(input)
```

```
sentence ──► lower ──► split ──► filter ──► words
ch             ch        ch         ch        ch
```

Each stage runs in its own goroutine. Value flows through all stages concurrently — the pipeline's throughput is bounded by the slowest stage.

### Key Implementation Details

- Each stage creates an output channel, spawns a goroutine that reads from input and writes to output, and returns the output channel.
- `Pipeline(stages...)` nests stages so that stage N's output is stage N+1's input.
- Closing the input channel signals the stage to finish; closing output propagates the signal downstream.
- The composition function iterates from last to first to maintain correct order (or uses left-to-right with functional reversal).

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Multi-step data transformation pipelines | Two-stage operations (just compose functions) |
| Streaming data where each stage can run concurrently | Data that fits in memory and doesn't benefit from streaming |
| Clean separation of processing concerns | Operations where stages must batch entire dataset before proceeding |

### Real-World Scenarios

**ETL pipeline.** `extract → transform → load`. Extract reads raw CSV rows from S3. Transform parses dates, normalises fields, validates. Load batches 1000 rows and inserts into Redshift. Each stage runs concurrently. While load is flushing, extract is already reading the next file.

**Video transcoding.** `demux → decode → filter → encode → mux`. The demuxer reads the container and emits audio+video packets. Decoder decompresses frames. Filters apply colour grading. Encoder compresses. Muxer writes the output container. Each stage is a separate goroutine, keeping core utilisation high.

---

## Or-Channel

**File:** [`or-channel/or-channel.go`](or-channel/or-channel.go)

### What It Does

Combines multiple done channels into one. The returned channel closes when **any** of the input channels close.

```go
done := or(sig1, sig2, sig3)
<-done  // blocks until sig1 OR sig2 OR sig3 fires
```

Uses recursive select:

```
or(a, b, c)  =  select { case <-a: case <-b: case <-or(c): }
or(c)        =  c
or()         =  closed channel
```

### Key Implementation Details

- Base cases: 0 channels → closed channel (trivially done); 1 channel → return it.
- Recursive case: select on ch[0], ch[1], and the result of `or(ch[2:])`.
- Each level spawns one goroutine — total goroutines = N-1 for N channels.
- Closed channels signal immediately (zero value read succeeds).

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Waiting on multiple cancellation sources | 2 channels (just use select directly) |
| Unknown number of done signals at compile time | Channels that outlive their receiver |
| Generic concurrency utility libraries | Performance-critical paths (select with N=2 is cheaper) |

### Real-World Scenarios

**Sidecar proxy shutdown.** Watches: `SIGTERM`, health-check failure, parent process death, watch-dog timer. `or(sigCh, healthCh, parentCh, watchdogCh)` returns when any fires. The shutdown sequence begins immediately.

**Database migration tool.** Runs N migrations in parallel, each with its own done channel. `or(doneChs...)` fires when the first migration fails or all complete, allowing the tool to stop early on error.

---

## Tee-Channel

**File:** [`tee-channel/tee-channel.go`](tee-channel/tee-channel.go)

### What It Does

Splits one input channel into two output channels. Every value sent to the input is received by **both** outputs.

```
              ┌─────────┐
  input ─────►│   tee   ├─────► out1 (all values)
              └─────────┴─────► out2 (all values)
```

The implementation uses an internal goroutine that reads from input and writes to each output, using a `select` with both outputs to ensure each value goes to both.

### Key Implementation Details

- A single goroutine reads from the input and writes to both outputs sequentially (not fan-out — both get every value).
- `defer close(out1); defer close(out2)` ensures both outputs close when input is exhausted.
- If one output blocks (slow consumer), the other is also blocked. Use buffered channels for decoupling.

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Broadcasting a single event stream to multiple consumers | Distributing work (use fan-out instead) |
| Audit logging + business processing from the same stream | Routing to specific consumers by type (use pub-sub) |

### Real-World Scenarios

**Trade feed.** A market data feed sends 10,000 trades/second. Tee splits into two identical streams. Stream 1 feeds the risk calculation engine. Stream 2 feeds the real-time price display. Both see every trade. If the risk engine falls behind (buffered channel), the price display is unaffected for `bufferSize` events.

**Webhook delivery.** An event bus sends order events. Tee splits the stream: one branch to the primary webhook delivery subsystem, one to the retry/dead-letter inspector. Both see every event independently.

---

## Fan-In

**File:** [`fan-in/fan-in.go`](fan-in/fan-in.go)

### What It Does

Merges multiple input channels into a single output channel. Values from any input channel are forwarded to the single output.

```
ch1 ───┐
ch2 ───┼──► fanIn ──► out
ch3 ───┘
```

Each input channel gets a dedicated goroutine that forwards values to the shared output channel. `sync.WaitGroup` coordinates completion.

### Key Implementation Details

- One goroutine per input channel, each reading with `range`.
- All goroutines write to the same output channel (this is safe — channel sends are concurrent-safe).
- A monitoring goroutine closes the output after all input goroutines finish (`wg.Wait()`).

### When to Use

| Do Use | Don't Use |
|--------|-----------|
| Merging results from multiple producers | Single producer (unnecessary overhead) |
| Combining data from multiple sources | Operations requiring ordered merging (use a priority queue) |
| Aggregating partial results from parallel computation | Exactly-once deduplication of values |

### Real-World Scenarios

**Kafka consumer group.** Three partitions consumed by three goroutines. Each goroutine has its own channel. Fan-in merges into one channel consumed by a single "process and commit" stage. The merge is order-independent, so no ordering guarantees are lost.

**Parallel API fetcher.** A dashboard fetches data from 5 microservices in parallel. Each fetch returns its result on a channel. Fan-in merges all 5 channels. A single consumer collects all results and renders the dashboard when all are received (tracked via a separate counter).
