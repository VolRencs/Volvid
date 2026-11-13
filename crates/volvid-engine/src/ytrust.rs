use std::collections::{HashMap, HashSet};
use std::time::Duration;

use anyhow::{Context, Result, anyhow, bail};
use futures_util::stream::{self, StreamExt};
use serde_json::Value;
use tokio_util::sync::CancellationToken;
use volvid_core::{
    AudioTrack, CheckDepsResult, MediaFormat, MediaProbe, PlaylistEntry, PlaylistInfo,
    QualityChoice, SearchResult, SubtitleTrack, default_quality_choices, parse_target,
};

use crate::deps::{PLAYLIST_LIMIT, ytrust_bin, ytrust_command_args};
use crate::process::{CaptureOutput, ProcessError, command_output, run_lines};

const RADIO_PLAYLIST_LIMIT: u64 = 100;

const QUALITY_SCAN_TIMEOUT: Duration = Duration::from_secs(90);
const SEARCH_TIMEOUT: Duration = Duration::from_secs(90);
const PLAYLIST_TIMEOUT: Duration = Duration::from_secs(15 * 60);
const MAX_DETAILED_QUALITY_URLS: usize = 5;

pub async fn probe_media(
    deps: &CheckDepsResult,
    url: &str,
    cancel: Option<CancellationToken>,
) -> Result<MediaProbe> {
    let output = run_capture(
        deps,
        &[
            "--dump-single-json".to_owned(),
            "--no-playlist".to_owned(),
            url.to_owned(),
        ],
        QUALITY_SCAN_TIMEOUT,
        cancel,
    )
    .await?;
    if !output.success {
        let error = output
            .stderr
            .lines()
            .find(|line| is_error_line(line))
            .map(str::trim)
            .unwrap_or("ytrust probe failed");
        bail!("{error}");
    }
    let text = String::from_utf8_lossy(&output.stdout);
    let payload: Value = serde_json::from_str(&text).context("parse ytrust probe JSON")?;
    probe_from_value(&payload)
}

/// Runs a ytrust metadata command and captures its output.
async fn run_capture(
    deps: &CheckDepsResult,
    base: &[String],
    timeout: Duration,
    cancel: Option<CancellationToken>,
) -> Result<CaptureOutput> {
    let bin = ytrust_bin(deps).ok_or_else(|| anyhow!("ytrust is required"))?;
    let args = ytrust_command_args(deps, base);
    command_output(bin, &args, Some(timeout), cancel)
        .await
        .map_err(ProcessError::into_anyhow)
}

pub fn probe_from_value(payload: &Value) -> Result<MediaProbe> {
    let duration = payload
        .get("duration")
        .map(value_f64)
        .unwrap_or(0.0)
        .round() as i64;
    let title = payload
        .get("title")
        .and_then(Value::as_str)
        .unwrap_or_default()
        .to_owned();
    let thumbnail = payload
        .get("thumbnail")
        .and_then(Value::as_str)
        .unwrap_or_default()
        .to_owned();

    let mut formats = Vec::new();
    if let Some(items) = payload.get("formats").and_then(Value::as_array) {
        for item in items {
            formats.push(MediaFormat {
                height: value_i64(item.get("height")),
                vcodec: value_string(item.get("vcodec")),
                acodec: value_string(item.get("acodec")),
                language: value_string(item.get("language")),
                filesize: value_i64(item.get("filesize")),
                filesize_approx: value_i64(item.get("filesize_approx")),
            });
        }
    }

    let manual = payload
        .get("subtitles")
        .and_then(Value::as_object)
        .cloned()
        .unwrap_or_default();
    let automatic = payload
        .get("automatic_captions")
        .and_then(Value::as_object)
        .cloned()
        .unwrap_or_default();

    let audio_tracks = audio_tracks(&formats);
    Ok(MediaProbe {
        duration,
        title,
        thumbnail,
        formats,
        subtitles: subtitle_tracks(&manual, &automatic),
        audio_tracks,
    })
}

pub async fn fetch_playlist(
    deps: &CheckDepsResult,
    url: &str,
    locale: volvid_core::Locale,
    cancel: Option<CancellationToken>,
) -> Result<PlaylistInfo> {
    let bin = ytrust_bin(deps).ok_or_else(|| anyhow!("ytrust is required"))?;
    let args = ytrust_command_args(
        deps,
        &[
            "--flat-playlist".to_owned(),
            "--dump-json".to_owned(),
            "--quiet".to_owned(),
            "--ignore-errors".to_owned(),
            "--playlist-limit".to_owned(),
            playlist_limit_for(url).to_string(),
            url.to_owned(),
        ],
    );
    let strings = volvid_i18n::strings(locale);
    let mut entries = Vec::new();
    let mut title = "playlist".to_owned();
    let mut error = None;
    let status = run_lines(bin, &args, Some(PLAYLIST_TIMEOUT), cancel, |line| {
        if !line.stdout {
            if error.is_none() && is_error_line(&line.text) {
                error = Some(line.text);
            }
            return;
        }
        let Ok(value) = serde_json::from_str::<Value>(&line.text) else {
            return;
        };
        if entries.is_empty() {
            title = value
                .get("playlist_title")
                .and_then(Value::as_str)
                .or_else(|| value.get("playlist").and_then(Value::as_str))
                .unwrap_or("playlist")
                .to_owned();
        }
        if let Some(entry) = media_entry(&value, entries.len() + 1, strings.video_title_fmt) {
            entries.push(entry);
        }
    })
    .await
    .map_err(ProcessError::into_anyhow)?;
    if !status.success() && entries.is_empty() {
        bail!(
            "{}",
            error.unwrap_or_else(|| "playlist empty or unavailable".to_owned())
        );
    }
    if entries.is_empty() {
        bail!("{}", strings.playlist_empty);
    }
    Ok(PlaylistInfo { title, entries })
}

pub async fn search_youtube(
    deps: &CheckDepsResult,
    query: &str,
    locale: volvid_core::Locale,
    cancel: Option<CancellationToken>,
) -> Result<Vec<SearchResult>> {
    let query = query.trim();
    if query.is_empty() {
        bail!("search query is empty");
    }
    let bin = ytrust_bin(deps).ok_or_else(|| anyhow!("ytrust is required"))?;
    let args = ytrust_command_args(
        deps,
        &[
            "--flat-playlist".to_owned(),
            "--dump-json".to_owned(),
            "--quiet".to_owned(),
            "--ignore-errors".to_owned(),
            format!("ytsearch5:{query}"),
        ],
    );
    let mut results = Vec::new();
    let mut error = None;
    let title_fmt = volvid_i18n::strings(locale).video_title_fmt;
    run_lines(bin, &args, Some(SEARCH_TIMEOUT), cancel, |line| {
        if !line.stdout {
            if error.is_none() && is_error_line(&line.text) {
                error = Some(line.text);
            }
            return;
        }
        if results.len() >= 5 {
            return;
        }
        let Ok(value) = serde_json::from_str::<Value>(&line.text) else {
            return;
        };
        if let Some(entry) = media_entry(&value, results.len() + 1, title_fmt) {
            results.push(SearchResult {
                title: entry.title,
                url: entry.url,
                duration: entry.duration,
            });
        }
    })
    .await
    .map_err(ProcessError::into_anyhow)?;
    if results.is_empty() {
        bail!(
            "{}",
            error.unwrap_or_else(|| "search returned no results".to_owned())
        );
    }
    Ok(results)
}

/// Builds quality choices from an already fetched probe (no second ytrust run).
pub fn quality_choices_from_probe(probe: &MediaProbe) -> Result<Vec<QualityChoice>> {
    Ok(build_choices(&[video_quality_info(probe)?]))
}

pub async fn resolve_quality_choices(
    deps: &CheckDepsResult,
    urls: &[String],
    cancel: Option<CancellationToken>,
) -> Result<Vec<QualityChoice>> {
    let mut unique = Vec::new();
    for url in urls {
        let url = url.trim();
        if !url.is_empty() && !unique.iter().any(|existing| existing == url) {
            unique.push(url.to_owned());
        }
    }
    if unique.is_empty() {
        bail!("quality scan: empty input");
    }
    if unique.len() > MAX_DETAILED_QUALITY_URLS {
        return Ok(default_quality_choices());
    }

    let scans = stream::iter(unique.into_iter().map(|url| {
        let cancel = cancel.clone();
        let deps = deps.clone();
        async move {
            let target = parse_target(&url)?;
            let probe_url = target.download_url(true);
            let probe = probe_media(&deps, &probe_url, cancel).await?;
            video_quality_info(&probe)
        }
    }))
    .buffer_unordered(MAX_DETAILED_QUALITY_URLS)
    .collect::<Vec<Result<VideoQualityInfo>>>()
    .await;

    let mut videos = Vec::new();
    let mut first_error = None;
    for result in scans {
        match result {
            Ok(info) if !info.heights.is_empty() => videos.push(info),
            Ok(_) => {}
            Err(err) => {
                if first_error.is_none() {
                    first_error = Some(err);
                }
            }
        }
    }
    if videos.is_empty() {
        if let Some(err) = first_error {
            return Err(err);
        }
        bail!("quality scan: no formats found");
    }
    Ok(build_choices(&videos))
}

#[derive(Clone, Debug, Default)]
struct VideoQualityInfo {
    heights: Vec<i64>,
    size_by_height: HashMap<i64, i64>,
}

fn video_quality_info(probe: &MediaProbe) -> Result<VideoQualityInfo> {
    let mut audio_size = 0i64;
    let mut heights_seen = HashSet::<i64>::new();
    let mut video_only = HashMap::<i64, i64>::new();
    let mut combined = HashMap::<i64, i64>::new();
    for format in &probe.formats {
        let size = format_size(format);
        if format.vcodec == "none" && !format.acodec.is_empty() && format.acodec != "none" {
            audio_size = audio_size.max(size);
        }
        if format.height <= 0 || format.vcodec.is_empty() || format.vcodec == "none" {
            continue;
        }
        heights_seen.insert(format.height);
        if format.acodec.is_empty() || format.acodec == "none" {
            video_only
                .entry(format.height)
                .and_modify(|current| *current = (*current).max(size))
                .or_insert(size);
        } else {
            combined
                .entry(format.height)
                .and_modify(|current| *current = (*current).max(size))
                .or_insert(size);
        }
    }
    if heights_seen.is_empty() {
        bail!("quality scan: no video heights");
    }
    let mut heights: Vec<i64> = heights_seen.into_iter().collect();
    heights.sort_unstable_by(|a, b| b.cmp(a));
    let mut size_by_height = HashMap::new();
    for height in &heights {
        let value = match (video_only.get(height), audio_size) {
            (Some(video), audio) if *video > 0 && audio > 0 => *video + audio,
            (Some(video), _) if *video > 0 => *video,
            _ => combined.get(height).copied().unwrap_or(0),
        };
        if value > 0 {
            size_by_height.insert(*height, value);
        }
    }
    Ok(VideoQualityInfo {
        heights,
        size_by_height,
    })
}

fn build_choices(videos: &[VideoQualityInfo]) -> Vec<QualityChoice> {
    let mut counts = HashMap::<i64, usize>::new();
    let mut heights = Vec::<i64>::new();
    for video in videos {
        for height in &video.heights {
            *counts.entry(*height).or_default() += 1;
            if !heights.contains(height) {
                heights.push(*height);
            }
        }
    }
    heights.sort_unstable_by(|a, b| b.cmp(a));
    let total = videos.len();
    let chains: Vec<String> = heights
        .iter()
        .map(|height| format!("bestvideo[height<={height}]+bestaudio/best[height<={height}]"))
        .collect();
    let labels: Vec<String> = heights.iter().map(|height| format!("{height}p")).collect();

    let best: Vec<Vec<i64>> = videos
        .iter()
        .map(|video| {
            let mut row = vec![0i64; heights.len() + 1];
            for index in (0..heights.len()).rev() {
                row[index] = row[index + 1];
                if let Some(size) = video.size_by_height.get(&heights[index]) {
                    row[index] = row[index].max(*size);
                }
            }
            row
        })
        .collect();

    heights
        .iter()
        .enumerate()
        .map(|(index, height)| {
            let size_bytes = best.iter().map(|row| row[index]).sum();
            QualityChoice {
                height: *height,
                best: false,
                worst: false,
                available: counts.get(height).copied().unwrap_or(0),
                total,
                size_bytes,
                fmt_chain: chains[index..].to_vec(),
                fmt_labels: labels[index..].to_vec(),
            }
        })
        .collect()
}

fn format_size(format: &MediaFormat) -> i64 {
    if format.filesize > 0 {
        format.filesize
    } else {
        format.filesize_approx
    }
}

fn media_entry(value: &Value, index: usize, title_fmt: &str) -> Option<PlaylistEntry> {
    let url = media_entry_url(value)?;
    let default_title = title_fmt.replace("%d", &index.to_string());
    let title = value
        .get("title")
        .and_then(Value::as_str)
        .or_else(|| value.get("id").and_then(Value::as_str))
        .map(str::trim)
        .filter(|title| !title.is_empty())
        .unwrap_or(&default_title)
        .to_owned();
    let duration = value.get("duration").map(value_f64).unwrap_or(0.0).round() as i64;
    Some(PlaylistEntry {
        index,
        title,
        url,
        duration,
    })
}

fn media_entry_url(value: &Value) -> Option<String> {
    for key in ["webpage_url", "original_url", "url"] {
        if let Some(raw) = value.get(key).and_then(Value::as_str) {
            let raw = raw.trim();
            if raw.is_empty() {
                continue;
            }
            if let Ok(target) = parse_target(raw) {
                return Some(if target.video_id.is_empty() {
                    target.canonical_url
                } else {
                    target.video_url()
                });
            }
            if let Some(id) = clean_entry_id(raw) {
                return Some(format!("https://youtu.be/{id}"));
            }
        }
    }
    value
        .get("id")
        .and_then(Value::as_str)
        .and_then(clean_entry_id)
        .map(|id| format!("https://youtu.be/{id}"))
}

fn clean_entry_id(raw: &str) -> Option<String> {
    let raw = raw.trim();
    if !(6..=128).contains(&raw.len())
        || !raw
            .chars()
            .all(|ch| ch.is_ascii_alphanumeric() || ch == '-' || ch == '_')
    {
        return None;
    }
    Some(raw.to_owned())
}

fn subtitle_tracks(
    manual: &serde_json::Map<String, Value>,
    automatic: &serde_json::Map<String, Value>,
) -> Vec<SubtitleTrack> {
    let mut manual_langs: Vec<String> = manual.keys().cloned().collect();
    manual_langs.sort();
    manual_langs.dedup();
    let mut auto_langs: Vec<String> = automatic
        .keys()
        .filter(|lang| !manual_langs.contains(lang))
        .cloned()
        .collect();
    auto_langs.sort();
    auto_langs.dedup();
    manual_langs
        .into_iter()
        .map(|lang| SubtitleTrack { lang, auto: false })
        .chain(
            auto_langs
                .into_iter()
                .map(|lang| SubtitleTrack { lang, auto: true }),
        )
        .collect()
}

fn audio_tracks(formats: &[MediaFormat]) -> Vec<AudioTrack> {
    let mut langs = Vec::new();
    for format in formats {
        if !format.vcodec.is_empty() && format.vcodec != "none" {
            continue;
        }
        if format.acodec.is_empty() || format.acodec == "none" {
            continue;
        }
        let lang = format.language.trim();
        if !lang.is_empty() {
            langs.push(lang.to_owned());
        }
    }
    langs.sort();
    langs.dedup();
    langs.into_iter().map(|lang| AudioTrack { lang }).collect()
}

fn value_string(value: Option<&Value>) -> String {
    match value {
        Some(Value::String(text)) => text.clone(),
        Some(Value::Number(number)) => number.to_string(),
        Some(Value::Bool(boolean)) => boolean.to_string(),
        _ => String::new(),
    }
}

fn value_i64(value: Option<&Value>) -> i64 {
    match value {
        Some(Value::Number(number)) => number.as_i64().unwrap_or(0),
        Some(Value::String(text)) => text.trim().parse().unwrap_or(0),
        _ => 0,
    }
}

fn value_f64(value: &Value) -> f64 {
    match value {
        Value::Number(number) => number.as_f64().unwrap_or(0.0),
        Value::String(text) => text.trim().parse().unwrap_or(0.0),
        _ => 0.0,
    }
}

fn is_error_line(line: &str) -> bool {
    let lower = line.to_ascii_lowercase();
    lower.starts_with("error:") || lower.starts_with("error ")
}

fn playlist_limit_for(url: &str) -> u64 {
    let radio = parse_target(url)
        .ok()
        .is_some_and(|target| target.playlist_id.starts_with("RD"));
    if radio {
        RADIO_PLAYLIST_LIMIT
    } else {
        PLAYLIST_LIMIT
    }
}

pub fn postprocess_label(name: &str, locale: volvid_core::Locale) -> String {
    let name = name.to_ascii_lowercase();
    let strings = volvid_i18n::strings(locale);
    if name.contains("audio") {
        strings.converting_audio.to_owned()
    } else if name.contains("subtitles") {
        strings.embedding_subs.to_owned()
    } else if name.contains("remux") {
        strings.remuxing.to_owned()
    } else if name.contains("download") {
        strings.downloading.to_owned()
    } else {
        strings.merging.to_owned()
    }
}

pub fn format_progress_speed(raw: f64, locale: volvid_core::Locale) -> String {
    if raw <= 0.0 {
        String::new()
    } else {
        volvid_i18n::format_speed(raw as i64, locale)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn builds_quality_choices_with_sizes() {
        let video = VideoQualityInfo {
            heights: vec![1080, 720],
            size_by_height: HashMap::from([(1080, 100), (720, 50)]),
        };
        let choices = build_choices(&[video]);
        assert_eq!(choices[0].height, 1080);
        assert_eq!(choices[0].size_bytes, 100);
        assert_eq!(choices[1].size_bytes, 50);
        assert_eq!(choices[1].fmt_chain.len(), 1);
    }

    #[test]
    fn builds_quality_from_probe_without_second_scan() {
        let probe = MediaProbe {
            duration: 10,
            formats: vec![
                MediaFormat {
                    height: 720,
                    vcodec: "avc1".to_owned(),
                    acodec: "none".to_owned(),
                    filesize: 100,
                    ..MediaFormat::default()
                },
                MediaFormat {
                    height: 0,
                    vcodec: "none".to_owned(),
                    acodec: "mp4a".to_owned(),
                    filesize: 20,
                    ..MediaFormat::default()
                },
            ],
            ..MediaProbe::default()
        };
        let choices = quality_choices_from_probe(&probe).unwrap();
        assert_eq!(choices[0].height, 720);
        assert_eq!(choices[0].size_bytes, 120);
    }

    #[test]
    fn caps_radio_mixes_but_not_regular_playlists() {
        assert_eq!(
            playlist_limit_for("https://www.youtube.com/watch?v=x&list=RDx"),
            RADIO_PLAYLIST_LIMIT
        );
        assert_eq!(
            playlist_limit_for("https://www.youtube.com/playlist?list=PL123"),
            PLAYLIST_LIMIT
        );
    }

    #[test]
    fn parses_media_entry() {
        let value: Value = serde_json::json!({
            "id": "dQw4w9WgXcQ",
            "title": "Never Gonna Give You Up",
            "duration": 212.0
        });
        let entry = media_entry(&value, 1, "Video %d").unwrap();
        assert_eq!(entry.url, "https://youtu.be/dQw4w9WgXcQ");
        assert_eq!(entry.duration, 212);
    }
}
