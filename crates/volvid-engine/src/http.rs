use std::path::{Path, PathBuf};
use std::time::{Duration, Instant};

use anyhow::{Context, Result, bail};
use futures_util::StreamExt;
use tokio::io::AsyncWriteExt;
use tokio::sync::mpsc;
use tokio_util::sync::CancellationToken;
use volvid_core::FileProgress;

use crate::paths::write_atomic;

pub async fn download_file(
    client: &reqwest::Client,
    url: &str,
    destination: &Path,
    progress: Option<&mpsc::UnboundedSender<FileProgress>>,
    cancel: Option<CancellationToken>,
) -> Result<PathBuf> {
    let response = client
        .get(url)
        .send()
        .await
        .with_context(|| format!("GET {url}"))?;
    if !response.status().is_success() {
        bail!("download failed with HTTP {}", response.status());
    }
    let total = response.content_length().unwrap_or(0) as i64;
    let mut stream = response.bytes_stream();
    let dir = destination.parent().unwrap_or_else(|| Path::new("."));
    std::fs::create_dir_all(dir)?;
    let mut temp = tokio::fs::File::create(destination).await?;
    let mut done = 0i64;
    let started = Instant::now();
    let mut last_emit = Instant::now() - Duration::from_secs(1);
    loop {
        let next = if let Some(token) = &cancel {
            tokio::select! {
                _ = token.cancelled() => {
                    let _ = tokio::fs::remove_file(destination).await;
                    send_progress(progress, FileProgress {
                        done: true,
                        err: Some("operation cancelled".to_owned()),
                        ..FileProgress::default()
                    });
                    bail!("operation cancelled");
                }
                next = stream.next() => next,
            }
        } else {
            stream.next().await
        };
        let Some(chunk) = next else {
            break;
        };
        let chunk = chunk.context("read download stream")?;
        temp.write_all(&chunk).await?;
        done += chunk.len() as i64;
        if last_emit.elapsed() >= Duration::from_millis(100) {
            let pct = if total > 0 {
                done as f64 / total as f64 * 100.0
            } else {
                0.0
            };
            let speed = if started.elapsed().as_secs_f64() > 0.0 {
                (done as f64 / started.elapsed().as_secs_f64()) as i64
            } else {
                0
            };
            send_progress(
                progress,
                FileProgress {
                    pct: pct.clamp(0.0, 100.0),
                    done_b: done,
                    total_b: total,
                    speed: volvid_i18n::format_speed(speed, volvid_core::Locale::En),
                    done: false,
                    err: None,
                },
            );
            last_emit = Instant::now();
        }
    }
    temp.flush().await?;
    temp.sync_all().await?;
    drop(temp);
    send_progress(
        progress,
        FileProgress {
            pct: 100.0,
            done_b: done,
            total_b: if total > 0 { total } else { done },
            speed: String::new(),
            done: true,
            err: None,
        },
    );
    Ok(destination.to_path_buf())
}

fn send_progress(progress: Option<&mpsc::UnboundedSender<FileProgress>>, value: FileProgress) {
    if let Some(tx) = progress {
        let _ = tx.send(value);
    }
}

pub async fn fetch_text(client: &reqwest::Client, url: &str) -> Result<String> {
    let response = client
        .get(url)
        .send()
        .await
        .with_context(|| format!("GET {url}"))?;
    if !response.status().is_success() {
        bail!("HTTP {} for {url}", response.status());
    }
    Ok(response.text().await?)
}

pub async fn fetch_json(client: &reqwest::Client, url: &str) -> Result<serde_json::Value> {
    let response = client
        .get(url)
        .send()
        .await
        .with_context(|| format!("GET {url}"))?;
    if !response.status().is_success() {
        bail!("HTTP {} for {url}", response.status());
    }
    Ok(response.json().await?)
}

pub fn write_text_atomic(path: &Path, text: &str) -> Result<()> {
    write_atomic(path, text.as_bytes())?;
    Ok(())
}
