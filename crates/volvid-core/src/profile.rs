use crate::fragment::DownloadFragment;
use crate::locale::Locale;
use crate::target::ParsedTarget;
use serde::{Deserialize, Serialize};

pub const BEST_VIDEO_FORMAT: &str = "bestvideo+bestaudio/best";
const WORST_360_FORMAT: &str = "bestvideo[height<=360]+bestaudio/best[height<=360]";

const QUALITY_CHAINS: [&[&str]; 2] = [
    &[BEST_VIDEO_FORMAT, "best"],
    &[WORST_360_FORMAT, "best[height<=360]", "worst"],
];

/// Returns a cloned static format chain, or `None` if out of bounds.
fn quality_chain_at(index: usize) -> Option<Vec<String>> {
    QUALITY_CHAINS
        .get(index)
        .map(|chain| chain.iter().map(|s| (*s).to_owned()).collect())
}

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum DownloadMode {
    #[default]
    Video,
    Audio,
    Thumbnail,
}

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum SubtitleMode {
    #[default]
    Off,
    Embed,
}

#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct QualityChoice {
    pub height: i64,
    pub best: bool,
    pub worst: bool,
    pub available: usize,
    pub total: usize,
    pub size_bytes: i64,
    pub fmt_chain: Vec<String>,
    pub fmt_labels: Vec<String>,
}

/// Fallback choices used when a live scan is impossible.
pub fn default_quality_choices() -> Vec<QualityChoice> {
    vec![
        QualityChoice {
            height: 0,
            best: true,
            worst: false,
            available: 1,
            total: 1,
            size_bytes: 0,
            fmt_chain: quality_chain_at(0).unwrap_or_default(),
            fmt_labels: vec!["best".to_owned(), "best".to_owned()],
        },
        QualityChoice {
            height: 360,
            best: false,
            worst: true,
            available: 1,
            total: 1,
            size_bytes: 0,
            fmt_chain: quality_chain_at(1).unwrap_or_default(),
            fmt_labels: vec!["360p".to_owned(), "360p".to_owned(), "worst".to_owned()],
        },
    ]
}

#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct OutputProfile {
    pub label: String,
    pub mode: DownloadMode,
    pub video_fmt_chain: Vec<String>,
    pub video_fmt_labels: Vec<String>,
    pub video_container: String,
    pub video_codec: String,
    pub video_crf: String,
    pub audio_codec: String,
    pub audio_bitrate: String,
    pub remux_only: bool,
    pub audio_format: String,
    pub audio_quality: String,
    pub audio_langs: Vec<String>,
    pub sub_mode: SubtitleMode,
    pub sub_langs: Vec<String>,
}

impl OutputProfile {
    pub fn wants_subtitles(&self) -> bool {
        self.mode == DownloadMode::Video
            && self.sub_mode == SubtitleMode::Embed
            && !self.sub_langs.is_empty()
    }

    pub fn needs_video_transcode(&self) -> bool {
        self.mode == DownloadMode::Video
            && !self.remux_only
            && (!self.video_codec.trim().is_empty()
                || !self.audio_codec.trim().is_empty()
                || !self.video_crf.trim().is_empty()
                || !self.audio_bitrate.trim().is_empty())
    }

    fn requires_video_postprocessing(&self) -> bool {
        self.mode == DownloadMode::Video && (self.remux_only || self.needs_video_transcode())
    }

    pub fn needs_ffmpeg(&self, fragment: Option<&DownloadFragment>) -> bool {
        self.mode == DownloadMode::Audio
            || fragment.is_some()
            || self.wants_subtitles()
            || self.requires_video_postprocessing()
            || (self.mode == DownloadMode::Video
                && self
                    .video_fmt_chain
                    .iter()
                    .any(|format| format.contains('+')))
    }
}

#[derive(Clone, Debug, Default)]
pub struct DownloadRequest {
    pub target: ParsedTarget,
    pub profile: OutputProfile,
    pub fragment: Option<DownloadFragment>,
    pub media_duration: i64,
    pub force_single: bool,
    pub playlist_info: Option<crate::types::PlaylistInfo>,
    pub entries: Vec<crate::types::PlaylistEntry>,
    pub workers: usize,
    pub output_dir: String,
    pub locale: Locale,
}

impl DownloadRequest {
    pub fn uses_playlist(&self) -> bool {
        self.playlist_info.is_some() && !self.force_single && !self.entries.is_empty()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn merged_video_formats_require_ffmpeg() {
        let profile = OutputProfile {
            mode: DownloadMode::Video,
            video_fmt_chain: vec![BEST_VIDEO_FORMAT.to_owned()],
            ..OutputProfile::default()
        };
        assert!(profile.needs_ffmpeg(None));
    }
}
