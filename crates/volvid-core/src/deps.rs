use serde::{Deserialize, Serialize};

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum DependencySource {
    #[default]
    Missing,
    System,
    Managed,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct DependencyInfo {
    pub key: String,
    pub name: String,
    pub path: String,
    pub version: String,
    pub source: DependencySource,
    pub required: bool,
    pub downloadable: bool,
    pub available: bool,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct BrowserCookiesInfo {
    pub status: String,
    pub browser: String,
    pub profile_name: String,
    pub ytdlp_profile: String,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct JSRuntimeInfo {
    pub status: String,
    pub name: String,
    pub path: String,
}

pub const STATUS_ACTIVE: &str = "active";
pub const STATUS_NO_PROFILE: &str = "browser found but no usable profile";
pub const STATUS_NOT_FOUND: &str = "not found";

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct CheckDepsResult {
    pub ytdlp: DependencyInfo,
    pub ffmpeg: DependencyInfo,
    pub node: DependencyInfo,
    pub cookies: BrowserCookiesInfo,
    pub runtime: JSRuntimeInfo,
}

impl CheckDepsResult {
    pub fn dependencies(&self) -> [&DependencyInfo; 3] {
        [&self.ytdlp, &self.ffmpeg, &self.node]
    }

    pub fn missing_required(&self) -> bool {
        self.dependencies()
            .into_iter()
            .any(|dep| dep.required && !dep.available)
    }
}

/// Renders yt-dlp's shared `--ffmpeg-location` prefix.
pub fn ffmpeg_args(deps: &CheckDepsResult) -> Vec<String> {
    let bin = deps.ffmpeg.path.trim();
    if bin.is_empty() {
        Vec::new()
    } else {
        vec!["--ffmpeg-location".to_owned(), bin.to_owned()]
    }
}
