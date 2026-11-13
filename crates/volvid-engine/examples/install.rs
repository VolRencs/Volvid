//! Managed ytrust installer check.
//!
//! `VOLVID_DEPS_DIR=/tmp/volvid-install cargo run -p volvid-engine --example install`

use anyhow::{Result, bail};
use volvid_core::Locale;
use volvid_engine::Engine;
use volvid_engine::deps::probe_version;

#[tokio::main]
async fn main() -> Result<()> {
    let engine = Engine::new()?;
    let (tx, mut rx) = tokio::sync::mpsc::unbounded_channel();
    let installer = engine.install_dependency("ytrust", Locale::En, None, tx);
    tokio::pin!(installer);
    loop {
        tokio::select! {
            result = &mut installer => {
                while let Ok(progress) = rx.try_recv() {
                    println!("  {:.1}%  {}", progress.pct, progress.speed);
                }
                result?;
                break;
            }
            progress = rx.recv() => {
                if let Some(progress) = progress {
                    println!("  {:.1}%  {}", progress.pct, progress.speed);
                }
            }
        }
    }
    let bin = engine.paths().ytrust_bin();
    if !bin.is_file() {
        bail!("managed ytrust was not installed at {}", bin.display());
    }
    let version = probe_version(&bin.to_string_lossy(), &["--version"], None).await;
    println!("installed ytrust {version} at {}", bin.display());
    if !version.starts_with("1.") {
        bail!("unexpected installed version {version:?}");
    }
    println!("install check passed");
    Ok(())
}
