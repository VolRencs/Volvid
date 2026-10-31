use crate::clock::parse_flexible_timestamp;
use thiserror::Error;
use url::Url;

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum TargetKind {
    #[default]
    Unknown,
    Video,
    Playlist,
    Mixed,
}

#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct ParsedTarget {
    pub canonical_url: String,
    pub kind: TargetKind,
    pub video_id: String,
    pub playlist_id: String,
    pub url_start_at: i64,
    pub has_url_start: bool,
}

impl ParsedTarget {
    pub fn is_video(&self) -> bool {
        matches!(self.kind, TargetKind::Video | TargetKind::Mixed)
    }

    pub fn is_playlist(&self) -> bool {
        matches!(self.kind, TargetKind::Playlist | TargetKind::Mixed)
    }

    pub fn video_url(&self) -> String {
        if self.video_id.is_empty() {
            self.canonical_url.clone()
        } else {
            format!("https://www.youtube.com/watch?v={}", self.video_id)
        }
    }

    pub fn download_url(&self, force_single: bool) -> String {
        if force_single && !self.video_id.is_empty() {
            self.video_url()
        } else {
            self.canonical_url.clone()
        }
    }
}

#[derive(Debug, Error)]
pub enum TargetError {
    #[error("empty URL")]
    Empty,
    #[error("invalid URL: {0}")]
    InvalidUrl(#[from] url::ParseError),
    #[error("unsupported host {0:?}")]
    UnsupportedHost(String),
    #[error("unsupported YouTube URL")]
    Unsupported,
}

pub fn parse_target(raw: &str) -> Result<ParsedTarget, TargetError> {
    let raw = raw.trim();
    if raw.is_empty() {
        return Err(TargetError::Empty);
    }
    let normalized = if raw.contains("://") {
        raw.to_owned()
    } else {
        format!("https://{raw}")
    };
    let url = Url::parse(&normalized)?;
    let host = normalize_youtube_host(url.host_str().unwrap_or_default()).ok_or_else(|| {
        TargetError::UnsupportedHost(url.host_str().unwrap_or_default().to_owned())
    })?;

    let mut target = ParsedTarget::default();
    if host == "youtu.be" {
        target.video_id = clean_first_segment(url.path());
        target.playlist_id = query(&url, "list");
    } else {
        let path = url.path().trim_matches('/');
        let mut segments = path.splitn(2, '/');
        let head = segments.next().unwrap_or_default();
        let rest = segments.next().unwrap_or_default();
        match head {
            "watch" => {
                target.video_id = query(&url, "v");
                target.playlist_id = query(&url, "list");
            }
            "playlist" => target.playlist_id = query(&url, "list"),
            "shorts" | "live" if !rest.is_empty() => {
                target.video_id = clean_first_segment(rest);
                target.playlist_id = query(&url, "list");
            }
            _ => {}
        }
    }

    if let Some((seconds, ok)) = parse_url_start(&url) {
        target.url_start_at = seconds;
        target.has_url_start = ok;
    }

    target.kind = match (!target.video_id.is_empty(), !target.playlist_id.is_empty()) {
        (true, true) => TargetKind::Mixed,
        (true, false) => TargetKind::Video,
        (false, true) => TargetKind::Playlist,
        (false, false) => return Err(TargetError::Unsupported),
    };
    target.canonical_url = canonical_url(&target);
    Ok(target)
}

fn normalize_youtube_host(host: &str) -> Option<String> {
    let host = host.trim().to_ascii_lowercase();
    let host = host
        .strip_prefix("www.")
        .or_else(|| host.strip_prefix("m."))
        .or_else(|| host.strip_prefix("music."))
        .unwrap_or(host.as_str());
    match host {
        "youtube.com" | "youtu.be" => Some(host.to_owned()),
        _ => None,
    }
}

fn query(url: &Url, key: &str) -> String {
    url.query_pairs()
        .find(|(k, _)| k == key)
        .map(|(_, v)| v.trim().to_owned())
        .unwrap_or_default()
}

fn clean_first_segment(path: &str) -> String {
    let value = path.trim_matches('/');
    if value.is_empty() || value == "." {
        return String::new();
    }
    value.split('/').next().unwrap_or_default().to_owned()
}

fn canonical_url(target: &ParsedTarget) -> String {
    match target.kind {
        TargetKind::Playlist => format!(
            "https://www.youtube.com/playlist?list={}",
            urlencoding(&target.playlist_id)
        ),
        TargetKind::Mixed => format!(
            "https://www.youtube.com/watch?v={}&list={}",
            urlencoding(&target.video_id),
            urlencoding(&target.playlist_id)
        ),
        _ => format!(
            "https://www.youtube.com/watch?v={}",
            urlencoding(&target.video_id)
        ),
    }
}

fn urlencoding(value: &str) -> String {
    url::form_urlencoded::byte_serialize(value.as_bytes()).collect()
}

fn parse_url_start(url: &Url) -> Option<(i64, bool)> {
    for key in ["t", "start", "time_continue"] {
        if let Some(seconds) = url
            .query_pairs()
            .find(|(k, _)| k == key)
            .and_then(|(_, value)| parse_flexible_timestamp(&value))
            .filter(|seconds| *seconds > 0)
        {
            return Some((seconds, true));
        }
    }
    let fragment = url.fragment()?.trim();
    if fragment.is_empty() {
        return None;
    }
    for (key, value) in url::form_urlencoded::parse(fragment.as_bytes()) {
        if ["t", "start", "time_continue"].contains(&key.as_ref())
            && let Some(seconds) = parse_flexible_timestamp(&value)
            && seconds > 0
        {
            return Some((seconds, true));
        }
    }
    parse_flexible_timestamp(fragment)
        .filter(|seconds| *seconds > 0)
        .map(|seconds| (seconds, true))
}

/// Alias kept for readability at call sites.
#[allow(non_snake_case)]
pub fn ParseTarget(raw: &str) -> Result<ParsedTarget, TargetError> {
    parse_target(raw)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_watch_urls() {
        let target = parse_target("https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s").unwrap();
        assert_eq!(target.kind, TargetKind::Video);
        assert_eq!(target.video_id, "dQw4w9WgXcQ");
        assert_eq!(target.url_start_at, 42);
        assert!(target.has_url_start);
    }

    #[test]
    fn parses_short_urls() {
        let target = parse_target("youtu.be/dQw4w9WgXcQ").unwrap();
        assert_eq!(target.kind, TargetKind::Video);
        assert_eq!(
            target.canonical_url,
            "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
        );
    }

    #[test]
    fn parses_playlist_and_mixed() {
        let playlist = parse_target("https://youtube.com/playlist?list=PL123").unwrap();
        assert_eq!(playlist.kind, TargetKind::Playlist);
        let mixed = parse_target("https://youtube.com/watch?v=abc&list=PL123&start=1:30").unwrap();
        assert_eq!(mixed.kind, TargetKind::Mixed);
        assert_eq!(mixed.url_start_at, 90);
    }

    #[test]
    fn parses_shorts_and_live() {
        assert_eq!(
            parse_target("https://youtube.com/shorts/abc123")
                .unwrap()
                .video_id,
            "abc123"
        );
        assert_eq!(
            parse_target("https://youtube.com/live/xyz789?t=5m")
                .unwrap()
                .video_id,
            "xyz789"
        );
    }

    #[test]
    fn rejects_non_youtube() {
        assert!(parse_target("https://example.com/watch?v=abc").is_err());
        assert!(parse_target("").is_err());
    }
}
