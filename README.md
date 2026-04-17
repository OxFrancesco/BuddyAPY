# BuddyAPY

`buddyapy` is a Go terminal explorer for yield opportunities from the free DefiLlama yields API.

It now ships with a full-screen TUI for interactive discovery plus the original CLI commands for scripting, JSON output, and shell workflows.

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

Launch the interactive explorer:

```bash
go run ./cmd/buddyapy tui
```

The TUI keeps a cached `/pools` snapshot in memory, reruns cheap filters locally, and loads chart history lazily for the selected pool.

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

Search Ethereum-related symbols with fuzzy matching:

```bash
go run ./cmd/buddyapy pools \
  --chain Ethereum \
  --symbol eth \
  --fuzzy \
  --min-tvl 10m \
  --lookback 30d \
  --rank-by snapshot-30d-mean \
  --limit 20
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

### `buddyapy tui`

Interactive dashboard with three panes:

- filter editor
- ranked results table
- selected-pool details with APY/TVL mini charts

Keybindings:

- `Tab` / `Shift+Tab` cycle panes
- `Enter` edits the focused filter or jumps from the results table into details
- `Space` toggles boolean filters
- `/` jumps directly to the symbol filter
- `c` clears the focused filter
- `C` clears all filters
- `r` refreshes live data and reruns the search
- `q` quits

Refresh behavior:

- cheap filters rerun automatically against the cached `/pools` snapshot
- `chart-mean` mode is treated as expensive and requires `r` to rerun
- selected-pool chart history loads lazily and is cached by pool ID

### `buddyapy pools`

Ranks pools after applying client-side filters over the live `/pools` response.

Supported flags:

- `--stablecoin`
- `--min-tvl`
- `--lookback`
- `--rank-by`
- `--min-yield`
- `--max-yield`
- `--symbol`
- `--fuzzy`
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
- `--symbol` filters by pool symbol; exact mode matches full symbols or symbol legs like `WETH` in `WETH-USDC`
- `--fuzzy` applies substring matching to `--symbol`, so `eth` can match `steth`, `wsteth`, `weeth`, `reth`, and similar names
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
