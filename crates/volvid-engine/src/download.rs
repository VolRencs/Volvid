use std::collections::HashSet;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::Duration;

use anyhow::{Context, Result, anyhow, bail};
use tokio::sync::mpsc;
use tokio_util::sync::CancellationToken;
use volvid_core::{
    CheckDepsResult, DlEventType, DlUpdate, DownloadMode, DownloadRequest, Locale, PlaylistEntry,
};

use crate::args::{
    MARK_MOVED, MARK_POST, MARK_PROGRESS, MARK_START, build_download_args, download_formats,
    format_label, parse_json_string, playlist_output_template, single_output_template,
};
use crate::deps::ytdlp_bin;
use crate::ffmpeg::{cleanup_subtitle_sidecars, retag_audio_languages, transcode_video};
use crate::paths::{AppPaths, clean_abs};
use crate::process::{ProcessError, ProcessLine, run_lines};
use crate::ytdlp::{format_progress_speed, postprocess_label};

const SLOT_RESET_DELAY: Duration = Duration::from_millis(300);

pub async fn run_download(
    paths: Arc<AppPaths>,
    deps: CheckDepsResult,
    request: DownloadRequest,
    tx: mpsc::UnboundedSender<DlUpdate>,
    cancel: CancellationToken,
    encoders: Option<Arc<HashSet<String>>>,
) -> Result<()> {
    let request = normalize_request(request, &paths);
    validate_request(&request, &deps)?;
    let output_dir = prepare_dir(&request.output_dir)?;
    if request.uses_playlist() {
        run_playlist(paths, deps, request, output_dir, tx, cancel, encoders).await
    } else {
        run_single(paths, deps, request, output_dir, tx, cancel, encoders).await
    }
}

async fn run_single(
    paths: Arc<AppPaths>,
    deps: CheckDepsResult,
    request: DownloadRequest,
    output_dir: PathBuf,
    tx: mpsc::UnboundedSender<DlUpdate>,
    cancel: CancellationToken,
    encoders: Option<Arc<HashSet<String>>>,
) -> Result<()> {
    let url = request.target.download_url(request.force_single);
    send(
        &tx,
        DlUpdate {
            event_type: DlEventType::Start,
            slot: 0,
            text: volvid_i18n::strings(request.locale).downloading.to_owned(),
            ..DlUpdate::default()
        },
    );
    let result = run_one(
        paths,
        deps,
        &request,
        0,
        &url,
        &single_output_template(&output_dir),
        &tx,
        &cancel,
        encoders.as_deref(),
    )
    .await;
    send(
        &tx,
        DlUpdate {
            event_type: DlEventType::Done,
            slot: 0,
            ok: result.is_ok(),
            err_text: result.as_ref().err().cloned().unwrap_or_default(),
            pct: 100.0,
            ..DlUpdate::default()
        },
    );
    result.map_err(|err| anyhow!("{err}"))
}

async fn run_playlist(
    paths: Arc<AppPaths>,
    deps: CheckDepsResult,
    request: DownloadRequest,
    output_dir: PathBuf,
    tx: mpsc::UnboundedSender<DlUpdate>,
    cancel: CancellationToken,
    encoders: Option<Arc<HashSet<String>>>,
) -> Result<()> {
    let entries = request.entries.clone();
    if entries.is_empty() {
        bail!("playlist entries are required");
    }
    let title = request
        .playlist_info
        .as_ref()
        .map(|info| info.title.as_str())
        .unwrap_or("playlist");
    let playlist_dir = playlist_output_dir(&output_dir, title);
    let workers = request.workers.clamp(1, 8).min(entries.len());
    let (jobs_tx, jobs_rx) = mpsc::channel::<PlaylistEntry>(entries.len().max(1));
    let jobs_rx = Arc::new(tokio::sync::Mutex::new(jobs_rx));
    let mut tasks = Vec::new();
    for slot in 0..workers {
        let paths = paths.clone();
        let deps = deps.clone();
        let request = request.clone();
        let tx = tx.clone();
        let cancel = cancel.clone();
        let jobs_rx = jobs_rx.clone();
        let playlist_dir = playlist_dir.clone();
        let encoders = encoders.clone();
        tasks.push(tokio::spawn(async move {
            loop {
                let entry = {
                    let mut rx = jobs_rx.lock().await;
                    match rx.recv().await {
                        Some(entry) => entry,
                        None => break,
                    }
                };
                if cancel.is_cancelled() {
                    break;
                }
                let result = run_playlist_entry(
                    paths.clone(),
                    deps.clone(),
                    request.clone(),
                    slot,
                    &playlist_dir,
                    entry,
                    &tx,
                    &cancel,
                    encoders.as_deref(),
                )
                .await;
                if let Err(err) = &result {
                    tracing::debug!(%err, "playlist entry failed");
                }
            }
        }));
    }
    for entry in entries {
        if cancel.is_cancelled() {
            break;
        }
        if jobs_tx.send(entry).await.is_err() {
            break;
        }
    }
    drop(jobs_tx);
    for task in tasks {
        if let Ok(()) = task.await {
            // Worker finished.
        }
    }
    if cancel.is_cancelled() {
        bail!("operation cancelled");
    }
    Ok(())
}

#[allow(clippy::too_many_arguments)]
async fn run_playlist_entry(
    paths: Arc<AppPaths>,
    deps: CheckDepsResult,
    request: DownloadRequest,
    slot: usize,
    output_dir: &Path,
    entry: PlaylistEntry,
    tx: &mpsc::UnboundedSender<DlUpdate>,
    cancel: &CancellationToken,
    encoders: Option<&HashSet<String>>,
) -> Result<(), String> {
    send(
        tx,
        DlUpdate {
            event_type: DlEventType::Start,
            slot,
            text: entry.title.clone(),
            ..DlUpdate::default()
        },
    );
    let template = playlist_output_template(output_dir, &entry);
    let result = run_one(
        paths, deps, &request, slot, &entry.url, &template, tx, cancel, encoders,
    )
    .await;
    send(
        tx,
        DlUpdate {
            event_type: DlEventType::Done,
            slot,
            ok: result.is_ok(),
            err_text: result.as_ref().err().cloned().unwrap_or_default(),
            pct: 100.0,
            ..DlUpdate::default()
        },
    );
    tokio::time::sleep(SLOT_RESET_DELAY).await;
    send(
        tx,
        DlUpdate {
            event_type: DlEventType::Reset,
            slot,
            ..DlUpdate::default()
        },
    );
    result
}

#[allow(clippy::too_many_arguments)]
async fn run_one(
    paths: Arc<AppPaths>,
    deps: CheckDepsResult,
    request: &DownloadRequest,
    slot: usize,
    url: &str,
    output_template: &str,
    tx: &mpsc::UnboundedSender<DlUpdate>,
    cancel: &CancellationToken,
    encoders: Option<&HashSet<String>>,
) -> Result<(), String> {
    let (formats, labels) = download_formats(request);
    if formats.is_empty() {
        return Err("no download formats configured".to_owned());
    }
    let mut last_result: Result<(), String> = Err("download failed".to_owned());
    for (index, format) in formats.iter().enumerate() {
        if cancel.is_cancelled() {
            return Err(volvid_i18n::strings(request.locale)
                .operation_cancelled
                .to_owned());
        }
        if request.profile.mode == DownloadMode::Video && index > 0 {
            let label = format_label(format, &labels, index);
            let text = volvid_i18n::strings(request.locale)
                .fallback_format
                .replace("%d", &index.to_string())
                .replace("%s", &label);
            send(
                tx,
                DlUpdate {
                    event_type: DlEventType::Fallback,
                    slot,
                    text,
                    ..DlUpdate::default()
                },
            );
        }
        let args = build_download_args(
            request,
            &deps,
            url,
            output_template,
            format,
            &["--no-playlist".to_owned()],
        );
        let mut progress = ProgressState::default();
        let Some(ytdlp) = ytdlp_bin(&deps) else {
            return Err("yt-dlp is required".to_owned());
        };
        let result = run_ytdlp_download(
            ytdlp,
            &args,
            slot,
            request.locale,
            tx,
            cancel,
            &mut progress,
        )
        .await;
        if let Err(err) = result {
            last_result = Err(err.to_string());
            continue;
        }
        let mut output_path = progress.output_path;
        if output_path.as_os_str().is_empty() {
            last_result = Err("downloaded file path is unknown".to_owned());
            continue;
        }
        match transcode_video(
            &paths,
            &deps,
            &request.profile,
            &output_path,
            encoders,
            Some(cancel.clone()),
        )
        .await
        {
            Ok(Some(final_path)) => output_path = final_path,
            Ok(None) => {}
            Err(err) => {
                last_result = Err(err.to_string());
                continue;
            }
        }
        if request.profile.wants_subtitles() {
            cleanup_subtitle_sidecars(&output_path, &request.profile.sub_langs);
        }
        if request.profile.mode == DownloadMode::Video
            && !request.profile.audio_langs.is_empty()
            && let Err(err) = retag_audio_languages(
                &paths,
                &deps,
                &output_path,
                &request.profile.audio_langs,
                Some(cancel.clone()),
            )
            .await
        {
            tracing::warn!(%err, "could not retag audio languages");
        }
        return Ok(());
    }
    last_result
}

#[derive(Default)]
struct ProgressState {
    output_path: PathBuf,
    last_title: String,
    error_text: String,
}

async fn run_ytdlp_download(
    bin: &str,
    args: &[String],
    slot: usize,
    locale: Locale,
    tx: &mpsc::UnboundedSender<DlUpdate>,
    cancel: &CancellationToken,
    progress: &mut ProgressState,
) -> Result<()> {
    let status = run_lines(bin, args, None, Some(cancel.clone()), |line| {
        handle_line(line, slot, locale, tx, progress)
    })
    .await
    .map_err(ProcessError::into_anyhow)?;
    if !status.success() {
        let message = if progress.error_text.is_empty() {
            format!("yt-dlp exited with status {status}")
        } else {
            progress.error_text.clone()
        };
        bail!("{message}");
    }
    Ok(())
}

fn handle_line(
    line: ProcessLine,
    slot: usize,
    locale: Locale,
    tx: &mpsc::UnboundedSender<DlUpdate>,
    progress: &mut ProgressState,
) {
    if !line.stdout {
        let lower = line.text.to_ascii_lowercase();
        if lower.starts_with("error:") && progress.error_text.is_empty() {
            progress.error_text = line.text[6..].trim().to_owned();
        }
        return;
    }
    let text = line.text;
    if let Some(rest) = text.strip_prefix(MARK_START) {
        let payload = rest.strip_prefix('\t').unwrap_or(rest);
        let mut parts = payload.splitn(2, '\t');
        let title = parse_json_string(parts.next().unwrap_or_default());
        let filename = parts.next().map(parse_json_string).unwrap_or_default();
        if progress.output_path.as_os_str().is_empty() && !filename.is_empty() {
            progress.output_path = PathBuf::from(filename);
        }
        if !title.is_empty() {
            progress.last_title = title.clone();
            send(
                tx,
                DlUpdate {
                    event_type: DlEventType::Start,
                    slot,
                    text: title,
                    ..DlUpdate::default()
                },
            );
        }
    } else if let Some(rest) = text.strip_prefix(MARK_MOVED) {
        progress.output_path = PathBuf::from(parse_json_string(rest));
    } else if let Some(rest) = text.strip_prefix(MARK_POST) {
        let name = rest.trim_start_matches('\t');
        let label = postprocess_label(name, locale);
        send(
            tx,
            DlUpdate {
                event_type: DlEventType::Proc,
                slot,
                text: label,
                ..DlUpdate::default()
            },
        );
    } else if let Some(rest) = text.strip_prefix(MARK_PROGRESS) {
        let payload = rest.strip_prefix('\t').unwrap_or(rest);
        let fields: Vec<&str> = payload.splitn(6, '\t').collect();
        if fields.len() != 6 {
            return;
        }
        let done_b = parse_i64(fields[0]);
        let mut total_b = parse_i64(fields[1]);
        if total_b <= 0 {
            total_b = parse_i64(fields[2]);
        }
        let speed = parse_f64(fields[3]);
        let mut pct = fields[4]
            .trim()
            .trim_end_matches('%')
            .trim()
            .parse::<f64>()
            .unwrap_or(0.0);
        if pct <= 0.0 && total_b > 0 && done_b > 0 {
            pct = done_b as f64 / total_b as f64 * 100.0;
        }
        pct = pct.clamp(0.0, 100.0);
        let title = parse_json_string(fields[5]);
        if !title.is_empty() && title != progress.last_title {
            progress.last_title = title.clone();
            send(
                tx,
                DlUpdate {
                    event_type: DlEventType::Dest,
                    slot,
                    text: title,
                    ..DlUpdate::default()
                },
            );
        }
        send(
            tx,
            DlUpdate {
                event_type: DlEventType::Progress,
                slot,
                pct,
                done_b,
                total_b,
                speed: format_progress_speed(speed, locale),
                speed_bps: speed.max(0.0) as i64,
                ..DlUpdate::default()
            },
        );
    }
}

fn parse_i64(raw: &str) -> i64 {
    raw.trim().parse().unwrap_or(0)
}

fn parse_f64(raw: &str) -> f64 {
    raw.trim().parse().unwrap_or(0.0)
}

fn send(tx: &mpsc::UnboundedSender<DlUpdate>, update: DlUpdate) {
    let _ = tx.send(update);
}

fn prepare_dir(path: &str) -> Result<PathBuf> {
    let path = clean_abs(path);
    if path.as_os_str().is_empty() {
        bail!("directory path is empty");
    }
    std::fs::create_dir_all(&path).with_context(|| format!("create {}", path.display()))?;
    Ok(path)
}

fn playlist_output_dir(output_dir: &Path, title: &str) -> PathBuf {
    for name in [volvid_core::sanitize_dirname(title), "playlist".to_owned()] {
        let dir = output_dir.join(name);
        if std::fs::create_dir_all(&dir).is_ok() {
            return dir;
        }
    }
    output_dir.to_path_buf()
}

pub fn validate_request(request: &DownloadRequest, deps: &CheckDepsResult) -> Result<()> {
    if request.target.kind == volvid_core::TargetKind::Unknown
        || request.target.canonical_url.is_empty()
    {
        bail!("download target is required");
    }
    if request.playlist_info.is_some() && !request.force_single && request.entries.is_empty() {
        bail!("playlist entries are required");
    }
    if request.uses_playlist() && request.fragment.is_some() {
        bail!("trimming is unavailable for playlists");
    }
    if !deps.ytdlp.available {
        bail!("yt-dlp is required");
    }
    if let Some(fragment) = &request.fragment {
        if !fragment.is_valid() {
            bail!("invalid download fragment");
        }
        volvid_core::validate_fragment_duration(fragment, request.media_duration)
            .map_err(|err| anyhow!(err))?;
    }
    if request.profile.needs_ffmpeg(request.fragment.as_ref()) && !deps.ffmpeg.available {
        bail!("ffmpeg is required for this download");
    }
    Ok(())
}

pub fn normalize_request(mut request: DownloadRequest, paths: &AppPaths) -> DownloadRequest {
    if request.profile.mode == DownloadMode::Video && request.profile.video_fmt_chain.is_empty() {
        request.profile.video_fmt_chain = vec![volvid_core::YTDLP_BEST_FORMAT.to_owned()];
    }
    if request.output_dir.trim().is_empty() {
        request.output_dir = paths.downloads_dir.to_string_lossy().into_owned();
    }
    request.workers = request.workers.max(1);
    request
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn normalizes_workers_and_output() {
        let paths = crate::AppPaths::detect();
        let request = DownloadRequest {
            workers: 0,
            ..DownloadRequest::default()
        };
        let normalized = normalize_request(request, &paths);
        assert_eq!(normalized.workers, 1);
        assert!(!normalized.output_dir.is_empty());
    }

    #[test]
    fn parses_progress_lines() {
        let progress = ProgressState::default();
        assert!(progress.output_path.as_os_str().is_empty());
    }
}
