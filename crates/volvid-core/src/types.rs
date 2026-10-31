use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct PlaylistEntry {
    pub index: usize,
    pub title: String,
    pub url: String,
    pub duration: i64,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct PlaylistInfo {
    pub title: String,
    pub entries: Vec<PlaylistEntry>,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct SearchResult {
    pub title: String,
    pub url: String,
    pub duration: i64,
}

/// A newer release found on GitHub.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct UpdateInfo {
    pub latest: String,
    pub download_url: String,
}

/// One dubbed/translated audio language from a probe.
#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct AudioTrack {
    pub lang: String,
}

/// One subtitle language from a probe.
#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct SubtitleTrack {
    pub lang: String,
    pub auto: bool,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct MediaFormat {
    pub height: i64,
    pub vcodec: String,
    pub acodec: String,
    pub language: String,
    pub filesize: i64,
    pub filesize_approx: i64,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct MediaProbe {
    pub duration: i64,
    pub title: String,
    pub thumbnail: String,
    pub formats: Vec<MediaFormat>,
    pub subtitles: Vec<SubtitleTrack>,
    pub audio_tracks: Vec<AudioTrack>,
}
