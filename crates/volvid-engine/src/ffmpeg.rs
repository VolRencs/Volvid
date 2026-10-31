use std::collections::HashSet;
use std::path::{Path, PathBuf};
use std::time::Duration;

use anyhow::{Context, Result, bail};
use tokio_util::sync::CancellationToken;
use volvid_core::{CheckDepsResult, OutputProfile};

use crate::deps::ffmpeg_bin;
use crate::paths::AppPaths;
use crate::process::{ProcessError, command_output_cancellable};

const ENCODERS_TIMEOUT: Duration = Duration::from_secs(3);
const DEFAULT_HARDWARE_CRF: &str = "23";
const NVENC_PRESET: &str = "p5";

#[derive(Clone, Debug)]
struct HardwareEncoder {
    codec: &'static str,
    family: &'static str,
}

pub async fn detect_video_encoders(
    paths: &AppPaths,
    deps: &CheckDepsResult,
    cancel: Option<CancellationToken>,
) -> HashSet<String> {
    let ffmpeg = ffmpeg_bin(deps, paths);
    let args = vec!["-hide_banner".to_owned(), "-encoders".to_owned()];
    let output = command_output_cancellable(&ffmpeg, &args, Some(ENCODERS_TIMEOUT), cancel).await;
    let Ok(output) = output else {
        return HashSet::new();
    };
    let text = String::from_utf8_lossy(&output.stdout);
    let mut encoders = HashSet::new();
    for line in text.lines() {
        let fields: Vec<&str> = line.split_whitespace().collect();
        if fields.len() < 2 || !fields[0].contains('V') {
            continue;
        }
        encoders.insert(fields[1].to_owned());
    }
    encoders
}

pub async fn transcode_video(
    paths: &AppPaths,
    deps: &CheckDepsResult,
    profile: &OutputProfile,
    input: &Path,
    encoders: Option<&HashSet<String>>,
    cancel: Option<CancellationToken>,
) -> Result<Option<PathBuf>> {
    if !profile.needs_video_transcode() {
        return Ok(None);
    }
    let ffmpeg = ffmpeg_bin(deps, paths);
    let final_path = transcode_final_path(input, &profile.video_container);
    let detected;
    let encoders = match encoders {
        Some(encoders) => encoders,
        None => {
            detected = detect_video_encoders(paths, deps, cancel.clone()).await;
            &detected
        }
    };
    let commands = transcode_commands(profile, encoders);

    let mut last_error = String::new();
    for command in commands {
        if cancel.as_ref().is_some_and(|token| token.is_cancelled()) {
            bail!("operation cancelled");
        }
        let temp = create_temp_path(input, "transcode", extension_for(profile, input))?;
        let mut args = fill_transcode_input(&command, input);
        args.push(temp.to_string_lossy().into_owned());
        let output = command_output_cancellable(&ffmpeg, &args, None, cancel.clone()).await;
        let output = match output {
            Ok(output) => output,
            Err(ProcessError::Cancelled) => {
                let _ = std::fs::remove_file(&temp);
                bail!("operation cancelled");
            }
            Err(err) => {
                let _ = std::fs::remove_file(&temp);
                return Err(err.into());
            }
        };
        if !output.success {
            last_error = output.stderr.trim().to_owned();
            let _ = std::fs::remove_file(&temp);
            continue;
        }
        finalize_file(&temp, &final_path)?;
        if final_path != input {
            let _ = std::fs::remove_file(input);
        }
        return Ok(Some(final_path));
    }
    if last_error.is_empty() {
        bail!("video transcoding failed");
    }
    bail!("video transcoding failed: {last_error}");
}

fn extension_for(profile: &OutputProfile, input: &Path) -> String {
    let container = profile
        .video_container
        .trim()
        .trim_start_matches('.')
        .to_owned();
    if !container.is_empty() {
        return container;
    }
    input
        .extension()
        .map(|ext| ext.to_string_lossy().into_owned())
        .unwrap_or_else(|| "mp4".to_owned())
}

fn transcode_final_path(input: &Path, container: &str) -> PathBuf {
    let container = container.trim().trim_start_matches('.');
    if container.is_empty() {
        return input.to_path_buf();
    }
    let current = input
        .extension()
        .map(|ext| ext.to_string_lossy().into_owned())
        .unwrap_or_default();
    if current.eq_ignore_ascii_case(container) || current.is_empty() {
        if current.is_empty() {
            return PathBuf::from(format!("{}.{}", input.display(), container));
        }
        return input.to_path_buf();
    }
    input.with_extension(container)
}

fn create_temp_path(input: &Path, kind: &str, extension: String) -> Result<PathBuf> {
    let dir = input.parent().unwrap_or_else(|| Path::new("."));
    let base = input
        .file_name()
        .map(|name| name.to_string_lossy().into_owned())
        .unwrap_or_else(|| "download".to_owned());
    let temp = tempfile::Builder::new()
        .prefix(&format!(".{base}.{kind}-"))
        .suffix(&format!(".{extension}"))
        .tempfile_in(dir)?;
    let (file, path) = temp.keep().map_err(|err| err.error)?;
    drop(file);
    Ok(path)
}

fn finalize_file(temp: &Path, final_path: &Path) -> Result<()> {
    let backup = final_path.with_extension("bak-volvid");
    if final_path.exists() {
        let _ = std::fs::remove_file(&backup);
        std::fs::rename(final_path, &backup)
            .with_context(|| format!("backup {}", final_path.display()))?;
    }
    if let Err(err) = std::fs::rename(temp, final_path) {
        if backup.exists() {
            let _ = std::fs::rename(&backup, final_path);
        }
        return Err(err).with_context(|| format!("replace {}", final_path.display()));
    }
    let _ = std::fs::remove_file(&backup);
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let _ = std::fs::set_permissions(final_path, std::fs::Permissions::from_mode(0o644));
    }
    Ok(())
}

fn transcode_commands(profile: &OutputProfile, encoders: &HashSet<String>) -> Vec<Vec<String>> {
    let mut commands = Vec::new();
    for encoder in hardware_candidates(&profile.video_codec)
        .into_iter()
        .filter(|encoder| encoders.contains(encoder.codec))
    {
        let mut hw = profile.clone();
        hw.video_codec = encoder.codec.to_owned();
        commands.push(transcode_args(&hw, Some(encoder.family)));
    }
    commands.push(transcode_args(profile, None));
    commands
}

fn transcode_args(profile: &OutputProfile, hardware_family: Option<&str>) -> Vec<String> {
    let mut args = vec![
        "-y".to_owned(),
        "-hide_banner".to_owned(),
        "-loglevel".to_owned(),
        "error".to_owned(),
        "-i".to_owned(),
        "%INPUT%".to_owned(),
        "-map".to_owned(),
        "0:v:0".to_owned(),
        "-map".to_owned(),
        "0:a?".to_owned(),
        "-dn".to_owned(),
    ];
    args.extend(subtitle_args(profile));
    let video_codec = if profile.video_codec.trim().is_empty() {
        "copy"
    } else {
        profile.video_codec.trim()
    };
    args.push("-c:v".to_owned());
    args.push(video_codec.to_owned());
    if video_codec != "copy" {
        if let Some(family) = hardware_family {
            args.extend(hardware_quality_args(family, &profile.video_crf));
        } else if !profile.video_crf.trim().is_empty() {
            args.push("-crf".to_owned());
            args.push(profile.video_crf.trim().to_owned());
        }
    }
    let audio_codec = if profile.audio_codec.trim().is_empty() {
        "copy"
    } else {
        profile.audio_codec.trim()
    };
    args.push("-c:a".to_owned());
    args.push(audio_codec.to_owned());
    if audio_codec != "copy" && !profile.audio_bitrate.trim().is_empty() {
        args.push("-b:a".to_owned());
        args.push(profile.audio_bitrate.trim().to_owned());
    }
    if profile.video_container.eq_ignore_ascii_case("mp4") {
        args.push("-movflags".to_owned());
        args.push("+faststart".to_owned());
    }
    args
}

fn subtitle_args(profile: &OutputProfile) -> Vec<String> {
    if !profile.wants_subtitles() {
        return vec!["-sn".to_owned()];
    }
    let codec = if profile.video_container.eq_ignore_ascii_case("mkv") {
        "srt"
    } else {
        "mov_text"
    };
    vec![
        "-map".to_owned(),
        "0:s?".to_owned(),
        "-c:s".to_owned(),
        codec.to_owned(),
    ]
}

fn hardware_quality_args(family: &str, crf: &str) -> Vec<String> {
    let crf = if crf.trim().is_empty() {
        DEFAULT_HARDWARE_CRF
    } else {
        crf.trim()
    };
    match family {
        "nvenc" => vec![
            "-preset".to_owned(),
            NVENC_PRESET.to_owned(),
            "-rc".to_owned(),
            "vbr".to_owned(),
            "-cq".to_owned(),
            crf.to_owned(),
        ],
        "qsv" => vec!["-global_quality".to_owned(), crf.to_owned()],
        "amf" => vec![
            "-rc".to_owned(),
            "cqp".to_owned(),
            "-qp".to_owned(),
            crf.to_owned(),
        ],
        _ => Vec::new(),
    }
}

fn hardware_candidates(codec: &str) -> Vec<HardwareEncoder> {
    match codec.trim().to_ascii_lowercase().as_str() {
        "libx264" | "h264" | "avc" => vec![
            HardwareEncoder {
                codec: "h264_nvenc",
                family: "nvenc",
            },
            HardwareEncoder {
                codec: "h264_qsv",
                family: "qsv",
            },
            HardwareEncoder {
                codec: "h264_amf",
                family: "amf",
            },
        ],
        "libx265" | "h265" | "hevc" => vec![
            HardwareEncoder {
                codec: "hevc_nvenc",
                family: "nvenc",
            },
            HardwareEncoder {
                codec: "hevc_qsv",
                family: "qsv",
            },
            HardwareEncoder {
                codec: "hevc_amf",
                family: "amf",
            },
        ],
        "libsvtav1" | "libaom-av1" | "av1" => vec![
            HardwareEncoder {
                codec: "av1_nvenc",
                family: "nvenc",
            },
            HardwareEncoder {
                codec: "av1_qsv",
                family: "qsv",
            },
            HardwareEncoder {
                codec: "av1_amf",
                family: "amf",
            },
        ],
        _ => Vec::new(),
    }
}

const ISO_639_2: &[(&str, &str)] = &[
    ("en", "eng"),
    ("ru", "rus"),
    ("de", "deu"),
    ("fr", "fra"),
    ("es", "spa"),
    ("it", "ita"),
    ("pt", "por"),
    ("uk", "ukr"),
    ("be", "bel"),
    ("pl", "pol"),
    ("cs", "ces"),
    ("sk", "slk"),
    ("bg", "bul"),
    ("sr", "srp"),
    ("hr", "hrv"),
    ("ro", "ron"),
    ("hu", "hun"),
    ("el", "ell"),
    ("tr", "tur"),
    ("ar", "ara"),
    ("he", "heb"),
    ("fa", "fas"),
    ("hi", "hin"),
    ("bn", "ben"),
    ("ta", "tam"),
    ("te", "tel"),
    ("ml", "mal"),
    ("ur", "urd"),
    ("id", "ind"),
    ("ms", "msa"),
    ("vi", "vie"),
    ("th", "tha"),
    ("ko", "kor"),
    ("ja", "jpn"),
    ("zh", "zho"),
    ("nl", "nld"),
    ("sv", "swe"),
    ("no", "nor"),
    ("da", "dan"),
    ("fi", "fin"),
    ("et", "est"),
    ("lv", "lav"),
    ("lt", "lit"),
    ("ka", "kat"),
    ("hy", "hye"),
    ("az", "aze"),
    ("kk", "kaz"),
    ("uz", "uzb"),
    ("sw", "swa"),
    ("af", "afr"),
];

fn audio_lang_iso(lang: &str) -> Option<String> {
    let tag = lang.trim().to_ascii_lowercase().replace('_', "-");
    if tag.is_empty() || tag == "all" {
        return None;
    }
    let primary = tag.split('-').next().unwrap_or_default();
    if primary.len() == 3 && primary.chars().all(|ch| ch.is_ascii_alphabetic()) {
        return Some(primary.to_owned());
    }
    ISO_639_2
        .iter()
        .find(|(code, _)| *code == primary)
        .map(|(_, iso)| (*iso).to_owned())
}

pub async fn retag_audio_languages(
    paths: &AppPaths,
    deps: &CheckDepsResult,
    video_path: &Path,
    langs: &[String],
    cancel: Option<CancellationToken>,
) -> Result<()> {
    if langs.is_empty() {
        return Ok(());
    }
    let ffmpeg = ffmpeg_bin(deps, paths);
    let mut iso_langs = Vec::with_capacity(langs.len());
    let mut mappable = 0;
    for lang in langs {
        let iso = audio_lang_iso(lang);
        mappable += usize::from(iso.is_some());
        iso_langs.push(iso.unwrap_or_default());
    }
    if mappable == 0 {
        return Ok(());
    }
    let ext = video_path
        .extension()
        .map(|ext| ext.to_string_lossy().into_owned())
        .unwrap_or_else(|| "mp4".to_owned());
    let temp = create_temp_path(video_path, "retag", ext.clone())?;
    let mut args = vec![
        "-y".to_owned(),
        "-hide_banner".to_owned(),
        "-loglevel".to_owned(),
        "error".to_owned(),
        "-i".to_owned(),
        video_path.to_string_lossy().into_owned(),
        "-map".to_owned(),
        "0".to_owned(),
        "-c".to_owned(),
        "copy".to_owned(),
    ];
    if ext.eq_ignore_ascii_case("mp4") {
        args.push("-movflags".to_owned());
        args.push("+faststart".to_owned());
    }
    for (index, iso) in iso_langs.iter().enumerate() {
        if !iso.is_empty() {
            args.push(format!("-metadata:s:a:{index}"));
            args.push(format!("language={iso}"));
        }
    }
    args.push(temp.to_string_lossy().into_owned());
    let output = match command_output_cancellable(&ffmpeg, &args, None, cancel.clone()).await {
        Ok(output) => output,
        Err(ProcessError::Cancelled) => {
            let _ = std::fs::remove_file(&temp);
            bail!("operation cancelled");
        }
        Err(err) => {
            let _ = std::fs::remove_file(&temp);
            return Err(err.into());
        }
    };
    if !output.success {
        let _ = std::fs::remove_file(&temp);
        bail!("retag audio languages: {}", output.stderr.trim());
    }
    if cancel.as_ref().is_some_and(|token| token.is_cancelled()) {
        let _ = std::fs::remove_file(&temp);
        bail!("operation cancelled");
    }
    finalize_file(&temp, video_path)?;
    Ok(())
}

pub fn cleanup_subtitle_sidecars(video_path: &Path, langs: &[String]) {
    let Some(base) = video_path
        .file_stem()
        .map(|stem| stem.to_string_lossy().into_owned())
    else {
        return;
    };
    let Some(dir) = video_path.parent() else {
        return;
    };
    let mut names = Vec::new();
    if langs.iter().any(|lang| lang == "all") {
        if let Ok(entries) = std::fs::read_dir(dir) {
            for entry in entries.flatten() {
                let name = entry.file_name().to_string_lossy().into_owned();
                if (name.starts_with(&format!("{base}.")) && name.ends_with(".srt"))
                    || (name.starts_with(&format!("{base}.")) && name.ends_with(".vtt"))
                {
                    names.push(name);
                }
            }
        }
    } else {
        for lang in langs {
            let lang = lang.trim();
            if lang.is_empty() {
                continue;
            }
            for ext in [".srt", ".vtt"] {
                names.push(format!("{base}.{lang}{ext}"));
            }
        }
    }
    for name in names {
        let path = dir.join(&name);
        if path.is_file() {
            let _ = std::fs::remove_file(path);
        }
    }
}

/// Renders the ffmpeg argument template with the actual input path.
pub fn fill_transcode_input(args: &[String], input: &Path) -> Vec<String> {
    let input = input.to_string_lossy();
    args.iter()
        .map(|arg| {
            if arg == "%INPUT%" {
                input.as_ref().to_owned()
            } else {
                arg.clone()
            }
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use volvid_core::DownloadMode;

    #[test]
    fn maps_iso_languages() {
        assert_eq!(audio_lang_iso("ru").as_deref(), Some("rus"));
        assert_eq!(audio_lang_iso("en-US").as_deref(), Some("eng"));
        assert_eq!(audio_lang_iso("deu").as_deref(), Some("deu"));
        assert_eq!(audio_lang_iso("xx"), None);
    }

    #[test]
    fn builds_transcode_args() {
        let profile = OutputProfile {
            label: "test".to_owned(),
            mode: DownloadMode::Video,
            video_codec: "libx264".to_owned(),
            audio_codec: "aac".to_owned(),
            video_crf: "20".to_owned(),
            audio_bitrate: "192k".to_owned(),
            video_container: "mp4".to_owned(),
            ..OutputProfile::default()
        };
        let args = transcode_args(&profile, None);
        assert!(args.contains(&"libx264".to_owned()));
        assert!(args.contains(&"+faststart".to_owned()));
    }
}
