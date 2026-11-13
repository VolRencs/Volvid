//! Async engine for Volvid: ytrust, ffmpeg, dependency management, updates.

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
pub mod thumbnail;
pub mod update;
pub mod ytrust;

use std::collections::{HashMap, HashSet};
use std::hash::Hash;
use std::path::PathBuf;
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};

use anyhow::{Result, bail};
use tokio::sync::Mutex;
use tokio_util::sync::CancellationToken;
use volvid_core::{
    CheckDepsResult, DownloadRequest, Locale, MediaProbe, PlaylistInfo, QualityChoice,
    SearchResult, UpdateInfo,
};

pub use config::{Config, ConfigError, Theme};
pub use paths::AppPaths;
use thumbnail::fetch_preview;
pub use thumbnail::{PreviewImage, youtube_preview_url};

use crate::deps::{detect_deps, enrich_deps};
use crate::download::run_download;
use crate::ffmpeg::detect_video_encoders;
use crate::ytrust::{
    fetch_playlist, probe_media, quality_choices_from_probe, resolve_quality_choices,
    search_youtube,
};

const PROBE_CACHE_TTL: Duration = Duration::from_secs(10 * 60);
const THUMBNAIL_CACHE_TTL: Duration = Duration::from_secs(30 * 60);
const DEPS_CACHE_TTL: Duration = Duration::from_secs(60);
const RUNTIME_DEPS_TTL: Duration = Duration::from_secs(5 * 60);
const MAX_PROBE_CACHE: usize = 64;
const MAX_THUMBNAIL_CACHE: usize = 32;

pub struct Engine {
    pub paths: AppPaths,
    config: RwLock<Config>,
    pub client: reqwest::Client,
    deps_cache: RwLock<Option<(Instant, CheckDepsResult)>>,
    runtime_deps_cache: RwLock<Option<(Instant, CheckDepsResult)>>,
    runtime_deps_lock: Mutex<()>,
    probe_cache: Mutex<HashMap<String, (Instant, MediaProbe)>>,
    thumbnail_cache: Mutex<HashMap<String, (Instant, Arc<thumbnail::PreviewImage>)>>,
    ffmpeg_encoder_cache: Mutex<Option<Arc<HashSet<String>>>>,
}

impl Engine {
    pub fn new() -> Result<Self> {
        let paths = AppPaths::detect();
        crate::update::cleanup_stale_update_files();
        let config = Config::load(&paths);
        let client = reqwest::Client::builder()
            .user_agent(format!("Volvid/{}", env!("CARGO_PKG_VERSION")))
            .connect_timeout(Duration::from_secs(30))
            .read_timeout(Duration::from_secs(60))
            .redirect(reqwest::redirect::Policy::limited(10))
            .https_only(true)
            .build()
            .map_err(|err| anyhow::anyhow!("failed to create HTTP client: {err}"))?;
        Ok(Self {
            paths,
            config: RwLock::new(config),
            client,
            deps_cache: RwLock::new(None),
            runtime_deps_cache: RwLock::new(None),
            runtime_deps_lock: Mutex::new(()),
            probe_cache: Mutex::new(HashMap::new()),
            thumbnail_cache: Mutex::new(HashMap::new()),
            ffmpeg_encoder_cache: Mutex::new(None),
        })
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
        Ok(())
    }

    fn cached_deps(&self) -> Option<CheckDepsResult> {
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
        let paths = self.paths.clone();
        let deps = tokio::task::spawn_blocking(move || detect_deps(&paths))
            .await
            .unwrap_or_else(|err| {
                tracing::warn!(%err, "dependency detection task failed");
                CheckDepsResult::default()
            });
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
        let _guard = self.runtime_deps_lock.lock().await;
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
        insert_cached(
            &mut cache,
            key,
            probe.clone(),
            PROBE_CACHE_TTL,
            MAX_PROBE_CACHE,
        );
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
        const MAX_BYTES: usize = 12 << 20;
        let parsed = url::Url::parse(url).map_err(|err| anyhow::anyhow!("invalid URL: {err}"))?;
        if parsed.scheme() != "https" {
            bail!("only HTTPS is allowed");
        }
        let response = self
            .client
            .get(parsed)
            .timeout(Duration::from_secs(20))
            .send()
            .await?;
        if !response.status().is_success() {
            bail!("HTTP {}", response.status());
        }
        if response
            .content_length()
            .is_some_and(|length| length > MAX_BYTES as u64)
        {
            bail!("response is too large");
        }
        crate::http::read_body(response, MAX_BYTES, None, "response is too large").await
    }

    pub async fn fetch_thumbnail(
        &self,
        url: &str,
        target_width: u32,
        cancel: Option<CancellationToken>,
    ) -> Result<Arc<thumbnail::PreviewImage>> {
        let key = format!("{url}#{target_width}");
        {
            let cache = self.thumbnail_cache.lock().await;
            if let Some((at, image)) = cache.get(&key)
                && at.elapsed() < THUMBNAIL_CACHE_TTL
            {
                return Ok(image.clone());
            }
        }
        let image = fetch_preview(&self.client, url, target_width, cancel.as_ref()).await?;
        let mut cache = self.thumbnail_cache.lock().await;
        insert_cached(
            &mut cache,
            key,
            image.clone(),
            THUMBNAIL_CACHE_TTL,
            MAX_THUMBNAIL_CACHE,
        );
        Ok(image)
    }

    pub async fn search_youtube(
        &self,
        query: &str,
        locale: Locale,
        cancel: Option<CancellationToken>,
    ) -> Result<Vec<SearchResult>> {
        let deps = self.runtime_deps().await;
        search_youtube(&deps, query, locale, cancel).await
    }

    pub fn quality_from_probe(&self, probe: &MediaProbe) -> Result<Vec<QualityChoice>> {
        quality_choices_from_probe(probe)
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

    async fn video_encoders(&self, deps: &CheckDepsResult) -> Arc<HashSet<String>> {
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

fn insert_cached<K: Eq + Hash + Clone, V>(
    cache: &mut HashMap<K, (Instant, V)>,
    key: K,
    value: V,
    ttl: Duration,
    max_entries: usize,
) {
    cache.retain(|_, (at, _)| at.elapsed() < ttl);
    if cache.len() >= max_entries
        && let Some(oldest) = cache
            .iter()
            .min_by_key(|(_, (at, _))| *at)
            .map(|(key, _)| key.clone())
    {
        cache.remove(&oldest);
    }
    cache.insert(key, (Instant::now(), value));
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
