use std::collections::HashSet;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::Duration;

use anyhow::{Context, Result, anyhow, bail};
use serde_json::Value;
use tokio::sync::mpsc;
use tokio_util::sync::CancellationToken;
use volvid_core::{
    CheckDepsResult, DlEventType, DlUpdate, DownloadMode, DownloadRequest, Locale, PlaylistEntry,
};

use crate::args::{
    build_download_args, download_formats, format_label, playlist_output_template,
    single_output_template,
};
use crate::deps::ytrust_bin;
use crate::ffmpeg::{cleanup_subtitle_sidecars, retag_audio_languages, transcode_video};
use crate::paths::{AppPaths, clean_abs};
use crate::process::{ProcessError, ProcessLine, run_lines};
use crate::ytrust::{format_progress_speed, postprocess_label};

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
        &output_dir,
        &single_output_template(),
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
    let mut worker_request = request.clone();
    worker_request.entries.clear();
    worker_request.playlist_info = None;
    let (jobs_tx, jobs_rx) = mpsc::channel::<PlaylistEntry>(entries.len().max(1));
    let jobs_rx = Arc::new(tokio::sync::Mutex::new(jobs_rx));
    let mut tasks = Vec::new();
    for slot in 0..workers {
        let paths = paths.clone();
        let deps = deps.clone();
        let request = worker_request.clone();
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
    let mut worker_failed = false;
    for task in tasks {
        if let Err(err) = task.await {
            worker_failed = true;
            tracing::warn!(%err, "playlist worker stopped unexpectedly");
        }
    }
    if cancel.is_cancelled() {
        bail!("operation cancelled");
    }
    if worker_failed {
        bail!("playlist worker failed");
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
    let template = playlist_output_template(&entry);
    let result = run_one(
        paths, deps, &request, slot, &entry.url, output_dir, &template, tx, cancel, encoders,
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
    output_dir: &Path,
    output_template: &str,
    tx: &mpsc::UnboundedSender<DlUpdate>,
    cancel: &CancellationToken,
    encoders: Option<&HashSet<String>>,
) -> Result<(), String> {
    let (mut formats, labels) = download_formats(request);
    if formats.is_empty() {
        formats.push(String::new());
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
            output_dir,
            output_template,
            format,
            &["--no-playlist".to_owned()],
        );
        let mut progress = ProgressState::default();
        let Some(ytrust) = ytrust_bin(&deps) else {
            return Err("ytrust is required".to_owned());
        };
        let result = run_ytrust_download(
            ytrust,
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
        let mut output_path = progress.output_path.clone();
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
            let embedded = !request.profile.video_container.eq_ignore_ascii_case("webm");
            if embedded {
                cleanup_subtitle_sidecars(&output_path, &request.profile.sub_langs);
                for sidecar in &progress.extra_files {
                    let _ = std::fs::remove_file(sidecar);
                }
            }
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
    error_text: String,
    extra_files: Vec<PathBuf>,
}

async fn run_ytrust_download(
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
            format!("ytrust exited with status {status}")
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
    let Ok(value) = serde_json::from_str::<Value>(&line.text) else {
        return;
    };
    match value.get("event").and_then(Value::as_str) {
        Some("start") => {
            let title = value
                .get("title")
                .and_then(Value::as_str)
                .unwrap_or_default();
            if !title.is_empty() {
                send(
                    tx,
                    DlUpdate {
                        event_type: DlEventType::Start,
                        slot,
                        text: title.to_owned(),
                        ..DlUpdate::default()
                    },
                );
            }
        }
        Some("progress") => {
            let done_b = event_i64(&value, "downloaded_bytes");
            let mut total_b = event_i64(&value, "total_bytes");
            if total_b <= 0 {
                total_b = event_i64(&value, "total_bytes_estimate");
            }
            let speed = value
                .get("speed_bps")
                .and_then(Value::as_f64)
                .unwrap_or(0.0);
            let mut pct = value.get("percent").and_then(Value::as_f64).unwrap_or(0.0);
            if pct <= 0.0 && total_b > 0 && done_b > 0 {
                pct = done_b as f64 / total_b as f64 * 100.0;
            }
            let phase = value
                .get("phase")
                .and_then(Value::as_str)
                .map(|phase| postprocess_label(phase, locale))
                .unwrap_or_default();
            send(
                tx,
                DlUpdate {
                    event_type: DlEventType::Progress,
                    slot,
                    text: phase,
                    pct: pct.clamp(0.0, 100.0),
                    done_b,
                    total_b,
                    speed: format_progress_speed(speed, locale),
                    speed_bps: speed.max(0.0) as i64,
                    eta_secs: event_i64(&value, "eta_seconds"),
                    ..DlUpdate::default()
                },
            );
        }
        Some("postprocess") => {
            let status = value.get("status").and_then(Value::as_str).unwrap_or("");
            let name = value
                .get("processor")
                .and_then(Value::as_str)
                .unwrap_or_default();
            if matches!(status, "started" | "progress" | "finished" | "failed") {
                send(
                    tx,
                    DlUpdate {
                        event_type: DlEventType::Proc,
                        slot,
                        text: postprocess_label(name, locale),
                        pct: value.get("percent").and_then(Value::as_f64).unwrap_or(0.0),
                        ..DlUpdate::default()
                    },
                );
            }
        }
        Some("file") => {
            let Some(path) = value.get("path").and_then(Value::as_str) else {
                return;
            };
            match value.get("kind").and_then(Value::as_str) {
                Some("subtitle") => progress.extra_files.push(PathBuf::from(path)),
                Some("thumbnail") => progress.output_path = PathBuf::from(path),
                _ => {}
            }
        }
        Some("done") => {
            if let Some(path) = value.get("filepath").and_then(Value::as_str)
                && !path.is_empty()
            {
                progress.output_path = PathBuf::from(path);
            }
        }
        Some("error") => {
            if progress.error_text.is_empty()
                && let Some(message) = value.get("message").and_then(Value::as_str)
            {
                progress.error_text = message.to_owned();
            }
        }
        Some("fallback") => {
            // ytrust 1.1.1 retries adaptive streams with a muxed format and
            // reports it here so the UI can explain the quality change.
            send(
                tx,
                DlUpdate {
                    event_type: DlEventType::Fallback,
                    slot,
                    text: volvid_i18n::strings(locale).fallback_muxed.to_owned(),
                    ..DlUpdate::default()
                },
            );
        }
        _ => {}
    }
}

fn event_i64(value: &Value, key: &str) -> i64 {
    value.get(key).and_then(Value::as_i64).unwrap_or(0)
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
    if !deps.ytrust.available {
        bail!("ytrust is required");
    }
    if !deps.ytrust.version.is_empty()
        && !crate::deps::ytrust_version_supported(&deps.ytrust.version)
    {
        bail!(
            "ytrust {} or newer is required",
            crate::deps::YTRUST_MIN_VERSION
        );
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
        request.profile.video_fmt_chain = vec![volvid_core::BEST_VIDEO_FORMAT.to_owned()];
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
    fn parses_phase_progress_and_postprocess_percent() {
        let (tx, mut rx) = mpsc::unbounded_channel();
        let mut progress = ProgressState::default();
        let progress_event = serde_json::json!({
            "v": 1,
            "event": "progress",
            "downloaded_bytes": 10,
            "total_bytes": 100,
            "percent": 25.0,
            "eta_seconds": 42,
            "phase": "merge",
            "phase_percent": 25.0
        });
        handle_line(
            ProcessLine {
                stdout: true,
                text: progress_event.to_string(),
            },
            0,
            Locale::En,
            &tx,
            &mut progress,
        );
        let update = rx.try_recv().unwrap();
        assert_eq!(update.event_type, DlEventType::Progress);
        assert_eq!(update.eta_secs, 42);
        assert_eq!(update.text, "Merging video and audio");

        let postprocess = serde_json::json!({
            "v": 1,
            "event": "postprocess",
            "processor": "audio",
            "status": "progress",
            "percent": 60.0
        });
        handle_line(
            ProcessLine {
                stdout: true,
                text: postprocess.to_string(),
            },
            0,
            Locale::En,
            &tx,
            &mut progress,
        );
        let update = rx.try_recv().unwrap();
        assert_eq!(update.event_type, DlEventType::Proc);
        assert!((update.pct - 60.0).abs() < f64::EPSILON);
    }

    #[test]
    fn parses_ytrust_fallback_event() {
        let (tx, mut rx) = mpsc::unbounded_channel();
        let mut progress = ProgressState::default();
        let event = serde_json::json!({
            "v": 1,
            "event": "fallback",
            "reason": "http_forbidden",
            "from": {"formats": [{"format_id": "160"}]},
            "to": {"formats": [{"format_id": "18"}]}
        });
        handle_line(
            ProcessLine {
                stdout: true,
                text: event.to_string(),
            },
            0,
            Locale::En,
            &tx,
            &mut progress,
        );
        let update = rx.try_recv().unwrap();
        assert_eq!(update.event_type, DlEventType::Fallback);
        assert!(update.text.contains("muxed"));
    }

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

    #[test]
    fn parses_ytrust_progress_events() {
        let (tx, mut rx) = mpsc::unbounded_channel();
        let mut progress = ProgressState::default();
        let start = serde_json::json!({
            "v": 1,
            "event": "start",
            "title": "Test video",
            "total_bytes": 100
        });
        handle_line(
            ProcessLine {
                stdout: true,
                text: start.to_string(),
            },
            0,
            Locale::En,
            &tx,
            &mut progress,
        );
        let update = rx.try_recv().unwrap();
        assert_eq!(update.event_type, DlEventType::Start);
        assert_eq!(update.text, "Test video");

        let progress_event = serde_json::json!({
            "v": 1,
            "event": "progress",
            "downloaded_bytes": 50,
            "total_bytes": 100,
            "percent": 50.0,
            "speed_bps": 1024.0
        });
        handle_line(
            ProcessLine {
                stdout: true,
                text: progress_event.to_string(),
            },
            0,
            Locale::En,
            &tx,
            &mut progress,
        );
        let update = rx.try_recv().unwrap();
        assert_eq!(update.event_type, DlEventType::Progress);
        assert_eq!(update.done_b, 50);
        assert_eq!(update.total_b, 100);
        assert!((update.pct - 50.0).abs() < f64::EPSILON);
        assert_eq!(update.speed_bps, 1024);

        let file = serde_json::json!({
            "v": 1,
            "event": "file",
            "kind": "subtitle",
            "path": "/tmp/video.en.srt"
        });
        handle_line(
            ProcessLine {
                stdout: true,
                text: file.to_string(),
            },
            0,
            Locale::En,
            &tx,
            &mut progress,
        );
        assert_eq!(
            progress.extra_files,
            vec![PathBuf::from("/tmp/video.en.srt")]
        );

        let done = serde_json::json!({
            "v": 1,
            "event": "done",
            "filepath": "/tmp/video.mp4",
            "kind": "video"
        });
        handle_line(
            ProcessLine {
                stdout: true,
                text: done.to_string(),
            },
            0,
            Locale::En,
            &tx,
            &mut progress,
        );
        assert_eq!(progress.output_path, PathBuf::from("/tmp/video.mp4"));

        let error = serde_json::json!({
            "v": 1,
            "event": "error",
            "message": "HTTP: error sending request"
        });
        handle_line(
            ProcessLine {
                stdout: true,
                text: error.to_string(),
            },
            0,
            Locale::En,
            &tx,
            &mut progress,
        );
        assert_eq!(progress.error_text, "HTTP: error sending request");
    }
}
