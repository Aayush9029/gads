# gads

A Go CLI to query, report on, and change Google Ads accounts. It targets Google Ads API v25 and ships through `aayush9029/tap`. Status: active.

## Commands

| Task | Command |
|------|---------|
| Build | `make build` |
| Test | `make test` |
| Full check (race, vet, `go mod verify`; same as CI) | `make check` |
| Generate `docs/commands/gads.md` | `make docs` |
| Live read-only smoke | `GADS_LIVE=1 go test ./internal/live/...` |

## Architecture

- `internal/commands` holds the command registry and the service matrix. `cmd/gendocs` writes the command reference from the registry.
- `internal/cli` splits reads (`read.go`), writes (`mutate.go`), and local commands (`local.go`).
- `internal/auth` runs the OAuth login and token refresh. `internal/config` stores profiles and credentials.
- `internal/audit` records write actions.

## Rules

- Write commands make a local plan by default. They call Google only with `--validate` or `--execute`. Keep this for each new write command.
- Create new campaigns paused.
- Executed writes do not retry. Only reads and validation retry.
- Profiles hold non-secret settings. Secrets (`GADS_CLIENT_ID`, `GADS_CLIENT_SECRET`, `GADS_DEVELOPER_TOKEN`, `GADS_REFRESH_TOKEN`) go in a separate credentials file with mode `0600`. Keep the refusal to read a file that other users can read.
- Configuration order: flags, then `GADS_*` environment variables, then profile values.
- Automated tests use fake servers. Before you publish a release, run the live smoke with real credentials and make sure that it passes.
- After a registry change, run `make docs`.
