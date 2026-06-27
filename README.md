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

## Run

```bash
go run ./slicesgo
go run ./mapsgo
# etc.
```

No `go.mod` — every directory is a standalone `package main`.
