# Go Internals Deep Dive

Educational repo exploring Go runtime internals — one topic per standalone `main` package.

## Topics

| Directory | Topic |
|-----------|-------|
| `slicesgo/` | Slice header (`Data`, `Len`, `Cap`), backing-array reallocation, sub-slice sharing |
| `mapsgo/` | `hmap` / `bmap` struct, hash seed, bucket overflow chains, load-factor resizing |
| `memoryallocation/` | Stack vs. heap, escape analysis |
| `goscheduler(GMP Model)/` | G (goroutine), M (OS thread), P (logical processor), netpoller, hand-off |
| `channel-internals/` | `hchan` struct, circular ring buffer, sendq/recvq, direct-copy optimization, close semantics |
| `garbage_collection_mechanidm/` | Tri-color concurrent mark & sweep, write barrier, `runtime.GC()` |
| `bypassing_gc/` | `sync.Pool` GC bypass for high-throughput, reuse pattern |
| `defer_panic_recover_internal/` | `_defer` struct, panic/recover internals, stack-allocated defer |

## Run

```bash
go run ./slicesgo
go run ./mapsgo
# etc.
```

No `go.mod` — every directory is a standalone `package main`.

## System Design Interview Q&A

Also contains [docs/](./docs) — 110 system design interview questions covering 11 chapters from the "Designing Data-Intensive Applications" problem set (Rate Limiter, Consistent Hashing, Key-Value Store, Unique ID Generator, URL Shortener, Web Crawler, Notification System, News Feed, Chat, Search Autocomplete, YouTube). Generated from an external question bank.
