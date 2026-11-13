//! Live search check: `cargo run -p volvid-engine --example search -- "query"`.

use anyhow::{Result, bail};
use volvid_core::Locale;
use volvid_engine::Engine;

#[tokio::main]
async fn main() -> Result<()> {
    let query = std::env::args()
        .nth(1)
        .unwrap_or_else(|| "rick astley".to_owned());
    let engine = Engine::new()?;
    let results = engine.search_youtube(&query, Locale::En, None).await?;
    for result in &results {
        println!("{}  {}s  {}", result.title, result.duration, result.url);
    }
    if results.is_empty() || results.len() > 5 {
        bail!("expected 1-5 results, got {}", results.len());
    }
    println!("search check passed");
    Ok(())
}
