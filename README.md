# BuddyAPY

`buddyapy` is a Go CLI for exploring yield opportunities from the free DefiLlama yields API.

It supports ranked pool searches, pool history lookups, stablecoin filtering, TVL thresholds, yield band filters, JSON output, and direct links to the DefiLlama pool page for each result.

## Install

Build the binary locally:

```bash
go build ./cmd/buddyapy
```

Or run it directly:

```bash
go run ./cmd/buddyapy --help
```

## Quick Start

Top stablecoin yields in the last 30 days with at least $10M TVL:

```bash
go run ./cmd/buddyapy pools \
  --stablecoin \
  --min-tvl 10m \
  --lookback 30d \
  --rank-by snapshot-30d-mean \
  --limit 10
```

Filter to a yield band:

```bash
go run ./cmd/buddyapy pools \
  --stablecoin \
  --min-tvl 10m \
  --lookback 30d \
  --rank-by snapshot-30d-mean \
  --min-yield 8 \
  --max-yield 10 \
  --limit 5
```

Inspect one pool:

```bash
go run ./cmd/buddyapy chart \
  --pool 1994cc35-a2b9-434e-b197-df6742fb5d81 \
  --lookback 30d
```

Request JSON output:

```bash
go run ./cmd/buddyapy pools --stablecoin --min-tvl 10m --json
```

## Commands

### `buddyapy pools`

Ranks pools after applying client-side filters over the live `/pools` response.

Supported flags:

- `--stablecoin`
- `--min-tvl`
- `--lookback`
- `--rank-by`
- `--min-yield`
- `--max-yield`
- `--chain`
- `--project`
- `--limit`
- `--json`

### `buddyapy chart`

Fetches historical APY and TVL data for one pool from `/chart/{pool}`.

Supported flags:

- `--pool`
- `--lookback`
- `--json`

## Ranking Rules

- `snapshot-30d-mean` ranks by DefiLlama's `apyMean30d` field from `/pools`
- `chart-mean` fetches `/chart/{pool}` and computes the mean APY over the requested `--lookback`
- `current-apy` ranks by the live `apy` field from `/pools`

Notes:

- `snapshot-30d-mean` only works with `--lookback 30d`
- `--min-yield` and `--max-yield` apply to the selected ranking metric
- the `url` column points to the DefiLlama pool page for that row

## Development

Run the full suite, including live DefiLlama integration coverage:

```bash
go test ./...
```

Skip only the live integration test:

```bash
go test -short ./...
```

Additional docs:

- [Query Guide](docs/queries.md)
- [Development Notes](docs/development.md)

## API Notes

The DefiLlama docs page lists the free yields endpoint as `https://api.llama.fi/pools`, but this project uses `https://yields.llama.fi/pools`, which was validated against live responses on April 17, 2026.

The CLI uses only the free DefiLlama yields endpoints:

- `GET https://yields.llama.fi/pools`
- `GET https://yields.llama.fi/chart/{pool}`
