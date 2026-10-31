use std::path::PathBuf;

use anyhow::Result;
use volvid_core::{DownloadMode, DownloadRequest, ParsedTarget, TargetKind, parse_target};
use volvid_engine::{AppPaths, Theme, youtube_preview_url};
use volvid_i18n::{default_quality, fragment_error_text};

use crate::backend::Backend;
use crate::history::load_history;
use crate::state::{
    AppEvent, AppState, Draft, FragmentMode, JobStatus, JobView, MixedChoice, Route, SlotView,
    ToastKind,
};

pub struct Controller {
    pub state: AppState,
    pub backend: Backend,
    next_op: u64,
    next_search_op: u64,
    next_job: u64,
}

impl Controller {
    pub fn new() -> Result<Self> {
        let backend = Backend::new()?;
        let config = backend.config();
        let paths = backend.paths().clone();
        let history = load_history(&paths);
        let state = AppState::new(config, &paths, history);
        let controller = Self {
            state,
            backend,
            next_op: 1,
            next_search_op: 1,
            next_job: 1,
        };
        controller.backend.startup();
        Ok(controller)
    }

    pub fn paths(&self) -> &AppPaths {
        self.backend.paths()
    }

    pub fn app_version(&self) -> &'static str {
        self.backend.app_version()
    }

    pub fn set_repaint(&self, repaint: std::sync::Arc<dyn Fn() + Send + Sync>) {
        self.backend.set_repaint(repaint);
    }

    pub fn poll(&mut self) {
        let events = self.backend.drain_events();
        let mut refresh_deps = false;
        let mut save_history = false;
        let mut save_config = false;
        for event in events {
            match &event {
                AppEvent::InstallFinished { result, .. } if result.is_ok() => {
                    refresh_deps = true;
                }
                AppEvent::DownloadFinished { .. } => save_history = true,
                AppEvent::FolderPicked(Ok(Some(_))) => save_config = true,
                _ => {}
            }
            self.state.apply(event);
        }
        if refresh_deps {
            self.backend.refresh_deps();
        }
        if save_history {
            self.backend.save_history_async(self.state.history.clone());
        }
        if save_config {
            self.backend.save_config(self.state.config.clone());
        }
        self.pump_queue();
    }

    fn next_op(&mut self) -> u64 {
        self.next_op += 1;
        self.next_op
    }

    pub fn analyze_url(&mut self) {
        let raw = self.state.url.trim().to_owned();
        if raw.is_empty() {
            self.state.url_error = Some(self.state.s().empty_url.to_owned());
            return;
        }
        let target = match volvid_core::parse_target(&raw) {
            Ok(target) => target,
            Err(err) => {
                self.state.url_error = Some(format!("{}: {err}", self.state.s().invalid_url));
                return;
            }
        };
        self.state.url = raw.clone();
        self.state.url_error = None;
        self.state.target = Some(target.clone());
        self.state.force_single = false;
        self.state.playlist_loading = false;
        self.state.quality_loading = false;
        self.state.probe_loading = false;
        self.state.mixed_choice = None;
        self.backend.cancel_metadata();
        let op = self.next_op();
        self.state.op_id = op;
        match target.kind {
            TargetKind::Mixed => {
                self.state.mixed_choice = Some(MixedChoice { target, url: raw });
            }
            TargetKind::Playlist => {
                self.state.playlist_loading = true;
                self.state.playlist = None;
                self.state.selected_entries.clear();
                self.state.route = Route::Playlist;
                self.backend.fetch_playlist(raw, self.state.locale, op);
            }
            TargetKind::Video => {
                self.start_configure(Some(target), false);
            }
            TargetKind::Unknown => {
                self.state.url_error = Some(self.state.s().unsupported_url.to_owned());
            }
        }
    }

    pub fn choose_mixed(&mut self, video_only: bool) {
        let Some(choice) = self.state.mixed_choice.take() else {
            return;
        };
        self.state.force_single = video_only;
        if video_only {
            self.start_configure(Some(choice.target), true);
        } else {
            self.backend.cancel_metadata();
            let op = self.next_op();
            self.state.op_id = op;
            self.state.playlist_loading = true;
            self.state.route = Route::Playlist;
            self.backend
                .fetch_playlist(choice.url, self.state.locale, op);
        }
    }

    pub fn start_configure(&mut self, target: Option<ParsedTarget>, force_single: bool) {
        let Some(target) = target.or_else(|| self.state.target.clone()) else {
            return;
        };
        let selected = self.state.selected_playlist_entries();
        if target.is_playlist() && !force_single && selected.is_empty() {
            self.state
                .toast(self.state.s().no_selection, ToastKind::Warn);
            return;
        }
        let op = self.next_op();
        self.state.op_id = op;
        self.state.target = Some(target.clone());
        self.state.force_single = force_single;
        self.state.route = Route::Configure;
        self.state.draft = Draft {
            workers: self.state.config.playlist_workers,
            ..Draft::default()
        };
        let defaults = default_quality(self.state.locale);
        self.state.draft.quality = defaults.first().cloned();
        self.state.quality_choices = defaults;
        self.state.media_duration = 0;
        self.state.media_title.clear();
        self.state.media_thumbnail.clear();
        self.state.preview_image = None;
        self.state.preview_generation = self.state.preview_generation.wrapping_add(1);
        self.state.thumbnail_loading = true;
        self.state.thumbnail_error = false;
        self.state.audio_tracks.clear();
        self.state.sub_tracks.clear();
        self.state.tracks_error = None;
        self.state.quality_error = None;
        self.state.probe_loading = true;
        self.state.quality_loading = true;

        let (probe_url, quality_urls) = if target.is_playlist() && !force_single {
            let first = selected
                .first()
                .expect("selection checked before starting configure");
            (
                first.url.clone(),
                selected.iter().map(|entry| entry.url.clone()).collect(),
            )
        } else {
            let url = target.download_url(force_single);
            (url.clone(), vec![url])
        };
        self.backend.load_media(probe_url, quality_urls, op);
    }

    pub fn continue_playlist(&mut self) {
        if self.state.selected_entries.is_empty() {
            self.state
                .toast(self.state.s().no_selection, ToastKind::Warn);
            return;
        }
        let target = self.state.target.clone();
        self.start_configure(target, false);
    }

    pub fn search(&mut self) {
        let query = self.state.search_query.trim().to_owned();
        if query.is_empty() {
            self.state.search_error = Some(self.state.s().query_empty.to_owned());
            return;
        }
        let op = self.next_search_op;
        self.next_search_op += 1;
        self.state.search_op = op;
        self.state.search_loading = true;
        self.state.search_error = None;
        self.backend.search(query, self.state.locale, op);
    }

    pub fn select_search_result(&mut self, index: usize) {
        let Some(result) = self.state.search_results.get(index).cloned() else {
            return;
        };
        self.state.search_modal = false;
        self.state.url = result.url.clone();
        self.analyze_url();
    }

    pub fn toggle_playlist(&mut self, index: usize) {
        self.state.toggle_playlist_entry(index);
        self.state.range_error = None;
    }

    pub fn select_all_playlist(&mut self) {
        self.state.select_all_playlist();
        self.state.range_error = None;
    }

    pub fn clear_playlist(&mut self) {
        self.state.clear_playlist_selection();
        self.state.range_error = None;
    }

    pub fn apply_playlist_range(&mut self) {
        let raw = self.state.range_input.trim();
        if raw.is_empty() {
            return;
        }
        match volvid_core::parse_selection(raw, self.state.playlist_entries().len()) {
            Ok(indices) => {
                self.state.selected_entries = indices.into_iter().collect();
                self.state.range_error = None;
            }
            Err(err) => {
                self.state.range_error =
                    Some(volvid_i18n::selection_error_text(err, self.state.locale));
            }
        }
    }

    pub fn set_mode(&mut self, mode: DownloadMode) {
        self.state.draft.mode = mode;
        if mode != DownloadMode::Video {
            self.state.draft.fragment_mode = FragmentMode::None;
        }
    }

    pub fn start_download(&mut self) {
        let Some(target) = self.state.target.clone() else {
            return;
        };
        let s = self.state.s();
        if !self.state.deps.ytdlp.available {
            self.state.route = Route::Settings;
            self.state.toast(s.missing_ytdlp, ToastKind::Error);
            return;
        }
        let fragment = match self
            .state
            .draft
            .fragment(Some(&target), self.state.media_duration)
        {
            Ok(fragment) => fragment,
            Err(err) => {
                self.state.toast(
                    fragment_error_text(err, self.state.media_duration, self.state.locale),
                    ToastKind::Error,
                );
                return;
            }
        };
        if target.is_playlist() && !self.state.force_single && fragment.is_some() {
            self.state
                .toast(s.trim_playlist_unavailable, ToastKind::Warn);
            return;
        }
        let profile = self.state.draft.profile(self.state.locale);
        if profile.needs_ffmpeg(fragment.as_ref()) && !self.state.deps.ffmpeg.available {
            self.state.route = Route::Settings;
            self.state.toast(s.missing_ffmpeg, ToastKind::Error);
            return;
        }

        let entries = if target.is_playlist() && !self.state.force_single {
            self.state.selected_playlist_entries()
        } else {
            Vec::new()
        };
        if target.is_playlist() && !self.state.force_single && entries.is_empty() {
            self.state.toast(s.no_selection, ToastKind::Warn);
            return;
        }
        let request = DownloadRequest {
            target: target.clone(),
            profile,
            fragment,
            media_duration: self.state.media_duration,
            force_single: self.state.force_single,
            playlist_info: if target.is_playlist() && !self.state.force_single {
                self.state.playlist.clone()
            } else {
                None
            },
            entries: entries.clone(),
            workers: self.state.draft.workers.clamp(1, 8),
            output_dir: self.state.downloads_dir.to_string_lossy().into_owned(),
            locale: self.state.locale,
        };
        if let Err(err) = volvid_engine::download::validate_request(&request, &self.state.deps) {
            self.state.toast(err.to_string(), ToastKind::Error);
            return;
        }

        let quality = self
            .state
            .draft
            .quality
            .as_ref()
            .map(|quality| quality.height)
            .unwrap_or(0);
        let detail = match request.profile.mode {
            DownloadMode::Video if quality > 0 => format!(
                "{} · {}p · {}",
                s.mode_video,
                quality,
                request.profile.video_container.to_uppercase()
            ),
            DownloadMode::Video => format!(
                "{} · {}",
                s.mode_video,
                request.profile.video_container.to_uppercase()
            ),
            DownloadMode::Audio => format!(
                "{} · {}",
                s.mode_audio,
                request.profile.audio_format.to_uppercase()
            ),
            DownloadMode::Thumbnail => s.mode_thumbnail.to_owned(),
        };
        let is_playlist = request.uses_playlist();
        let title = if is_playlist {
            self.state
                .playlist
                .as_ref()
                .map(|playlist| playlist.title.clone())
                .unwrap_or_else(|| s.playlist_title.to_owned())
        } else if !self.state.media_title.trim().is_empty() {
            self.state.media_title.clone()
        } else {
            target.canonical_url.clone()
        };
        let url = target.download_url(self.state.force_single);
        let workers = request.workers;
        self.enqueue_request(
            request,
            title,
            url,
            is_playlist,
            workers,
            detail,
            entries.len(),
        );
    }

    /// Best-effort thumbnail linked to the job's first video, used by the queue tile.
    fn job_preview_url(request: &DownloadRequest) -> Option<String> {
        let url = if request.uses_playlist() {
            request.entries.first()?.url.clone()
        } else {
            request.target.download_url(request.force_single)
        };
        parse_target(&url)
            .ok()
            .filter(|target| !target.video_id.is_empty())
            .map(|target| youtube_preview_url(&target.video_id))
    }

    #[allow(clippy::too_many_arguments)]
    fn enqueue_request(
        &mut self,
        request: DownloadRequest,
        title: String,
        url: String,
        is_playlist: bool,
        workers: usize,
        detail: String,
        entries: usize,
    ) {
        let job_id = self.next_job;
        self.next_job += 1;
        let preview_url = Self::job_preview_url(&request);
        let slots = if is_playlist {
            (0..workers).map(|_| SlotView::default()).collect()
        } else {
            vec![SlotView::default()]
        };
        self.state.jobs.push(JobView {
            id: job_id,
            title,
            url,
            output_dir: PathBuf::from(&request.output_dir),
            mode: request.profile.mode,
            detail,
            request: Some(request),
            is_playlist,
            total: if is_playlist { entries } else { 0 },
            done: 0,
            failed: 0,
            slots,
            status: JobStatus::Queued,
            error: None,
            run_id: 0,
            started: false,
            started_at: std::time::Instant::now(),
            elapsed: std::time::Duration::ZERO,
            recorded: false,
            pending_resume: false,
            thumbnail: None,
        });
        self.state.route = Route::Downloads;
        self.state
            .toast(self.state.s().queued_message, ToastKind::Info);
        if let Some(url) = preview_url {
            self.backend.fetch_job_preview(job_id, url);
        }
        self.pump_queue();
    }

    fn pump_queue(&mut self) {
        let limit = self.state.config.max_parallel_batches.clamp(1, 3);
        loop {
            let running = self.state.active_jobs();
            if running >= limit {
                break;
            }
            let Some(job_id) = self
                .state
                .jobs
                .iter()
                .filter(|job| job.status == JobStatus::Queued)
                .map(|job| job.id)
                .min()
            else {
                break;
            };
            self.start_job(job_id);
        }
    }

    fn start_job(&mut self, job_id: u64) {
        let Some(job) = self.state.jobs.iter_mut().find(|job| job.id == job_id) else {
            return;
        };
        let Some(request) = job.request.clone() else {
            return;
        };
        job.status = JobStatus::Active;
        job.started = true;
        job.run_id += 1;
        job.started_at = std::time::Instant::now();
        job.elapsed = std::time::Duration::ZERO;
        job.done = 0;
        job.failed = 0;
        job.error = None;
        job.recorded = false;
        job.pending_resume = false;
        let slots = if job.is_playlist {
            request.workers.max(1)
        } else {
            1
        };
        job.slots = (0..slots).map(|_| SlotView::default()).collect();
        let run_id = job.run_id;
        self.backend
            .start_download(job_id, run_id, request, self.state.deps.clone());
    }

    pub fn retry_job(&mut self, job_id: u64) {
        if let Some(job) = self.state.jobs.iter_mut().find(|job| job.id == job_id)
            && job.status == JobStatus::Failed
        {
            job.status = JobStatus::Queued;
            job.error = None;
            job.recorded = false;
            job.pending_resume = false;
            self.pump_queue();
        }
    }

    pub fn pause_job(&mut self, job_id: u64) {
        if let Some(job) = self.state.jobs.iter_mut().find(|job| job.id == job_id) {
            match job.status {
                JobStatus::Active => {
                    self.backend.cancel_download(job_id);
                    job.status = JobStatus::Paused;
                    job.pending_resume = false;
                }
                JobStatus::Queued => {
                    job.status = JobStatus::Paused;
                    job.pending_resume = false;
                }
                _ => {}
            }
        }
    }

    pub fn resume_job(&mut self, job_id: u64) {
        let running = self.backend.is_job_running(job_id);
        let mut queued = false;
        if let Some(job) = self.state.jobs.iter_mut().find(|job| job.id == job_id)
            && job.status == JobStatus::Paused
        {
            job.error = None;
            if running {
                job.pending_resume = true;
            } else {
                job.status = JobStatus::Queued;
                job.pending_resume = false;
                queued = true;
            }
        }
        if queued {
            self.pump_queue();
        }
    }

    pub fn pause_all_jobs(&mut self) {
        let ids: Vec<u64> = self
            .state
            .jobs
            .iter()
            .filter(|job| matches!(job.status, JobStatus::Active | JobStatus::Queued))
            .map(|job| job.id)
            .collect();
        for id in ids {
            self.pause_job(id);
        }
    }

    pub fn resume_all_jobs(&mut self) {
        let ids: Vec<u64> = self
            .state
            .jobs
            .iter()
            .filter(|job| job.status == JobStatus::Paused)
            .map(|job| job.id)
            .collect();
        for id in ids {
            self.resume_job(id);
        }
    }

    pub fn dismiss_job(&mut self, job_id: u64) {
        let removable = self
            .state
            .jobs
            .iter()
            .find(|job| job.id == job_id)
            .is_some_and(|job| !matches!(job.status, JobStatus::Active));
        if removable {
            self.state.jobs.retain(|job| job.id != job_id);
        }
    }

    pub fn clear_finished_jobs(&mut self) {
        self.state.jobs.retain(|job| job.status.is_pending());
    }

    pub fn cancel_job(&mut self, job_id: u64) {
        let active = self
            .state
            .jobs
            .iter()
            .find(|job| job.id == job_id)
            .is_some_and(|job| job.status == JobStatus::Active);
        if active {
            self.backend.cancel_download(job_id);
        }
        if let Some(job) = self.state.jobs.iter_mut().find(|job| job.id == job_id) {
            job.status = JobStatus::Cancelled;
            job.error = None;
            job.pending_resume = false;
        }
        self.pump_queue();
    }

    pub fn pick_folder(&mut self) {
        let title = self.state.s().choose_folder.to_owned();
        self.backend
            .pick_folder(Some(self.state.downloads_dir.clone()), title);
    }

    pub fn open_folder(&self, path: PathBuf) {
        self.backend.open_folder(path);
    }

    pub fn install_dependency(&mut self, key: &str) {
        if self.state.installing_dep.is_some() {
            return;
        }
        self.state.installing_dep = Some(key.to_owned());
        self.backend
            .install_dependency(key.to_owned(), self.state.locale);
    }

    pub fn cancel_install(&mut self) {
        self.backend.cancel_install();
    }

    pub fn apply_update(&mut self) {
        let Some(info) = self.state.update.clone() else {
            return;
        };
        self.state.updating = true;
        self.backend.apply_update(info, self.state.locale);
    }

    pub fn check_updates(&mut self) {
        self.state.update_checking = true;
        self.backend.check_update();
    }

    pub fn refresh_deps(&mut self) {
        self.state.deps_checking = true;
        self.backend.refresh_deps();
    }

    pub fn set_locale(&mut self, locale: volvid_core::Locale) {
        self.state.locale = locale;
        self.state.config.locale = locale;
        self.backend.save_config(self.state.config.clone());
    }

    pub fn set_theme(&mut self, theme: Theme) {
        self.state.config.theme = theme;
        self.backend.save_config(self.state.config.clone());
    }

    pub fn set_playlist_workers(&mut self, workers: usize) {
        self.state.config.playlist_workers = workers.clamp(1, 8);
        self.state.draft.workers = self.state.config.playlist_workers;
        self.backend.save_config(self.state.config.clone());
    }

    pub fn clear_history(&mut self) {
        self.state.history.clear();
        self.backend.clear_history_async();
    }

    pub fn set_route(&mut self, route: Route) {
        if route != Route::Playlist && self.state.playlist_loading {
            self.backend.cancel_metadata();
            self.state.playlist_loading = false;
            self.state.op_id = self.next_op();
        }
        self.state.route = route;
    }
}
