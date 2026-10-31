use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use anyhow::{Context, Result};
use tokio::sync::mpsc;
use tokio_util::sync::CancellationToken;
use volvid_core::{CheckDepsResult, DlUpdate, DownloadRequest, Locale, parse_target};
use volvid_engine::{AppPaths, Config, Engine, youtube_preview_url};

use crate::state::{AppEvent, HistoryItem, ThumbnailStage};

type Repaint = Arc<dyn Fn() + Send + Sync>;

#[derive(Clone)]
struct EventSink {
    tx: mpsc::UnboundedSender<AppEvent>,
    repaint: Arc<Mutex<Option<Repaint>>>,
}

impl EventSink {
    fn send(&self, event: AppEvent) {
        let _ = self.tx.send(event);
        let repaint = self
            .repaint
            .lock()
            .ok()
            .and_then(|callback| callback.clone());
        if let Some(repaint) = repaint {
            repaint();
        }
    }
}

pub struct Backend {
    pub engine: Arc<Engine>,
    runtime: tokio::runtime::Runtime,
    sink: EventSink,
    rx: Mutex<mpsc::UnboundedReceiver<AppEvent>>,
    cancels: Arc<Mutex<HashMap<u64, CancellationToken>>>,
    metadata_cancels: Arc<Mutex<HashMap<u64, CancellationToken>>>,
    search_cancel: Arc<Mutex<Option<CancellationToken>>>,
    install_cancel: Arc<Mutex<Option<CancellationToken>>>,
    config_save_lock: Arc<tokio::sync::Mutex<()>>,
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
            sink: EventSink {
                tx,
                repaint: Arc::new(Mutex::new(None)),
            },
            rx: Mutex::new(rx),
            cancels: Arc::new(Mutex::new(HashMap::new())),
            metadata_cancels: Arc::new(Mutex::new(HashMap::new())),
            search_cancel: Arc::new(Mutex::new(None)),
            install_cancel: Arc::new(Mutex::new(None)),
            config_save_lock: Arc::new(tokio::sync::Mutex::new(())),
        })
    }

    pub fn set_repaint(&self, repaint: Repaint) {
        if let Ok(mut callback) = self.sink.repaint.lock() {
            *callback = Some(repaint);
        }
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
        let sink = self.sink.clone();
        self.runtime.spawn(async move {
            engine.invalidate_caches();
            let deps = engine.check_deps().await;
            sink.send(AppEvent::Deps(deps.clone()));
            let enriched = engine.enrich_deps(deps, None).await;
            sink.send(AppEvent::DepsEnriched(enriched));
        });
    }

    pub fn check_update(&self) {
        let engine = self.engine.clone();
        let sink = self.sink.clone();
        self.runtime.spawn(async move {
            match engine.check_update().await {
                Ok(info) => {
                    sink.send(AppEvent::UpdateChecked(info));
                }
                Err(err) => {
                    tracing::warn!(%err, "update check failed");
                    sink.send(AppEvent::UpdateChecked(None));
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
        let preview_url = parse_target(&probe_url)
            .ok()
            .filter(|target| !target.video_id.is_empty())
            .map(|target| youtube_preview_url(&target.video_id));
        let single_quality = quality_urls.len() == 1;
        let engine = self.engine.clone();
        let sink = self.sink.clone();
        let metadata = self.metadata_cancels.clone();
        self.runtime.spawn(async move {
            let preview_sink = sink.clone();
            let preview_engine = engine.clone();
            let preview_cancel = cancel.clone();
            let preview_url_for_probe = preview_url.clone();
            let preview_task = tokio::spawn(async move {
                let result = match preview_url {
                    Some(url) => preview_engine
                        .fetch_thumbnail(&url, 640, Some(preview_cancel))
                        .await
                        .map_err(|err| err.to_string()),
                    None => Err("no preview available".to_owned()),
                };
                preview_sink.send(AppEvent::Thumbnail {
                    op,
                    stage: ThumbnailStage::Preview,
                    result,
                });
            });

            let probe_engine = engine.clone();
            let probe_sink = sink.clone();
            let probe_cancel = cancel.clone();
            let probe_task = tokio::spawn(async move {
                let result = probe_engine
                    .probe_media(&probe_url, Some(probe_cancel.clone()))
                    .await;
                let quality = single_quality
                    .then(|| {
                        result
                            .as_ref()
                            .ok()
                            .and_then(|probe| probe_engine.quality_from_probe(probe).ok())
                    })
                    .flatten();
                let thumbnail_url = result
                    .as_ref()
                    .ok()
                    .map(|probe| probe.thumbnail.clone())
                    .unwrap_or_default();
                probe_sink.send(AppEvent::Probe {
                    op,
                    result: result.map_err(|err| err.to_string()),
                });
                if single_quality {
                    probe_sink.send(AppEvent::Quality {
                        op,
                        result: quality.ok_or_else(|| "quality scan failed".to_owned()),
                    });
                }
                if !thumbnail_url.is_empty()
                    && !probe_cancel.is_cancelled()
                    && preview_url_for_probe.as_deref() != Some(thumbnail_url.as_str())
                {
                    let result = probe_engine
                        .fetch_thumbnail(&thumbnail_url, 960, Some(probe_cancel.clone()))
                        .await
                        .map_err(|err| err.to_string());
                    probe_sink.send(AppEvent::Thumbnail {
                        op,
                        stage: ThumbnailStage::Full,
                        result,
                    });
                }
            });

            let quality_task = (!single_quality).then(|| {
                let quality_engine = engine.clone();
                let quality_sink = sink.clone();
                let quality_cancel = cancel.clone();
                let quality_urls = quality_urls.clone();
                tokio::spawn(async move {
                    let result = quality_engine
                        .resolve_quality(&quality_urls, Some(quality_cancel))
                        .await
                        .map_err(|err| err.to_string());
                    quality_sink.send(AppEvent::Quality { op, result });
                })
            });

            let _ = tokio::join!(probe_task, preview_task);
            if let Some(task) = quality_task {
                let _ = task.await;
            }
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
        let sink = self.sink.clone();
        let metadata = self.metadata_cancels.clone();
        self.runtime.spawn(async move {
            let result = engine
                .fetch_playlist(&url, locale, Some(cancel))
                .await
                .map_err(|err| err.to_string());
            sink.send(AppEvent::Playlist { op, result });
            if let Ok(mut cancels) = metadata.lock() {
                cancels.remove(&op);
            }
        });
    }

    pub fn search(&self, query: String, locale: Locale, op: u64) {
        let cancel = CancellationToken::new();
        if let Ok(mut previous) = self.search_cancel.lock() {
            if let Some(previous) = previous.take() {
                previous.cancel();
            }
            *previous = Some(cancel.clone());
        }
        let engine = self.engine.clone();
        let sink = self.sink.clone();
        let search_cancel = self.search_cancel.clone();
        let this_cancel = cancel.clone();
        self.runtime.spawn(async move {
            let result = engine
                .search_youtube(&query, locale, Some(cancel))
                .await
                .map_err(|err| err.to_string());
            sink.send(AppEvent::Search { op, result });
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
        let sink = self.sink.clone();
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
                sink.send(AppEvent::Download {
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
            sink.send(AppEvent::DownloadFinished {
                job_id,
                run_id,
                result,
            });
        });
    }

    pub fn is_job_running(&self, job_id: u64) -> bool {
        self.cancels
            .lock()
            .map(|cancels| cancels.contains_key(&job_id))
            .unwrap_or(false)
    }

    /// Fetches a small queue preview for a job without blocking the UI thread.
    pub fn fetch_job_preview(&self, job_id: u64, url: String) {
        let engine = self.engine.clone();
        let sink = self.sink.clone();
        self.runtime.spawn(async move {
            let result = engine
                .fetch_thumbnail(&url, 320, None)
                .await
                .map_err(|err| err.to_string());
            sink.send(AppEvent::JobThumbnail { job_id, result });
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
        let cancel = CancellationToken::new();
        {
            let Ok(mut current) = self.install_cancel.lock() else {
                return;
            };
            if current.is_some() {
                return;
            }
            *current = Some(cancel.clone());
        }
        let engine = self.engine.clone();
        let sink = self.sink.clone();
        let install_cancel = self.install_cancel.clone();
        let task_cancel = cancel.clone();
        self.runtime.spawn(async move {
            let (progress_tx, mut progress_rx) = mpsc::unbounded_channel();
            let task = {
                let engine = engine.clone();
                let key = key.clone();
                tokio::spawn(async move {
                    engine
                        .install_dependency(&key, locale, Some(task_cancel), progress_tx)
                        .await
                })
            };
            while let Some(progress) = progress_rx.recv().await {
                sink.send(AppEvent::InstallProgress {
                    key: key.clone(),
                    progress,
                });
            }
            let result = match task.await {
                Ok(Ok(())) => Ok(()),
                Ok(Err(err)) => Err(err.to_string()),
                Err(err) => Err(format!("install task failed: {err}")),
            };
            if let Ok(mut current) = install_cancel.lock()
                && current.as_ref().is_some_and(|token| token == &cancel)
            {
                *current = None;
            }
            sink.send(AppEvent::InstallFinished { key, result });
        });
    }

    pub fn cancel_install(&self) {
        if let Ok(mut current) = self.install_cancel.lock()
            && let Some(cancel) = current.take()
        {
            cancel.cancel();
        }
    }

    pub fn apply_update(&self, info: volvid_core::UpdateInfo, locale: Locale) {
        let engine = self.engine.clone();
        let sink = self.sink.clone();
        self.runtime.spawn(async move {
            let (progress_tx, mut progress_rx) = mpsc::unbounded_channel();
            let task = {
                let engine = engine.clone();
                tokio::spawn(
                    async move { engine.apply_update(&info, locale, None, progress_tx).await },
                )
            };
            while let Some(progress) = progress_rx.recv().await {
                sink.send(AppEvent::UpdateProgress(progress));
            }
            let result = match task.await {
                Ok(Ok(())) => Ok(()),
                Ok(Err(err)) => Err(err.to_string()),
                Err(err) => Err(format!("update task failed: {err}")),
            };
            sink.send(AppEvent::UpdateFinished(result));
        });
    }

    pub fn pick_folder(&self, current: Option<PathBuf>, title: String) {
        let engine = self.engine.clone();
        let sink = self.sink.clone();
        self.runtime.spawn(async move {
            let result = engine
                .pick_folder(current, &title)
                .await
                .map_err(|err| err.to_string());
            sink.send(AppEvent::FolderPicked(result));
        });
    }

    pub fn open_folder(&self, path: PathBuf) {
        let engine = self.engine.clone();
        let sink = self.sink.clone();
        self.runtime.spawn_blocking(move || {
            let result = engine.open_folder(&path).map_err(|err| err.to_string());
            sink.send(AppEvent::OpenFolder(result));
        });
    }

    pub fn save_config(&self, config: Config) {
        let engine = self.engine.clone();
        let sink = self.sink.clone();
        let lock = self.config_save_lock.clone();
        self.runtime.spawn(async move {
            let _guard = lock.lock().await;
            let result = tokio::task::spawn_blocking(move || {
                engine.save_config(&config).map_err(|err| err.to_string())
            })
            .await
            .map_err(|err| err.to_string())
            .and_then(|result| result);
            sink.send(AppEvent::ConfigSaved(result));
        });
    }

    pub fn save_history_async(&self, history: Vec<HistoryItem>) {
        let paths = self.paths().clone();
        let sink = self.sink.clone();
        self.runtime.spawn_blocking(move || {
            let result =
                crate::history::save_history(&paths, &history).map_err(|err| err.to_string());
            sink.send(AppEvent::HistorySaved(result));
        });
    }

    pub fn clear_history_async(&self) {
        let paths = self.paths().clone();
        let sink = self.sink.clone();
        self.runtime.spawn_blocking(move || {
            let result = crate::history::clear_history(&paths).map_err(|err| err.to_string());
            sink.send(AppEvent::HistorySaved(result));
        });
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
