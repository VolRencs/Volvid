//! End-to-end format matrix for the ytrust/ffmpeg engine.
//!
//! Usage:
//!   cargo run -p volvid-engine --example formats
//!   cargo run -p volvid-engine --example formats -- --case=video-h264
//!   cargo run -p volvid-engine --example formats -- --playlist
//!
//! Downloads short 5-second fragments into `/tmp/volvid-formats` and verifies
//! the resulting container/codec with ffprobe.
#![allow(clippy::disallowed_methods)]

use std::path::{Path, PathBuf};
use std::process::Command;

use anyhow::{Context, Result, bail};
use volvid_core::{DownloadFragment, DownloadRequest, Locale, OutputProfile, parse_target};
use volvid_engine::Engine;
use volvid_engine::ytrust::quality_choices_from_probe;

const VIDEO: &str = "https://www.youtube.com/watch?v=dQw4w9WgXcQ";
const PLAYLIST: &str = "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=RDdQw4w9WgXcQ";
const OUT: &str = "/tmp/volvid-formats";

#[derive(Clone, Copy)]
enum Kind {
    Video(usize),
    Audio(usize),
    Thumbnail,
    Playlist,
    PlaylistVideo,
}

struct Case {
    name: &'static str,
    kind: Kind,
    fragment: Option<(i64, i64)>,
    subtitles: bool,
}

#[tokio::main]
async fn main() -> Result<()> {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let only = args
        .iter()
        .find_map(|arg| arg.strip_prefix("--case=").map(str::to_owned));
    let playlist = args.iter().any(|arg| arg == "--playlist");
    let playlist_video = args.iter().any(|arg| arg == "--playlist-video");
    let full = args.iter().any(|arg| arg == "--full");
    let url = args
        .iter()
        .find(|arg| !arg.starts_with("--"))
        .cloned()
        .unwrap_or_else(|| VIDEO.to_owned());

    let engine = Engine::new()?;
    let deps = engine.runtime_deps().await;
    println!(
        "ytrust {} ({:?}), ffmpeg {} ({:?}), node {}",
        deps.ytrust.version,
        deps.ytrust.source,
        deps.ffmpeg.version,
        deps.ffmpeg.source,
        deps.node.version
    );
    if !deps.ytrust.available || !deps.ffmpeg.available {
        bail!("required dependencies are missing");
    }

    let probe = engine.probe_media(&url, None).await?;
    println!(
        "probe: {} ({}) formats={} subs={} audio={}",
        probe.title,
        volvid_core::format_duration(probe.duration),
        probe.formats.len(),
        probe.subtitles.len(),
        probe.audio_tracks.len()
    );
    let requested_height = args.iter().find_map(|arg| {
        arg.strip_prefix("--height=")
            .and_then(|value| value.parse::<i64>().ok())
    });
    let choices = quality_choices_from_probe(&probe)?;
    let quality = requested_height
        .and_then(|height| {
            choices
                .iter()
                .find(|choice| choice.height == height)
                .cloned()
        })
        .or_else(|| choices.last().cloned())
        .context("probe returned no quality choices")?;
    println!(
        "quality: {}p, chain={:?}",
        quality.height, quality.fmt_chain
    );
    let base = volvid_i18n::quality_profile(&quality, Locale::En);
    let video_profiles = volvid_i18n::video_output_profiles(base, Locale::En);
    let audio_profiles = volvid_i18n::audio_output_profiles(Locale::En);
    let sub_lang = probe
        .subtitles
        .iter()
        .find(|track| !track.auto)
        .or_else(|| probe.subtitles.first())
        .map(|track| track.lang.clone());

    let fragment = if full { None } else { Some((0, 5)) };
    let mut cases = vec![
        Case {
            name: "video-original-mp4",
            kind: Kind::Video(0),
            fragment,
            subtitles: false,
        },
        Case {
            name: "video-h264-mp4",
            kind: Kind::Video(1),
            fragment,
            subtitles: false,
        },
        Case {
            name: "video-h265-mp4",
            kind: Kind::Video(2),
            fragment,
            subtitles: false,
        },
        Case {
            name: "video-vp9-webm",
            kind: Kind::Video(3),
            fragment,
            subtitles: false,
        },
        Case {
            name: "video-av1-mkv",
            kind: Kind::Video(4),
            fragment,
            subtitles: false,
        },
        Case {
            name: "video-mkv-copy",
            kind: Kind::Video(5),
            fragment,
            subtitles: false,
        },
        Case {
            name: "fragment-60-65",
            kind: Kind::Video(0),
            fragment: Some((60, 65)),
            subtitles: false,
        },
        Case {
            name: "audio-mp3-320",
            kind: Kind::Audio(0),
            fragment,
            subtitles: false,
        },
        Case {
            name: "audio-mp3-192",
            kind: Kind::Audio(1),
            fragment,
            subtitles: false,
        },
        Case {
            name: "audio-m4a",
            kind: Kind::Audio(2),
            fragment,
            subtitles: false,
        },
        Case {
            name: "audio-opus",
            kind: Kind::Audio(3),
            fragment,
            subtitles: false,
        },
        Case {
            name: "audio-flac",
            kind: Kind::Audio(4),
            fragment,
            subtitles: false,
        },
        Case {
            name: "thumbnail",
            kind: Kind::Thumbnail,
            fragment: None,
            subtitles: false,
        },
    ];
    if sub_lang.is_some() {
        cases.push(Case {
            name: "subs-embed-mp4",
            kind: Kind::Video(0),
            fragment,
            subtitles: true,
        });
        cases.push(Case {
            name: "subs-sidecar-webm",
            kind: Kind::Video(3),
            fragment,
            subtitles: true,
        });
    }
    if playlist {
        cases.push(Case {
            name: "playlist-thumbnails",
            kind: Kind::Playlist,
            fragment: None,
            subtitles: false,
        });
    }
    if playlist_video {
        cases.push(Case {
            name: "playlist-videos",
            kind: Kind::PlaylistVideo,
            fragment: None,
            subtitles: false,
        });
    }

    let _ = std::fs::remove_dir_all(OUT);
    std::fs::create_dir_all(OUT)?;

    let mut failures = Vec::new();
    for case in cases {
        if let Some(only) = &only
            && only != case.name
        {
            continue;
        }
        print!("{:<24} ... ", case.name);
        let dir = Path::new(OUT).join(case.name);
        std::fs::create_dir_all(&dir)?;
        let result = run_case(
            &engine,
            &deps,
            &url,
            &dir,
            &case,
            &video_profiles,
            &audio_profiles,
            sub_lang.as_deref(),
            probe.duration,
        )
        .await;
        match result {
            Ok(summary) => println!("PASS  {summary}"),
            Err(err) => {
                println!("FAIL  {err:#}");
                failures.push(case.name);
            }
        }
    }

    println!();
    if failures.is_empty() {
        println!("all format checks passed");
        Ok(())
    } else {
        println!("failed: {}", failures.join(", "));
        bail!("{} format checks failed", failures.len());
    }
}

#[allow(clippy::too_many_arguments)]
async fn run_case(
    engine: &Engine,
    deps: &volvid_core::CheckDepsResult,
    url: &str,
    dir: &Path,
    case: &Case,
    video_profiles: &[OutputProfile],
    audio_profiles: &[OutputProfile],
    sub_lang: Option<&str>,
    duration: i64,
) -> Result<String> {
    if matches!(case.kind, Kind::Playlist) {
        return run_playlist(engine, deps, dir, false, video_profiles).await;
    }
    if matches!(case.kind, Kind::PlaylistVideo) {
        return run_playlist(engine, deps, dir, true, video_profiles).await;
    }
    let mut profile = match case.kind {
        Kind::Video(index) => video_profiles
            .get(index)
            .cloned()
            .with_context(|| format!("video profile {index} is missing"))?,
        Kind::Audio(index) => audio_profiles
            .get(index)
            .cloned()
            .with_context(|| format!("audio profile {index} is missing"))?,
        Kind::Thumbnail => volvid_i18n::thumbnail_profile(Locale::En),
        Kind::Playlist | Kind::PlaylistVideo => unreachable!(),
    };
    if case.subtitles {
        let lang = sub_lang.context("no subtitle track available")?;
        profile.sub_mode = volvid_core::SubtitleMode::Embed;
        profile.sub_langs = vec![lang.to_owned()];
    }
    let fragment = case.fragment.map(|(start, end)| DownloadFragment {
        start_at: start,
        end_at: Some(end),
    });
    let request = DownloadRequest {
        target: parse_target(url)?,
        profile,
        fragment,
        media_duration: duration,
        output_dir: dir.to_string_lossy().into_owned(),
        locale: Locale::En,
        ..DownloadRequest::default()
    };
    download(engine, deps, request).await?;
    verify(dir, case)
}

async fn run_playlist(
    engine: &Engine,
    deps: &volvid_core::CheckDepsResult,
    dir: &Path,
    video: bool,
    video_profiles: &[OutputProfile],
) -> Result<String> {
    let playlist = engine.fetch_playlist(PLAYLIST, Locale::En, None).await?;
    let take = if video { 2 } else { 3 };
    let entries = playlist
        .entries
        .iter()
        .take(take)
        .cloned()
        .collect::<Vec<_>>();
    println!(
        "(playlist {:?}, {} entries) ",
        playlist.title,
        playlist.entries.len()
    );
    let profile = if video {
        video_profiles
            .first()
            .cloned()
            .context("video profile is missing")?
    } else {
        volvid_i18n::thumbnail_profile(Locale::En)
    };
    let request = DownloadRequest {
        target: parse_target(PLAYLIST)?,
        profile,
        playlist_info: Some(playlist),
        entries,
        workers: take.min(3),
        output_dir: dir.to_string_lossy().into_owned(),
        locale: Locale::En,
        ..DownloadRequest::default()
    };
    download(engine, deps, request).await?;
    let files = files(dir);
    if video {
        let media = files
            .iter()
            .filter(|path| {
                path.extension().is_some_and(|ext| {
                    matches!(
                        ext.to_string_lossy().to_ascii_lowercase().as_str(),
                        "mp4" | "mkv" | "webm"
                    )
                })
            })
            .count();
        if media == 0 {
            bail!("expected playlist videos, found none");
        }
        if media < take {
            return Ok(format!(
                "{media}/{take} videos (one or more entries failed)"
            ));
        }
        return Ok(format!("{media} videos"));
    }
    let images = files
        .iter()
        .filter(|path| {
            path.extension().is_some_and(|ext| {
                matches!(
                    ext.to_string_lossy().to_ascii_lowercase().as_str(),
                    "jpg" | "jpeg" | "png" | "webp"
                )
            })
        })
        .count();
    if images < 3 {
        bail!("expected 3 playlist thumbnails, found {images}");
    }
    Ok(format!("{images} thumbnails"))
}

async fn download(
    engine: &Engine,
    deps: &volvid_core::CheckDepsResult,
    request: DownloadRequest,
) -> Result<()> {
    let (tx, mut rx) = tokio::sync::mpsc::unbounded_channel();
    let cancel = tokio_util::sync::CancellationToken::new();
    let runner = engine.download(request, deps.clone(), tx, cancel);
    tokio::pin!(runner);
    let mut events = 0usize;
    let mut last_err = String::new();
    loop {
        tokio::select! {
            result = &mut runner => {
                while let Ok(update) = rx.try_recv() {
                    if !update.err_text.is_empty() {
                        last_err = update.err_text;
                    }
                    events += 1;
                }
                result.map_err(|err| anyhow::anyhow!("{err}"))?;
                break;
            }
            update = rx.recv() => {
                let Some(update) = update else { continue };
                events += 1;
                if !update.err_text.is_empty() {
                    last_err = update.err_text;
                }
            }
        }
    }
    if events == 0 {
        bail!("no progress events");
    }
    if !last_err.is_empty() {
        eprintln!("  warning: {last_err}");
    }
    Ok(())
}

fn verify(dir: &Path, case: &Case) -> Result<String> {
    let files = files(dir);
    if files.is_empty() {
        bail!("no output files");
    }
    if matches!(case.kind, Kind::Thumbnail | Kind::Playlist) {
        let images = files
            .iter()
            .filter(|path| {
                path.extension().is_some_and(|ext| {
                    matches!(
                        ext.to_string_lossy().to_ascii_lowercase().as_str(),
                        "jpg" | "jpeg" | "png" | "webp"
                    )
                })
            })
            .count();
        if images == 0 {
            bail!("no image produced");
        }
        return Ok(format!("{images} image(s)"));
    }
    let media = files
        .iter()
        .find(|path| {
            let ext = path
                .extension()
                .map(|ext| ext.to_string_lossy().to_ascii_lowercase())
                .unwrap_or_default();
            !matches!(
                ext.as_str(),
                "jpg" | "jpeg" | "png" | "webp" | "srt" | "vtt"
            )
        })
        .cloned()
        .context("no media file")?;
    let ext = media
        .extension()
        .map(|ext| ext.to_string_lossy().to_ascii_lowercase())
        .unwrap_or_default();
    if let Some((start, end)) = case.fragment {
        let duration = media_duration(&media)?;
        let expected = (end - start) as f64;
        if (duration - expected).abs() > 2.0 {
            bail!("expected ~{expected:.0}s fragment, got {duration:.2}s");
        }
    }
    match case.kind {
        Kind::Thumbnail | Kind::Playlist | Kind::PlaylistVideo => unreachable!(),
        Kind::Audio(index) => {
            let expected = ["mp3", "mp3", "m4a", "opus", "flac"][index];
            if ext != expected {
                bail!("expected .{expected}, got .{ext} ({})", media.display());
            }
            let codec = stream_codec(&media, "a:0")?.unwrap_or_default();
            let wanted = match index {
                0 | 1 => "mp3",
                2 => "aac",
                3 => "opus",
                4 => "flac",
                _ => unreachable!(),
            };
            if !codec.contains(wanted) {
                bail!("expected audio codec {wanted}, got {codec:?}");
            }
            Ok(format!(".{ext} {codec} {}", size(&media)))
        }
        Kind::Video(index) => {
            let (expected_ext, expected_vcodec, expected_acodec) = match index {
                0 => ("mp4", None, None),
                1 => ("mp4", Some("h264"), Some("aac")),
                2 => ("mp4", Some("hevc"), Some("aac")),
                3 => ("webm", Some("vp9"), Some("opus")),
                4 => ("mkv", Some("av1"), Some("opus")),
                5 => ("mkv", None, None),
                _ => unreachable!(),
            };
            if ext != expected_ext {
                bail!("expected .{expected_ext}, got .{ext} ({})", media.display());
            }
            let vcodec = stream_codec(&media, "v:0")?.unwrap_or_default();
            if let Some(expected) = expected_vcodec
                && !vcodec.contains(expected)
            {
                bail!("expected video codec {expected}, got {vcodec:?}");
            }
            let acodec = stream_codec(&media, "a:0")?.unwrap_or_default();
            if let Some(expected) = expected_acodec
                && !acodec.contains(expected)
            {
                bail!("expected audio codec {expected}, got {acodec:?}");
            }
            if case.subtitles {
                let subs = stream_codec(&media, "s:0")?.unwrap_or_default();
                let sidecars = files
                    .iter()
                    .filter(|path| {
                        path.extension().is_some_and(|ext| {
                            matches!(
                                ext.to_string_lossy().to_ascii_lowercase().as_str(),
                                "srt" | "vtt"
                            )
                        })
                    })
                    .count();
                if expected_ext == "webm" {
                    if sidecars == 0 {
                        bail!("expected subtitle sidecar files for WebM");
                    }
                } else if subs.is_empty() {
                    bail!("expected an embedded subtitle stream");
                }
                return Ok(format!(
                    ".{ext} {vcodec} {acodec} subs={subs:?} sidecars={sidecars} {}",
                    size(&media)
                ));
            }
            Ok(format!(".{ext} {vcodec} {acodec} {}", size(&media)))
        }
    }
}

fn media_duration(file: &Path) -> Result<f64> {
    let output = Command::new("ffprobe")
        .args([
            "-v",
            "error",
            "-show_entries",
            "format=duration",
            "-of",
            "csv=p=0",
        ])
        .arg(file)
        .output()
        .context("run ffprobe duration")?;
    if !output.status.success() {
        bail!(
            "ffprobe duration failed: {}",
            String::from_utf8_lossy(&output.stderr).trim()
        );
    }
    String::from_utf8_lossy(&output.stdout)
        .trim()
        .parse()
        .context("parse ffprobe duration")
}

fn stream_codec(file: &Path, stream: &str) -> Result<Option<String>> {
    let output = Command::new("ffprobe")
        .args([
            "-v",
            "error",
            "-select_streams",
            stream,
            "-show_entries",
            "stream=codec_name",
            "-of",
            "csv=p=0",
        ])
        .arg(file)
        .output()
        .context("run ffprobe")?;
    if !output.status.success() {
        bail!(
            "ffprobe failed: {}",
            String::from_utf8_lossy(&output.stderr).trim()
        );
    }
    let text = String::from_utf8_lossy(&output.stdout).trim().to_owned();
    Ok((!text.is_empty()).then_some(text))
}

fn files(dir: &Path) -> Vec<PathBuf> {
    let mut out = Vec::new();
    if let Ok(entries) = std::fs::read_dir(dir) {
        for entry in entries.flatten() {
            let path = entry.path();
            if path.is_dir() {
                out.extend(files(&path));
            } else if path.is_file() {
                out.push(path);
            }
        }
    }
    out.sort();
    out
}

fn size(path: &Path) -> String {
    std::fs::metadata(path)
        .map(|meta| volvid_i18n::format_bytes(meta.len() as i64, Locale::En))
        .unwrap_or_default()
}
