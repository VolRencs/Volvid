## What was done

<!-- Briefly: what task, what changed -->

## Why

<!-- Why this is needed -->

## How it was verified

- [ ] `cargo fmt --all -- --check`
- [ ] `cargo clippy --workspace --all-targets -- -D warnings`
- [ ] `cargo test --workspace`
- [ ] Build: `cargo build --release -p volvid-gui`
- [ ] Manual check in the GUI: <!-- screens walked through, language EN/RU -->

## Checklist

- [ ] UI strings added in both RU and EN (`crates/volvid-i18n/`)
- [ ] `crates/volvid-core` stays pure (no I/O)
- [ ] No binaries, cookies, tokens, or private paths committed
- [ ] Base branch is `Dev`
