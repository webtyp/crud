# Performance of `webtyp/crud`

What a request costs through `crud`, measured by benchmarks that live in this repo, so any change can be compared with the numbers below. **Every plan that touches a hot path re-runs them and updates this file in the same change** (skill **testing** → «Benchmarks»).

## How to run and compare

```bash
go test ./tests -run '^$' -bench . -benchmem -count=6 > new.txt     # in crud/
benchstat old.txt new.txt                                           # go install golang.org/x/perf/cmd/benchstat@latest
```

Keep the output of the last release in `docs/bench/<version>.txt` (raw `go test -bench` output); a change is an improvement or a regression only when `benchstat` says so (p < 0.05), never from a single run.

| Group | File | What it measures |
|---|---|---|
| `Wire_*` | `tests/bench_wire_test.go` | encoding and decoding the arguments of each operation: what the browser (WASM) and the server pay on the wire |
| `Engine_*` | `tests/bench_test.go` | one request through the engine over the in-memory store: decode, validate, rule, transaction, store, publish, encode |

The database is not part of these numbers (it is measured in `orm`, see its `docs/PERFORMANCE.md`): a real request adds the database's time, and the database is the bottleneck.

## Results — `crud` v0.0.7 (unreleased benchmarks added after), 2026-11-09

Machine: 11th Gen Intel Core i7-11800H @ 2.30 GHz (16 threads), Linux x86_64, Go native. `-benchtime=1s -count=6`; the tables show the mean of the six runs, the raw output is `docs/bench/v0.0.7.txt` (the six runs of every row differ by less than 5%).

### Wire (client and server)

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `Wire_Cap` | 0.23 | 0 | 0 |
| `Wire_ReadArgs_Encode` | 163 | 80 | 1 |
| `Wire_ReadArgs_EncodeBytes` | 161 | 80 | 1 |
| `Wire_ReadArgs_Decode` | 349 | 72 | 3 |
| `Wire_ReadArgs_Pointers` | 23 | 64 | 1 |
| `Wire_CreateArgs_Encode_1` | 191 | 64 | 1 |
| `Wire_CreateArgs_Encode_10` | 1,049 | 448 | 1 |
| `Wire_CreateArgs_Encode_100` | 9,543 | 4,871 | 1 |
| `Wire_UpdateArgs_Encode` | 340 | 128 | 1 |
| `Wire_DeleteArgs_Encode_1` | 93 | 24 | 1 |
| `Wire_DeleteArgs_Encode_10` | 323 | 160 | 1 |

Every encode allocates once (the output buffer): the browser heap does not fragment.

### Engine (one request, in-process, in-memory store)

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `Engine_ReadOneByID` | 2,642 | 3,504 | 32 |
| `Engine_UpdateOneField` | 3,220 | 4,426 | 51 |
| `Engine_CreateAndDelete_NoRule` (2 requests) | 5,241 | 6,812 | 87 |
| `Engine_CreateAndDelete_WithRule` (2 requests) | 5,808 | 7,485 | 94 |
| `Engine_ReadPageOf50` (500 rows stored) | 120,200 | 127,998 | 736 |
| `Engine_Baseline_StoreOnly` (the same create+delete with no crud) | 797 | 1,841 | 19 |

Reading it:
- A single-record write without a business rule runs without opening a transaction: `Engine_CreateAndDelete_NoRule` drops from 5.8 µs (94 allocs) to 5.2 µs (87 allocs, -11% ns/op, -7 allocs); `Engine_UpdateOneField` drops from 3.6 µs (56 allocs) to 3.2 µs (51 allocs, -10% ns/op, -5 allocs). The in-memory store's transaction copies the store for rollback, so the simple path shows a large gain there that a real database does not have (where the gain is the saved BEGIN/COMMIT cycle).
- `crud`'s own cost per write request with a transaction is about **2.5 µs and 37 allocations** (`CreateAndDelete_WithRule` minus the baseline, over two requests).
- Per **1,000 requests**: about 2.5–6 ms of CPU and 3.5–7.5 MB allocated (garbage: `crud` keeps no state between requests). A page of 50 rows costs about 2.4 µs per row, almost all of it encoding the answer.
- Next to the database (`orm/docs/PERFORMANCE.md`: one insert on SQLite on disk takes about 1,250 µs) `crud` is **about 0.2%** of a write.

### Notes on the method

- `Engine_*` creates in steady state (each create is deleted before the next): the in-memory test store copies itself for its rollback, so a create into a store that keeps growing measures that copy, not `crud` (an earlier version of the benchmark did, and reported 1.7 ms per create).
- The numbers include the in-process loopback codec on both sides; over HTTP the server pays the decode once and the client the encode once, as in the `Wire_*` rows.
- The benchmarks of the previous version of this file (`BenchmarkMount*_Loopback_*`, `BenchmarkOp`) were deleted with the change that made `crud` persist tables (`feat!: crud persists tables`); their numbers are not comparable and were removed.

## What a transaction costs, by database (2026-11-09)

`conformance.RunBenchmarks` (in `crud/conformance`) measures one insert directly and the same insert inside `Atomic`; an implementation runs it from its own tests. The numbers below were taken with `orm` v0.12.28 over each driver, on an i7-11800H with an NVMe disk (ext4); raw output: `docs/bench/store-insert-sqlite-orm-v0.12.28.txt` and `docs/bench/store-insert-postgres-orm-v0.12.28.txt` (`-count=6`).

| Backend | No transaction | In a transaction | The transaction costs |
|---|---:|---:|---|
| SQLite in memory | 9.7 µs · 43 allocs | 12.5 µs · 55 allocs | +2.8 µs (+29%) |
| SQLite file, real disk (`fsync`), median | 1,279 µs | 1,254 µs | no measurable difference (the runs differ more than the gap) |
| Postgres 14 (Docker, same machine, TCP) | 352 µs · 54 allocs | 402 µs · 73 allocs | **+50 µs (+14%)**, the six runs agree within 2% |

On SQLite a real disk is dominated by the commit's `fsync` (about 1.25 ms), and two of the six file runs stalled at 3.8 ms and 10.8 ms (kept in the raw output: compare with `benchstat`, never with a mean). A file benchmark on a RAM disk (`tmpfs`) has no `fsync` and looks 30 times faster: never use it. On Postgres the transaction costs 14% even with the server on the same machine, which is why a write of one record with no rule runs without a transaction (decision D48).
