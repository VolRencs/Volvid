use std::path::{Path, PathBuf};
use std::time::{Duration, Instant};

use anyhow::{Context, Result, bail};
use futures_util::StreamExt;
use tokio::io::AsyncWriteExt;
use tokio::sync::mpsc;
use tokio_util::sync::CancellationToken;
use volvid_core::{FileProgress, Locale};

/// Reads a response body while enforcing a byte limit and cancellation.
pub async fn read_body(
    response: reqwest::Response,
    max_bytes: usize,
    cancel: Option<&CancellationToken>,
    too_large_message: &str,
) -> Result<Vec<u8>> {
    let mut bytes = Vec::with_capacity(
        response
            .content_length()
            .unwrap_or(64 * 1024)
            .min(max_bytes as u64) as usize,
    );
    let mut stream = response.bytes_stream();
    loop {
        let next = if let Some(token) = cancel {
            tokio::select! {
                _ = token.cancelled() => bail!("operation cancelled"),
                next = stream.next() => next,
            }
        } else {
            stream.next().await
        };
        let Some(chunk) = next else {
            break;
        };
        let chunk = chunk.context("read response stream")?;
        if bytes.len() + chunk.len() > max_bytes {
            bail!("{too_large_message}");
        }
        bytes.extend_from_slice(&chunk);
    }
    Ok(bytes)
}

pub async fn download_file(
    client: &reqwest::Client,
    url: &str,
    destination: &Path,
    locale: Locale,
    progress: Option<&mpsc::UnboundedSender<FileProgress>>,
    cancel: Option<CancellationToken>,
) -> Result<PathBuf> {
    let response = get_ok(client, url).await?;
    let total = response.content_length().unwrap_or(0) as i64;
    let mut stream = response.bytes_stream();
    let dir = destination.parent().unwrap_or_else(|| Path::new("."));
    std::fs::create_dir_all(dir)?;
    let temp = tempfile::Builder::new()
        .prefix(".volvid-download-")
        .tempfile_in(dir)
        .context("create temporary download file")?;
    let temp_path = temp.path().to_path_buf();
    let mut file = tokio::fs::File::from_std(temp.reopen()?);
    let mut done = 0i64;
    let started = Instant::now();
    let mut last_emit = Instant::now()
        .checked_sub(Duration::from_secs(1))
        .unwrap_or_else(Instant::now);
    loop {
        let next = if let Some(token) = &cancel {
            tokio::select! {
                _ = token.cancelled() => {
                    drop(file);
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
        file.write_all(&chunk).await?;
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
                    speed: volvid_i18n::format_speed(speed, locale),
                    done: false,
                    err: None,
                },
            );
            last_emit = Instant::now();
        }
    }
    file.flush().await?;
    file.sync_all().await?;
    drop(file);
    temp.persist(destination)
        .map_err(|err| anyhow::anyhow!("persist {}: {}", temp_path.display(), err.error))?;
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

async fn get_ok(client: &reqwest::Client, url: &str) -> Result<reqwest::Response> {
    let response = client
        .get(url)
        .send()
        .await
        .with_context(|| format!("GET {url}"))?;
    if !response.status().is_success() {
        bail!("HTTP {} for {url}", response.status());
    }
    Ok(response)
}

pub async fn fetch_text(client: &reqwest::Client, url: &str) -> Result<String> {
    Ok(get_ok(client, url).await?.text().await?)
}

pub async fn fetch_json(client: &reqwest::Client, url: &str) -> Result<serde_json::Value> {
    Ok(get_ok(client, url).await?.json().await?)
}
