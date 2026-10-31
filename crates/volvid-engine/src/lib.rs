//! Async engine for Volvid: yt-dlp, ffmpeg, dependency management, updates.

pub mod args;
pub mod config;
pub mod cookies;
pub mod deps;
pub mod download;
pub mod ffmpeg;
pub mod http;
pub mod install;
pub mod paths;
pub mod platform;
pub mod process;
pub mod update;
pub mod ytdlp;

use std::path::PathBuf;
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};

use anyhow::Result;
use tokio::sync::Mutex;
use tokio_util::sync::CancellationToken;
use volvid_core::{
    CheckDepsResult, DownloadRequest, Locale, MediaProbe, PlaylistInfo, QualityChoice,
    SearchResult, UpdateInfo,
};

pub use config::{Config, ConfigError, Theme};
pub use paths::AppPaths;

use crate::config::load_config;
use crate::deps::{detect_deps, enrich_deps};
use crate::download::run_download;
use crate::ffmpeg::detect_video_encoders;
use crate::ytdlp::{fetch_playlist, probe_media, resolve_quality_choices, search_youtube};

const PROBE_CACHE_TTL: Duration = Duration::from_secs(10 * 60);
const DEPS_CACHE_TTL: Duration = Duration::from_secs(60);
const RUNTIME_DEPS_TTL: Duration = Duration::from_secs(5 * 60);

pub struct Engine {
    pub paths: AppPaths,
    config: RwLock<Config>,
    pub client: reqwest::Client,
    downloads_dir: RwLock<PathBuf>,
    deps_cache: RwLock<Option<(Instant, CheckDepsResult)>>,
    runtime_deps_cache: RwLock<Option<(Instant, CheckDepsResult)>>,
    probe_cache: Mutex<std::collections::HashMap<String, (Instant, MediaProbe)>>,
    ffmpeg_encoder_cache: Mutex<Option<Arc<std::collections::HashSet<String>>>>,
}

impl Engine {
    pub fn new() -> Self {
        let paths = AppPaths::detect();
        crate::update::cleanup_stale_update_files();
        let config = load_config(&paths);
        let downloads_dir = config.downloads_dir(&paths);
        let client = reqwest::Client::builder()
            .user_agent(format!("Volvid/{}", env!("CARGO_PKG_VERSION")))
            .connect_timeout(Duration::from_secs(30))
            .timeout(Duration::from_secs(120))
            .redirect(reqwest::redirect::Policy::limited(10))
            .build()
            .unwrap_or_default();
        Self {
            paths,
            config: RwLock::new(config),
            client,
            downloads_dir: RwLock::new(downloads_dir),
            deps_cache: RwLock::new(None),
            runtime_deps_cache: RwLock::new(None),
            probe_cache: Mutex::new(std::collections::HashMap::new()),
            ffmpeg_encoder_cache: Mutex::new(None),
        }
    }

    pub fn config(&self) -> Config {
        self.config
            .read()
            .map(|config| config.clone())
            .unwrap_or_default()
    }

    pub fn paths(&self) -> &AppPaths {
        &self.paths
    }

    pub fn save_config(&self, config: &Config) -> Result<()> {
        config.save(&self.paths)?;
        if let Ok(mut current) = self.config.write() {
            *current = config.clone();
        }
        if let Some(dir) = &config.downloads_dir
            && let Ok(mut current) = self.downloads_dir.write()
        {
            *current = paths::clean_abs(dir.clone());
        }
        Ok(())
    }

    pub fn downloads_dir(&self) -> PathBuf {
        self.downloads_dir
            .read()
            .map(|dir| dir.clone())
            .unwrap_or_else(|_| self.paths.downloads_dir.clone())
    }

    pub fn detect_deps(&self) -> CheckDepsResult {
        detect_deps(&self.paths)
    }

    pub fn cached_deps(&self) -> Option<CheckDepsResult> {
        self.deps_cache
            .read()
            .ok()
            .and_then(|cache| cache.clone())
            .filter(|(at, _)| at.elapsed() < DEPS_CACHE_TTL)
            .map(|(_, deps)| deps)
    }

    pub async fn check_deps(&self) -> CheckDepsResult {
        if let Some(deps) = self.cached_deps() {
            return deps;
        }
        let deps = detect_deps(&self.paths);
        if let Ok(mut cache) = self.deps_cache.write() {
            *cache = Some((Instant::now(), deps.clone()));
        }
        deps
    }

    pub async fn enrich_deps(
        &self,
        deps: CheckDepsResult,
        cancel: Option<CancellationToken>,
    ) -> CheckDepsResult {
        let enriched = enrich_deps(deps, cancel).await;
        if let Ok(mut cache) = self.deps_cache.write() {
            *cache = Some((Instant::now(), enriched.clone()));
        }
        enriched
    }

    pub async fn runtime_deps(&self) -> CheckDepsResult {
        if let Ok(cache) = self.runtime_deps_cache.read()
            && let Some((at, deps)) = cache.as_ref()
            && at.elapsed() < RUNTIME_DEPS_TTL
        {
            return deps.clone();
        }
        let deps = self.check_deps().await;
        let enriched = self.enrich_deps(deps.clone(), None).await;
        if let Ok(mut cache) = self.runtime_deps_cache.write() {
            *cache = Some((Instant::now(), enriched.clone()));
        }
        enriched
    }

    pub fn invalidate_caches(&self) {
        if let Ok(mut cache) = self.deps_cache.write() {
            *cache = None;
        }
        if let Ok(mut cache) = self.runtime_deps_cache.write() {
            *cache = None;
        }
        if let Ok(mut cache) = self.probe_cache.try_lock() {
            cache.clear();
        }
        if let Ok(mut cache) = self.ffmpeg_encoder_cache.try_lock() {
            *cache = None;
        }
    }

    pub async fn probe_media(
        &self,
        url: &str,
        cancel: Option<CancellationToken>,
    ) -> Result<MediaProbe> {
        let key = probe_cache_key(url);
        {
            let cache = self.probe_cache.lock().await;
            if let Some((at, probe)) = cache.get(&key)
                && at.elapsed() < PROBE_CACHE_TTL
            {
                return Ok(probe.clone());
            }
        }
        let deps = self.runtime_deps().await;
        let probe = probe_media(&deps, url, cancel).await?;
        let mut cache = self.probe_cache.lock().await;
        cache.insert(key, (Instant::now(), probe.clone()));
        Ok(probe)
    }

    pub async fn fetch_playlist(
        &self,
        url: &str,
        locale: Locale,
        cancel: Option<CancellationToken>,
    ) -> Result<PlaylistInfo> {
        let deps = self.runtime_deps().await;
        fetch_playlist(&deps, url, locale, cancel).await
    }

    pub async fn fetch_bytes(&self, url: &str) -> Result<Vec<u8>> {
        let parsed = url::Url::parse(url).map_err(|err| anyhow::anyhow!("invalid URL: {err}"))?;
        if parsed.scheme() != "https" {
            anyhow::bail!("only HTTPS is allowed");
        }
        let response = self
            .client
            .get(parsed)
            .timeout(Duration::from_secs(20))
            .send()
            .await?;
        if !response.status().is_success() {
            anyhow::bail!("HTTP {}", response.status());
        }
        let bytes = response.bytes().await?;
        if bytes.len() > 12 << 20 {
            anyhow::bail!("image is too large");
        }
        Ok(bytes.to_vec())
    }

    pub async fn search_youtube(
        &self,
        query: &str,
        cancel: Option<CancellationToken>,
    ) -> Result<Vec<SearchResult>> {
        let deps = self.runtime_deps().await;
        search_youtube(&deps, query, cancel).await
    }

    pub async fn resolve_quality(
        &self,
        urls: &[String],
        cancel: Option<CancellationToken>,
    ) -> Result<Vec<QualityChoice>> {
        let deps = self.runtime_deps().await;
        resolve_quality_choices(&deps, urls, cancel).await
    }

    pub async fn download(
        &self,
        request: DownloadRequest,
        deps: CheckDepsResult,
        tx: tokio::sync::mpsc::UnboundedSender<volvid_core::DlUpdate>,
        cancel: CancellationToken,
    ) -> Result<()> {
        let encoders = if request.profile.needs_video_transcode() {
            Some(self.video_encoders(&deps).await)
        } else {
            None
        };
        run_download(
            Arc::new(self.paths.clone()),
            deps,
            request,
            tx,
            cancel,
            encoders,
        )
        .await
    }

    pub async fn video_encoders(
        &self,
        deps: &CheckDepsResult,
    ) -> Arc<std::collections::HashSet<String>> {
        let mut cache = self.ffmpeg_encoder_cache.lock().await;
        if let Some(encoders) = cache.as_ref() {
            return encoders.clone();
        }
        let encoders = Arc::new(detect_video_encoders(&self.paths, deps, None).await);
        *cache = Some(encoders.clone());
        encoders
    }

    pub fn app_version(&self) -> &'static str {
        env!("CARGO_PKG_VERSION")
    }

    pub fn check_update(
        &self,
    ) -> impl std::future::Future<Output = Result<Option<UpdateInfo>>> + '_ {
        crate::update::check_update(&self.client)
    }

    pub async fn apply_update(
        &self,
        info: &UpdateInfo,
        locale: Locale,
        cancel: Option<CancellationToken>,
        progress: tokio::sync::mpsc::UnboundedSender<volvid_core::FileProgress>,
    ) -> Result<()> {
        crate::update::apply_update(&self.client, info, locale, cancel, progress).await
    }

    pub fn open_folder(&self, path: &std::path::Path) -> Result<()> {
        crate::platform::open_folder(path)
    }

    pub async fn pick_folder(
        &self,
        current: Option<PathBuf>,
        title: &str,
    ) -> Result<Option<PathBuf>> {
        crate::platform::pick_folder(current, title).await
    }

    pub async fn install_dependency(
        &self,
        key: &str,
        locale: Locale,
        cancel: Option<CancellationToken>,
        progress: tokio::sync::mpsc::UnboundedSender<volvid_core::FileProgress>,
    ) -> Result<()> {
        crate::install::install_dependency(
            &self.paths,
            &self.client,
            key,
            locale,
            cancel,
            progress,
        )
        .await?;
        self.invalidate_caches();
        Ok(())
    }
}

impl Default for Engine {
    fn default() -> Self {
        Self::new()
    }
}

fn probe_cache_key(url: &str) -> String {
    if let Ok(target) = volvid_core::parse_target(url) {
        if !target.video_id.is_empty() {
            return target.video_url();
        }
        if !target.canonical_url.is_empty() {
            return target.canonical_url;
        }
    }
    url.trim().to_owned()
}
