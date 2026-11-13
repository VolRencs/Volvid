//! Cancellation check: verifies that cancelling a download stops ytrust and
//! returns quickly instead of letting the child keep running.
//!
//! `cargo run -p volvid-engine --example cancel`
#![allow(clippy::disallowed_methods)]

use std::time::{Duration, Instant};

use anyhow::{Result, bail};
use volvid_core::{DownloadRequest, Locale, parse_target};
use volvid_engine::Engine;
use volvid_engine::ytrust::quality_choices_from_probe;

const URL: &str = "https://www.youtube.com/watch?v=dQw4w9WgXcQ";
const OUT: &str = "/tmp/volvid-cancel";

#[tokio::main]
async fn main() -> Result<()> {
    let engine = Engine::new()?;
    let deps = engine.runtime_deps().await;
    if !deps.ytrust.available {
        bail!("ytrust is missing");
    }
    let probe = engine.probe_media(URL, None).await?;
    let choices = quality_choices_from_probe(&probe)?;
    let quality = choices
        .iter()
        .find(|choice| choice.height >= 1080)
        .or_else(|| choices.first())
        .cloned()
        .ok_or_else(|| anyhow::anyhow!("no quality choices"))?;
    let profile = volvid_i18n::quality_profile(&quality, Locale::En);
    let request = DownloadRequest {
        target: parse_target(URL)?,
        profile,
        media_duration: probe.duration,
        output_dir: OUT.to_owned(),
        locale: Locale::En,
        ..DownloadRequest::default()
    };
    let _ = std::fs::remove_dir_all(OUT);
    std::fs::create_dir_all(OUT)?;

    let (tx, mut rx) = tokio::sync::mpsc::unbounded_channel();
    let cancel = tokio_util::sync::CancellationToken::new();
    let cancel_task = {
        let cancel = cancel.clone();
        tokio::spawn(async move {
            tokio::time::sleep(Duration::from_millis(1200)).await;
            cancel.cancel();
        })
    };
    let started = Instant::now();
    let result = engine.download(request, deps, tx, cancel).await;
    cancel_task.abort();
    while rx.try_recv().is_ok() {}
    let elapsed = started.elapsed();
    let error = result.err().map(|err| err.to_string()).unwrap_or_default();
    println!("cancel after {:.1}s: {error:?}", elapsed.as_secs_f64());
    if !error.to_ascii_lowercase().contains("cancel") {
        bail!("expected a cancellation error, got {error:?}");
    }
    if elapsed > Duration::from_secs(15) {
        bail!("cancellation took too long: {elapsed:?}");
    }
    #[cfg(unix)]
    {
        let output = std::process::Command::new("pgrep")
            .args(["-af", "ytrust.*dQw4w9WgXcQ"])
            .output()?;
        let text = String::from_utf8_lossy(&output.stdout);
        if !text.trim().is_empty() {
            bail!("ytrust is still running after cancellation: {text}");
        }
    }
    println!("cancel check passed");
    Ok(())
}
