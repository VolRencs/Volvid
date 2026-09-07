## What was done

<!-- Briefly: what task, what changed -->

## Why

<!-- Why this is needed -->

## How it was verified

- [ ] `go vet ./...`
- [ ] `go test -race ./...`
- [ ] Build: `scripts/build-linux-downloader.sh` / `scripts/build-windows-downloader.sh`
- [ ] Manual check in the TUI: <!-- screens walked through, language EN/RU -->

## Checklist

- [ ] UI strings added in both RU and EN (`internal/i18n/`)
- [ ] `internal/core` stays pure (no I/O)
- [ ] No binaries, cookies, tokens, or private paths committed
- [ ] Base branch is `Dev`
