use volvid_core::{CheckDepsResult, DownloadMode, DownloadRequest, OutputProfile};

use crate::deps::ytdlp_command_args;

pub const MARK_START: &str = "VRDL_START";
pub const MARK_PROGRESS: &str = "VRDL_PROGRESS";
pub const MARK_POST: &str = "VRDL_POST";
pub const MARK_MOVED: &str = "VRDL_MOVED";

pub fn stream_protocol_args() -> Vec<String> {
    vec![
        "--newline".to_owned(),
        "--progress".to_owned(),
        "--progress-delta".to_owned(),
        "0.25".to_owned(),
        "--print".to_owned(),
        format!("before_dl:{MARK_START}\t%(title|)j\t%(_filename|)j"),
        "--print".to_owned(),
        format!("after_move:{MARK_MOVED}%(filepath)j"),
        "--progress-template".to_owned(),
        format!(
            "download:{MARK_PROGRESS}\t%(progress.downloaded_bytes|0)s\t%(progress.total_bytes|0)s\t%(progress.total_bytes_estimate|0)s\t%(progress.speed|0)s\t%(progress._percent_str|0)s\t%(info.title|)j"
        ),
        "--progress-template".to_owned(),
        format!("postprocess:{MARK_POST}\t%(progress.postprocessor|)s"),
    ]
}

pub fn build_download_args(
    req: &DownloadRequest,
    deps: &CheckDepsResult,
    source_url: &str,
    output_template: &str,
    format: &str,
    extra: &[String],
) -> Vec<String> {
    let mut base = stream_protocol_args();
    base.extend(mode_args(&req.profile, format));
    base.extend(reliability_args(req));
    base.push("-o".to_owned());
    base.push(output_template.to_owned());
    base.push("--windows-filenames".to_owned());
    base.extend(fragment_args(req));
    base.extend(extra.iter().cloned());
    base.push(source_url.to_owned());
    ytdlp_command_args(deps, &base)
}

fn mode_args(profile: &OutputProfile, format: &str) -> Vec<String> {
    match profile.mode {
        DownloadMode::Thumbnail => {
            vec!["--skip-download".to_owned(), "--write-thumbnail".to_owned()]
        }
        DownloadMode::Audio => {
            let mut format = "bestaudio/best".to_owned();
            if let Some(lang) = profile
                .audio_langs
                .iter()
                .map(|lang| lang.trim())
                .find(|lang| !lang.is_empty())
            {
                format = format!("bestaudio[language={lang}]/bestaudio/best");
            }
            let mut args = vec!["-f".to_owned(), format, "--extract-audio".to_owned()];
            if !profile.audio_format.trim().is_empty() {
                args.push("--audio-format".to_owned());
                args.push(profile.audio_format.trim().to_owned());
            }
            if !profile.audio_quality.trim().is_empty() {
                args.push("--audio-quality".to_owned());
                args.push(profile.audio_quality.trim().to_owned());
            }
            args
        }
        DownloadMode::Video => {
            let container = if profile.video_container.trim().is_empty() {
                "mp4"
            } else {
                profile.video_container.trim()
            };
            let format = apply_audio_track_selector(format, profile);
            let mut args = vec!["-f".to_owned(), format];
            if profile.remux_only {
                args.push("--remux-video".to_owned());
                args.push(container.to_owned());
            } else {
                args.push("--merge-output-format".to_owned());
                args.push(container.to_owned());
            }
            if profile.audio_langs.len() > 1 {
                args.push("--audio-multistreams".to_owned());
            }
            args.extend(subtitle_args(profile));
            args
        }
    }
}

fn apply_audio_track_selector(format: &str, profile: &OutputProfile) -> String {
    let langs: Vec<&str> = profile
        .audio_langs
        .iter()
        .map(|lang| lang.trim())
        .filter(|lang| !lang.is_empty())
        .collect();
    if langs.is_empty() {
        return format.to_owned();
    }
    let legs = langs
        .iter()
        .map(|lang| format!("bestaudio[language={lang}]"))
        .collect::<Vec<_>>()
        .join("+");
    format.replace("bestaudio", &legs)
}

fn subtitle_args(profile: &OutputProfile) -> Vec<String> {
    if !profile.wants_subtitles() {
        return Vec::new();
    }
    vec![
        "--write-subs".to_owned(),
        "--write-auto-subs".to_owned(),
        "--sub-langs".to_owned(),
        profile.sub_langs.join(","),
        "--sub-format".to_owned(),
        "srt".to_owned(),
        "--convert-subs".to_owned(),
        "srt".to_owned(),
        "--embed-subs".to_owned(),
    ]
}

fn fragment_args(req: &DownloadRequest) -> Vec<String> {
    let Some(fragment) = &req.fragment else {
        return Vec::new();
    };
    let Some(section) = fragment.section_arg() else {
        return Vec::new();
    };
    let mut args = vec!["--download-sections".to_owned(), section];
    if req.profile.mode != DownloadMode::Audio {
        args.push("--force-keyframes-at-cuts".to_owned());
    }
    args
}

fn reliability_args(req: &DownloadRequest) -> Vec<String> {
    let mut args = vec![
        "--continue".to_owned(),
        "--part".to_owned(),
        "--retries".to_owned(),
        "10".to_owned(),
        "--fragment-retries".to_owned(),
        "10".to_owned(),
        "--retry-sleep".to_owned(),
        "linear=1:5:2".to_owned(),
        "--abort-on-unavailable-fragments".to_owned(),
    ];
    if req.profile.mode != DownloadMode::Thumbnail {
        args.extend(["--concurrent-fragments".to_owned(), "12".to_owned()]);
    }
    args
}

pub fn download_formats(req: &DownloadRequest) -> (Vec<String>, Vec<String>) {
    if req.profile.mode != DownloadMode::Video {
        return (vec![String::new()], vec![String::new()]);
    }
    let formats = if req.profile.video_fmt_chain.is_empty() {
        vec![volvid_core::YTDLP_BEST_FORMAT.to_owned()]
    } else {
        req.profile.video_fmt_chain.clone()
    };
    (formats, req.profile.video_fmt_labels.clone())
}

pub fn format_label(format: &str, labels: &[String], index: usize) -> String {
    labels
        .get(index)
        .filter(|label| !label.trim().is_empty())
        .cloned()
        .unwrap_or_else(|| format.to_owned())
}

fn output_template(output_dir: &std::path::Path, filename: String) -> String {
    let directory = output_dir.to_string_lossy().replace('%', "%%");
    std::path::PathBuf::from(directory)
        .join(filename)
        .to_string_lossy()
        .into_owned()
}

pub fn single_output_template(output_dir: &std::path::Path) -> String {
    output_template(output_dir, "%(title)s [%(id)s].%(ext)s".to_owned())
}

pub fn playlist_output_template(
    output_dir: &std::path::Path,
    entry: &volvid_core::PlaylistEntry,
) -> String {
    output_template(
        output_dir,
        format!("{:03} - %(title)s [%(id)s].%(ext)s", entry.index),
    )
}

pub fn parse_json_string(raw: &str) -> String {
    let raw = raw.trim();
    if raw.is_empty() {
        return String::new();
    }
    serde_json::from_str::<String>(raw)
        .map(|value| value.trim().to_owned())
        .unwrap_or_else(|_| raw.trim_matches('"').to_owned())
}

#[cfg(test)]
mod tests {
    use super::*;
    use volvid_core::{OutputProfile, SubtitleMode};

    fn request() -> DownloadRequest {
        DownloadRequest {
            profile: OutputProfile {
                mode: DownloadMode::Video,
                video_container: "mp4".to_owned(),
                video_fmt_chain: vec!["bestvideo+bestaudio/best".to_owned()],
                ..OutputProfile::default()
            },
            ..DownloadRequest::default()
        }
    }

    #[test]
    fn builds_video_args() {
        let req = request();
        let args = build_download_args(
            &req,
            &CheckDepsResult::default(),
            "https://youtu.be/x",
            "%(title)s.%(ext)s",
            "bestvideo+bestaudio/best",
            &["--no-playlist".to_owned()],
        );
        assert!(args.iter().any(|arg| arg == "--merge-output-format"));
        assert!(args.iter().any(|arg| arg == "--no-playlist"));
    }

    #[test]
    fn output_template_keeps_video_id_and_escapes_percent() {
        let template = single_output_template(std::path::Path::new("/tmp/100% hits"));
        assert!(template.contains("%(id)s"));
        assert!(template.contains("100%% hits"));
    }

    #[test]
    fn builds_subtitle_args() {
        let mut req = request();
        req.profile.sub_mode = SubtitleMode::Embed;
        req.profile.sub_langs = vec!["en".to_owned()];
        let args = build_download_args(
            &req,
            &CheckDepsResult::default(),
            "https://youtu.be/x",
            "%(title)s.%(ext)s",
            "best",
            &[],
        );
        assert!(args.iter().any(|arg| arg == "--embed-subs"));
    }
}
