use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use anyhow::{Context, Result};
use tokio::sync::mpsc;
use tokio_util::sync::CancellationToken;
use volvid_core::{CheckDepsResult, DlUpdate, DownloadRequest, Locale};
use volvid_engine::{AppPaths, Config, Engine};

use crate::state::{AppEvent, HistoryItem};

pub struct Backend {
    pub engine: Arc<Engine>,
    runtime: tokio::runtime::Runtime,
    tx: mpsc::UnboundedSender<AppEvent>,
    rx: Mutex<mpsc::UnboundedReceiver<AppEvent>>,
    cancels: Arc<Mutex<HashMap<u64, CancellationToken>>>,
    metadata_cancels: Arc<Mutex<HashMap<u64, CancellationToken>>>,
    search_cancel: Arc<Mutex<Option<CancellationToken>>>,
}

impl Backend {
    pub fn new() -> Result<Self> {
        let engine = Arc::new(Engine::new());
        let runtime = tokio::runtime::Builder::new_multi_thread()
            .worker_threads(4)
            .enable_all()
            .build()
            .context("create tokio runtime")?;
        let (tx, rx) = mpsc::unbounded_channel();
        Ok(Self {
            engine,
            runtime,
            tx,
            rx: Mutex::new(rx),
            cancels: Arc::new(Mutex::new(HashMap::new())),
            metadata_cancels: Arc::new(Mutex::new(HashMap::new())),
            search_cancel: Arc::new(Mutex::new(None)),
        })
    }

    pub fn paths(&self) -> &AppPaths {
        self.engine.paths()
    }

    pub fn app_version(&self) -> &'static str {
        self.engine.app_version()
    }

    pub fn config(&self) -> Config {
        self.engine.config()
    }

    pub fn drain_events(&self) -> Vec<AppEvent> {
        let mut events = Vec::new();
        if let Ok(mut rx) = self.rx.lock() {
            while let Ok(event) = rx.try_recv() {
                events.push(event);
            }
        }
        events
    }

    pub fn startup(&self) {
        self.refresh_deps();
        self.check_update();
    }

    pub fn refresh_deps(&self) {
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        self.runtime.spawn(async move {
            engine.invalidate_caches();
            let deps = engine.check_deps().await;
            let _ = tx.send(AppEvent::Deps(deps.clone()));
            let enriched = engine.enrich_deps(deps, None).await;
            let _ = tx.send(AppEvent::DepsEnriched(enriched));
        });
    }

    pub fn check_update(&self) {
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        self.runtime.spawn(async move {
            match engine.check_update().await {
                Ok(info) => {
                    let _ = tx.send(AppEvent::UpdateChecked(info));
                }
                Err(err) => {
                    tracing::warn!(%err, "update check failed");
                    let _ = tx.send(AppEvent::UpdateChecked(None));
                }
            }
        });
    }

    pub fn load_media(&self, probe_url: String, quality_urls: Vec<String>, op: u64) {
        let cancel = CancellationToken::new();
        self.cancel_metadata();
        if let Ok(mut cancels) = self.metadata_cancels.lock() {
            cancels.insert(op, cancel.clone());
        }
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        let metadata = self.metadata_cancels.clone();
        self.runtime.spawn(async move {
            let probe_engine = engine.clone();
            let probe_tx = tx.clone();
            let probe_cancel = cancel.clone();
            let probe_task = tokio::spawn(async move {
                let result = probe_engine
                    .probe_media(&probe_url, Some(probe_cancel.clone()))
                    .await
                    .map_err(|err| err.to_string());
                let thumbnail_url = result
                    .as_ref()
                    .ok()
                    .map(|probe| probe.thumbnail.clone())
                    .unwrap_or_default();
                let _ = probe_tx.send(AppEvent::Probe { op, result });
                if !thumbnail_url.is_empty() && !probe_cancel.is_cancelled() {
                    let thumb = probe_engine.fetch_bytes(&thumbnail_url).await;
                    let result = thumb.map_err(|err| err.to_string());
                    let _ = probe_tx.send(AppEvent::Thumbnail { op, result });
                }
            });
            let quality_engine = engine.clone();
            let quality_tx = tx.clone();
            let quality_task = tokio::spawn(async move {
                let result = quality_engine
                    .resolve_quality(&quality_urls, Some(cancel))
                    .await
                    .map_err(|err| err.to_string());
                let _ = quality_tx.send(AppEvent::Quality { op, result });
            });
            let _ = tokio::join!(probe_task, quality_task);
            if let Ok(mut cancels) = metadata.lock() {
                cancels.remove(&op);
            }
        });
    }

    pub fn fetch_playlist(&self, url: String, locale: Locale, op: u64) {
        let cancel = CancellationToken::new();
        self.cancel_metadata();
        if let Ok(mut cancels) = self.metadata_cancels.lock() {
            cancels.insert(op, cancel.clone());
        }
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        let metadata = self.metadata_cancels.clone();
        self.runtime.spawn(async move {
            let result = engine
                .fetch_playlist(&url, locale, Some(cancel))
                .await
                .map_err(|err| err.to_string());
            let _ = tx.send(AppEvent::Playlist { op, result });
            if let Ok(mut cancels) = metadata.lock() {
                cancels.remove(&op);
            }
        });
    }

    pub fn search(&self, query: String, op: u64) {
        let cancel = CancellationToken::new();
        if let Ok(mut previous) = self.search_cancel.lock() {
            if let Some(previous) = previous.take() {
                previous.cancel();
            }
            *previous = Some(cancel.clone());
        }
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        let search_cancel = self.search_cancel.clone();
        let this_cancel = cancel.clone();
        self.runtime.spawn(async move {
            let result = engine
                .search_youtube(&query, Some(cancel))
                .await
                .map_err(|err| err.to_string());
            let _ = tx.send(AppEvent::Search { op, result });
            if let Ok(mut current) = search_cancel.lock()
                && current.as_ref().is_some_and(|token| token == &this_cancel)
            {
                *current = None;
            }
        });
    }

    pub fn cancel_metadata(&self) {
        if let Ok(mut cancels) = self.metadata_cancels.lock() {
            for cancel in cancels.values() {
                cancel.cancel();
            }
            cancels.clear();
        }
    }

    pub fn start_download(
        &self,
        job_id: u64,
        run_id: u64,
        request: DownloadRequest,
        deps: CheckDepsResult,
    ) {
        let cancel = CancellationToken::new();
        if let Ok(mut cancels) = self.cancels.lock() {
            cancels.insert(job_id, cancel.clone());
        }
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        let cancels = self.cancels.clone();
        self.runtime.spawn(async move {
            let (progress_tx, mut progress_rx) = mpsc::unbounded_channel::<DlUpdate>();
            let task_cancel = cancel.clone();
            let engine_task = tokio::spawn(async move {
                engine
                    .download(request, deps, progress_tx, task_cancel)
                    .await
            });
            while let Some(update) = progress_rx.recv().await {
                let _ = tx.send(AppEvent::Download {
                    job_id,
                    run_id,
                    update,
                });
            }
            let result = match engine_task.await {
                Ok(Ok(())) => Ok(()),
                Ok(Err(err)) => Err(err.to_string()),
                Err(err) => Err(format!("download task failed: {err}")),
            };
            if let Ok(mut cancels) = cancels.lock()
                && cancels
                    .get(&job_id)
                    .is_some_and(|current| current == &cancel)
            {
                cancels.remove(&job_id);
            }
            let _ = tx.send(AppEvent::DownloadFinished {
                job_id,
                run_id,
                result,
            });
        });
    }

    pub fn cancel_download(&self, job_id: u64) {
        if let Ok(cancels) = self.cancels.lock()
            && let Some(cancel) = cancels.get(&job_id)
        {
            cancel.cancel();
        }
    }

    pub fn install_dependency(&self, key: String, locale: Locale) {
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        self.runtime.spawn(async move {
            let (progress_tx, mut progress_rx) = mpsc::unbounded_channel();
            let task = {
                let engine = engine.clone();
                let key = key.clone();
                tokio::spawn(async move {
                    engine
                        .install_dependency(&key, locale, None, progress_tx)
                        .await
                })
            };
            while let Some(progress) = progress_rx.recv().await {
                let _ = tx.send(AppEvent::InstallProgress {
                    key: key.clone(),
                    progress,
                });
            }
            let result = match task.await {
                Ok(Ok(())) => Ok(()),
                Ok(Err(err)) => Err(err.to_string()),
                Err(err) => Err(format!("install task failed: {err}")),
            };
            let _ = tx.send(AppEvent::InstallFinished { key, result });
        });
    }

    pub fn apply_update(&self, info: volvid_core::UpdateInfo, locale: Locale) {
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        self.runtime.spawn(async move {
            let (progress_tx, mut progress_rx) = mpsc::unbounded_channel();
            let task = {
                let engine = engine.clone();
                tokio::spawn(
                    async move { engine.apply_update(&info, locale, None, progress_tx).await },
                )
            };
            while let Some(progress) = progress_rx.recv().await {
                let _ = tx.send(AppEvent::UpdateProgress(progress));
            }
            let result = match task.await {
                Ok(Ok(())) => Ok(()),
                Ok(Err(err)) => Err(err.to_string()),
                Err(err) => Err(format!("update task failed: {err}")),
            };
            let _ = tx.send(AppEvent::UpdateFinished(result));
        });
    }

    pub fn pick_folder(&self, current: Option<PathBuf>, title: String) {
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        self.runtime.spawn(async move {
            let result = engine
                .pick_folder(current, &title)
                .await
                .map_err(|err| err.to_string());
            let _ = tx.send(AppEvent::FolderPicked(result));
        });
    }

    pub fn open_folder(&self, path: PathBuf) {
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        self.runtime.spawn_blocking(move || {
            let result = engine.open_folder(&path).map_err(|err| err.to_string());
            let _ = tx.send(AppEvent::OpenFolder(result));
        });
    }

    pub fn save_config(&self, config: Config) {
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        self.runtime.spawn_blocking(move || {
            let result = engine.save_config(&config).map_err(|err| err.to_string());
            let _ = tx.send(AppEvent::ConfigSaved(result));
        });
    }

    pub fn save_history(&self, history: &[HistoryItem]) -> Result<()> {
        crate::history::save_history(self.paths(), history)?;
        Ok(())
    }

    pub fn clear_history(&self) -> Result<()> {
        crate::history::clear_history(self.paths())?;
        Ok(())
    }

    pub fn shutdown(&self) {
        self.cancel_metadata();
        if let Ok(cancels) = self.cancels.lock() {
            for cancel in cancels.values() {
                cancel.cancel();
            }
        }
        if let Ok(mut search) = self.search_cancel.lock()
            && let Some(cancel) = search.take()
        {
            cancel.cancel();
        }
        self.runtime.block_on(async {
            tokio::time::sleep(Duration::from_millis(10)).await;
        });
    }
}
