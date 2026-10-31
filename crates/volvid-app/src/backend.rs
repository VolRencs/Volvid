use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use anyhow::{Context, Result};
use tokio::sync::mpsc;
use tokio_util::sync::CancellationToken;
use volvid_core::{CheckDepsResult, DlUpdate, DownloadRequest, Locale, ParsedTarget};
use volvid_engine::{AppPaths, Config, Engine};

use crate::state::{AppEvent, HistoryItem, ToastKind};

pub struct Backend {
    pub engine: Arc<Engine>,
    runtime: tokio::runtime::Runtime,
    tx: mpsc::UnboundedSender<AppEvent>,
    rx: Mutex<mpsc::UnboundedReceiver<AppEvent>>,
    cancels: Arc<Mutex<HashMap<u64, CancellationToken>>>,
    next_job: AtomicU64,
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
            next_job: AtomicU64::new(1),
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
                    let _ = tx.send(AppEvent::Toast(
                        format!("Update check: {err}"),
                        ToastKind::Warn,
                    ));
                    let _ = tx.send(AppEvent::UpdateChecked(None));
                }
            }
        });
    }

    pub fn load_media(
        &self,
        target: ParsedTarget,
        probe_url: String,
        quality_urls: Vec<String>,
        op: u64,
    ) {
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        self.runtime.spawn(async move {
            let probe_engine = engine.clone();
            let probe_tx = tx.clone();
            let probe_task = tokio::spawn(async move {
                let result = probe_engine
                    .probe_media(&probe_url, None)
                    .await
                    .map_err(|err| err.to_string());
                let thumbnail_url = result
                    .as_ref()
                    .ok()
                    .map(|probe| probe.thumbnail.clone())
                    .unwrap_or_default();
                let _ = probe_tx.send(AppEvent::Probe { op, result });
                if !thumbnail_url.is_empty() {
                    let thumb = probe_engine.fetch_bytes(&thumbnail_url).await;
                    let result = thumb.map_err(|err| err.to_string());
                    let _ = probe_tx.send(AppEvent::Thumbnail { op, result });
                }
            });
            let quality_engine = engine.clone();
            let quality_tx = tx.clone();
            let quality_task = tokio::spawn(async move {
                let result = quality_engine
                    .resolve_quality(&quality_urls, None)
                    .await
                    .map_err(|err| err.to_string());
                let _ = quality_tx.send(AppEvent::Quality { op, result });
            });
            let _ = tokio::join!(probe_task, quality_task);
            let _ = target;
        });
    }

    pub fn fetch_playlist(&self, url: String, locale: Locale, op: u64) {
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        self.runtime.spawn(async move {
            let result = engine
                .fetch_playlist(&url, locale, None)
                .await
                .map_err(|err| err.to_string());
            let _ = tx.send(AppEvent::Playlist { op, result });
        });
    }

    pub fn search(&self, query: String) {
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        self.runtime.spawn(async move {
            let result = engine
                .search_youtube(&query, None)
                .await
                .map_err(|err| err.to_string());
            let _ = tx.send(AppEvent::Search(result));
        });
    }

    pub fn start_download(
        &self,
        request: DownloadRequest,
        deps: CheckDepsResult,
        title: String,
        url: String,
        is_playlist: bool,
        workers: usize,
    ) -> u64 {
        let job_id = self.next_job.fetch_add(1, Ordering::Relaxed);
        let cancel = CancellationToken::new();
        if let Ok(mut cancels) = self.cancels.lock() {
            cancels.insert(job_id, cancel.clone());
        }
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        let cancels = self.cancels.clone();
        let output_dir = PathBuf::from(&request.output_dir);
        let mode = request.profile.mode;
        let total = if is_playlist {
            request.entries.len()
        } else {
            0
        };
        let _ = (title, url, is_playlist, workers, output_dir, mode, total);
        self.runtime.spawn(async move {
            let (progress_tx, mut progress_rx) = mpsc::unbounded_channel::<DlUpdate>();
            let engine_task =
                tokio::spawn(
                    async move { engine.download(request, deps, progress_tx, cancel).await },
                );
            while let Some(update) = progress_rx.recv().await {
                let _ = tx.send(AppEvent::Download { job_id, update });
            }
            let result = match engine_task.await {
                Ok(Ok(())) => Ok(()),
                Ok(Err(err)) => Err(err.to_string()),
                Err(err) => Err(format!("download task failed: {err}")),
            };
            if let Ok(mut cancels) = cancels.lock() {
                cancels.remove(&job_id);
            }
            let _ = tx.send(AppEvent::DownloadFinished { job_id, result });
        });
        job_id
    }

    pub fn cancel_download(&self, job_id: u64) {
        if let Ok(cancels) = self.cancels.lock() {
            if let Some(cancel) = cancels.get(&job_id) {
                cancel.cancel();
            }
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
                tokio::spawn(async move { engine.apply_update(&info, None, progress_tx).await })
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
            let _ = locale;
        });
    }

    pub fn pick_folder(&self, current: Option<PathBuf>) {
        let engine = self.engine.clone();
        let tx = self.tx.clone();
        self.runtime.spawn(async move {
            let result = engine
                .pick_folder(current)
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

    pub fn set_downloads_dir_and_save(&self, mut config: Config, path: PathBuf) {
        config.set_downloads_dir(path);
        self.engine
            .set_downloads_dir(config.downloads_dir.clone().unwrap_or_default());
        self.save_config(config);
    }

    pub fn save_history(&self, history: &[HistoryItem]) -> Result<()> {
        crate::history::save_history(self.paths(), history)?;
        Ok(())
    }

    pub fn clear_history(&self) -> Result<()> {
        crate::history::clear_history(self.paths())?;
        Ok(())
    }

    pub fn history_path(&self) -> PathBuf {
        self.paths().history_file()
    }

    pub fn logs_dir(&self) -> PathBuf {
        self.paths().logs_dir()
    }

    pub fn shutdown(&self) {
        if let Ok(cancels) = self.cancels.lock() {
            for cancel in cancels.values() {
                cancel.cancel();
            }
        }
        self.runtime.block_on(async {
            tokio::time::sleep(Duration::from_millis(10)).await;
        });
    }
}

#[cfg(test)]
mod tests {
    #[test]
    fn backend_type_exists() {
        // Real backend construction touches the user's home directory; the
        // state-machine tests cover the UI-facing logic instead.
    }
}
