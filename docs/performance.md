# Performance

## Running benchmarks

```sh
go test -run=^$ -bench=. -benchmem .
```

## Comparing before/after

```sh
go test -run=^$ -bench=. -benchmem -count=10 . > old.txt
# make your change
go test -run=^$ -bench=. -benchmem -count=10 . > new.txt
benchstat old.txt new.txt
```

## Current baseline

Measured on a development machine (AMD Ryzen 7 5800X3D); treat as relative, not absolute.

| Benchmark | Time/op | Throughput | B/op | allocs/op |
|---|---:|---:|---:|---:|
| `ParseGenericArguments` | 1.54 ms | 15.6 MB/s | 1.09 MB | 2054 |
| `ParseLargeFile` | 145.6 ms | 12.3 MB/s | 157 MB | 98245 |
| `ParseTokensLargeFile` | 98.2 ms | 18.3 MB/s | 93 MB | 98083 |
| `ParseForLinterLargeFile` | 79.2 ms | 22.6 MB/s | 37.7 MB | 97781 |
| `ParseCompactLargeFile` | 77.3 ms | 23.2 MB/s | 37.6 MB | 97780 |
| `ParseCompactRetainedLargeFile` | 108.6 ms | 16.5 MB/s | 122.9 MB | 97850 |
| `TokensOnlyLargeFile` | 33.6 ms | 53.4 MB/s | 32.8 MB | 42 |
| `TypedSyntaxTraversal` | 46.1 µs | n/a | 0 | 0 |

`ParseCompactLargeFile` is the cheapest full-parse path; the `*Retained*`
and generic-node variants trade allocations for retained CST detail.

## Regression policy

Not an automatic CI failure; compare with `benchstat` before merging a
change that touches lexing or parsing.
