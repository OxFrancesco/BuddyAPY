# Query Guide

This project is built around the `buddyapy pools` and `buddyapy chart` subcommands.

## Common Searches

Top stablecoin yields in the last 30 days with at least $10M TVL:

```bash
go run ./cmd/buddyapy pools \
  --stablecoin \
  --min-tvl 10m \
  --lookback 30d \
  --rank-by snapshot-30d-mean \
  --limit 10
```

Constrain the result set to a yield band:

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

Only search one chain:

```bash
go run ./cmd/buddyapy pools \
  --stablecoin \
  --chain Ethereum \
  --min-tvl 25m \
  --rank-by current-apy
```

Compute the ranking from pool history instead of DefiLlama's `apyMean30d` snapshot field:

```bash
go run ./cmd/buddyapy pools \
  --stablecoin \
  --min-tvl 10m \
  --lookback 14d \
  --rank-by chart-mean
```

## Inspect One Pool

Fetch recent chart history for a pool:

```bash
go run ./cmd/buddyapy chart \
  --pool 1994cc35-a2b9-434e-b197-df6742fb5d81 \
  --lookback 30d
```

Get JSON for scripting:

```bash
go run ./cmd/buddyapy pools --stablecoin --min-tvl 10m --json
go run ./cmd/buddyapy chart --pool 1994cc35-a2b9-434e-b197-df6742fb5d81 --json
```

## Ranking Semantics

- `snapshot-30d-mean` ranks by DefiLlama's `apyMean30d` field from `/pools`
- `chart-mean` fetches `/chart/{pool}` and averages APY values inside `--lookback`
- `current-apy` ranks by the live `apy` field from `/pools`

`--min-yield` and `--max-yield` always apply to the selected ranking metric.

## Output Notes

- The `pool` column is the DefiLlama pool ID from the API.
- The `url` column links directly to the corresponding DefiLlama pool page.
- `--json` returns machine-readable output suitable for `jq` or scripts.
