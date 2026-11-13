use volvid_core::{CheckDepsResult, DownloadMode, DownloadRequest, OutputProfile};

use crate::deps::ytrust_command_args;

pub fn stream_protocol_args() -> Vec<String> {
    vec!["--progress-json".to_owned()]
}

pub fn build_download_args(
    req: &DownloadRequest,
    deps: &CheckDepsResult,
    source_url: &str,
    output_dir: &std::path::Path,
    output_template: &str,
    format: &str,
    extra: &[String],
) -> Vec<String> {
    let mut base = stream_protocol_args();
    base.extend(mode_args(&req.profile, format));
    base.extend(reliability_args(req));
    base.push("-P".to_owned());
    base.push(output_dir.to_string_lossy().into_owned());
    base.push("-o".to_owned());
    base.push(output_template.to_owned());
    base.extend(fragment_args(req));
    base.extend(extra.iter().cloned());
    base.push(source_url.to_owned());
    ytrust_command_args(deps, &base)
}

fn mode_args(profile: &OutputProfile, format: &str) -> Vec<String> {
    match profile.mode {
        DownloadMode::Thumbnail => {
            vec!["--skip-download".to_owned(), "--write-thumbnail".to_owned()]
        }
        DownloadMode::Audio => {
            let langs: Vec<&str> = profile
                .audio_langs
                .iter()
                .map(|lang| lang.trim())
                .filter(|lang| !lang.is_empty())
                .collect();
            let format = if langs.is_empty() {
                "bestaudio/best".to_owned()
            } else {
                let legs = langs
                    .iter()
                    .map(|lang| format!("bestaudio[language={lang}]"))
                    .collect::<Vec<_>>()
                    .join("+");
                format!("{legs}/bestaudio/best")
            };
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
            } else if !profile.needs_video_transcode() {
                // Transcoding profiles let ytrust pick a container that matches
                // the downloaded streams; ffmpeg changes it to the target
                // container afterwards. Forcing e.g. WebM here would make
                // ytrust fail when YouTube only serves H.264/AAC.
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
    let mut args = vec![
        "--write-subs".to_owned(),
        "--write-auto-subs".to_owned(),
        "--sub-langs".to_owned(),
        profile.sub_langs.join(","),
        "--sub-format".to_owned(),
        "srt".to_owned(),
    ];
    // ytrust can embed subtitles into mp4/mkv containers. WebM keeps the
    // sidecar files instead of failing the whole download.
    if !profile.video_container.eq_ignore_ascii_case("webm") {
        args.push("--embed-subs".to_owned());
    }
    args
}

fn fragment_args(req: &DownloadRequest) -> Vec<String> {
    let Some(fragment) = &req.fragment else {
        return Vec::new();
    };
    let Some(section) = fragment_section_arg(fragment) else {
        return Vec::new();
    };
    vec!["--download-sections".to_owned(), section]
}

fn fragment_section_arg(fragment: &volvid_core::DownloadFragment) -> Option<String> {
    if !fragment.is_valid() {
        return None;
    }
    let start = volvid_core::format_clock(fragment.start_at);
    let end = fragment
        .end_at
        .map(volvid_core::format_clock)
        .unwrap_or_else(|| "inf".to_owned());
    Some(format!("*{start}-{end}"))
}

fn reliability_args(_req: &DownloadRequest) -> Vec<String> {
    vec!["--retries".to_owned(), "3".to_owned()]
}

pub fn download_formats(req: &DownloadRequest) -> (Vec<String>, Vec<String>) {
    if req.profile.mode != DownloadMode::Video {
        return (Vec::new(), Vec::new());
    }
    let formats = if req.profile.video_fmt_chain.is_empty() {
        vec![volvid_core::BEST_VIDEO_FORMAT.to_owned()]
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

pub fn single_output_template() -> String {
    "%(title)s [%(id)s].%(ext)s".to_owned()
}

pub fn playlist_output_template(entry: &volvid_core::PlaylistEntry) -> String {
    format!("{:03} - %(title)s [%(id)s].%(ext)s", entry.index)
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
            std::path::Path::new("/tmp"),
            "%(title)s.%(ext)s",
            "bestvideo+bestaudio/best",
            &["--no-playlist".to_owned()],
        );
        assert!(args.iter().any(|arg| arg == "--merge-output-format"));
        assert!(args.iter().any(|arg| arg == "--no-playlist"));
        assert!(args.iter().any(|arg| arg == "--progress-json"));
        assert!(args.iter().any(|arg| arg == "-P"));
        assert!(args.iter().any(|arg| arg == "-o"));
        assert!(args.iter().any(|arg| arg == "--retries"));
        for noop in [
            "--continue",
            "--part",
            "--fragment-retries",
            "--retry-sleep",
            "--abort-on-unavailable-fragments",
            "--concurrent-fragments",
            "--force-keyframes-at-cuts",
            "--windows-filenames",
        ] {
            assert!(
                !args.iter().any(|arg| arg == noop),
                "{noop} is a no-op in ytrust 1.1"
            );
        }
    }

    #[test]
    fn output_template_keeps_video_id() {
        let template = single_output_template();
        assert!(template.contains("%(title)s"));
        assert!(template.contains("%(id)s"));
        assert!(template.contains("%(ext)s"));
    }

    #[test]
    fn transcode_profiles_do_not_force_a_merge_container() {
        let mut req = request();
        req.profile.video_codec = "libvpx-vp9".to_owned();
        req.profile.audio_codec = "libopus".to_owned();
        req.profile.video_container = "webm".to_owned();
        let args = build_download_args(
            &req,
            &CheckDepsResult::default(),
            "https://youtu.be/x",
            std::path::Path::new("/tmp"),
            "%(title)s.%(ext)s",
            "bestvideo+bestaudio/best",
            &[],
        );
        assert!(!args.iter().any(|arg| arg == "--merge-output-format"));
        assert!(!args.iter().any(|arg| arg == "--remux-video"));
    }

    #[test]
    fn selects_each_dubbed_audio_language() {
        let profile = OutputProfile {
            audio_langs: vec!["en".to_owned(), "de".to_owned()],
            ..OutputProfile::default()
        };
        assert_eq!(
            apply_audio_track_selector("bestvideo+bestaudio/best", &profile),
            "bestvideo+bestaudio[language=en]+bestaudio[language=de]/best"
        );
    }

    #[test]
    fn builds_multilanguage_audio_args() {
        let mut req = request();
        req.profile.mode = DownloadMode::Audio;
        req.profile.audio_format = "mp3".to_owned();
        req.profile.audio_langs = vec!["en".to_owned(), "de".to_owned()];
        let args = build_download_args(
            &req,
            &CheckDepsResult::default(),
            "https://youtu.be/x",
            std::path::Path::new("/tmp"),
            "%(title)s.%(ext)s",
            "",
            &[],
        );
        let format = args
            .iter()
            .position(|arg| arg == "-f")
            .and_then(|index| args.get(index + 1))
            .cloned()
            .unwrap_or_default();
        assert!(format.contains("bestaudio[language=en]"));
        assert!(format.contains("bestaudio[language=de]"));
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
            std::path::Path::new("/tmp"),
            "%(title)s.%(ext)s",
            "best",
            &[],
        );
        assert!(args.iter().any(|arg| arg == "--embed-subs"));
    }

    #[test]
    fn keeps_webm_subtitles_as_sidecars() {
        let mut req = request();
        req.profile.sub_mode = SubtitleMode::Embed;
        req.profile.sub_langs = vec!["en".to_owned()];
        req.profile.video_container = "webm".to_owned();
        let args = build_download_args(
            &req,
            &CheckDepsResult::default(),
            "https://youtu.be/x",
            std::path::Path::new("/tmp"),
            "%(title)s.%(ext)s",
            "best",
            &[],
        );
        assert!(!args.iter().any(|arg| arg == "--embed-subs"));
        assert!(args.iter().any(|arg| arg == "--write-subs"));
    }
}
