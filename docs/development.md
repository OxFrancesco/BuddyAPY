# Development

## Requirements

- Go 1.26 or newer
- Network access for the live integration tests
- Optional: GitHub CLI (`gh`) if you want to publish the project

## Local Commands

Build the CLI:

```bash
go build ./cmd/buddyapy
```

Run the full test suite, including live DefiLlama checks:

```bash
go test ./...
```

Skip only the live integration test:

```bash
go test -short ./...
```

Run the CLI directly without building a binary:

```bash
go run ./cmd/buddyapy pools --stablecoin --min-tvl 10m
```

## Repository Layout

- `cmd/buddyapy`: executable entrypoint
- `internal/app`: CLI parsing, filtering, ranking, rendering, live tests
- `internal/llama`: DefiLlama client and API response types
- `docs`: usage examples and development notes

## API Host Note

The DefiLlama docs page currently documents the free yields endpoint under `https://api.llama.fi/pools`, but this project intentionally uses `https://yields.llama.fi` because that host was validated against live responses on April 17, 2026.
