use std::collections::BTreeSet;
use std::path::PathBuf;
use std::sync::Arc;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};
use volvid_core::{
    AudioTrack, CheckDepsResult, DlEventType, DlUpdate, DownloadFragment, DownloadMode,
    DownloadRequest, FileProgress, FragmentError, Locale, MediaProbe, OutputProfile, ParsedTarget,
    PlaylistEntry, PlaylistInfo, QualityChoice, SearchResult, SubtitleTrack, UpdateInfo,
    format_fragment_label, parse_bounded_fragment_range,
};
use volvid_engine::{AppPaths, Config, PreviewImage, Theme};
use volvid_i18n::{
    audio_output_profiles, default_quality, quality_profile, strings, thumbnail_profile,
    video_output_profiles,
};

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum Route {
    #[default]
    Home,
    Playlist,
    Configure,
    Downloads,
    Summary,
    Settings,
}

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum FragmentMode {
    #[default]
    None,
    FromUrl,
    Custom,
}

#[derive(Clone, Debug)]
pub struct Draft {
    pub mode: DownloadMode,
    pub quality: Option<QualityChoice>,
    pub video_profile_index: usize,
    pub audio_profile_index: usize,
    pub audio_langs: BTreeSet<String>,
    pub sub_langs: BTreeSet<String>,
    pub fragment_mode: FragmentMode,
    pub custom_start: String,
    pub custom_end: String,
    pub workers: usize,
}

impl Default for Draft {
    fn default() -> Self {
        Self {
            mode: DownloadMode::Video,
            quality: None,
            video_profile_index: 0,
            audio_profile_index: 0,
            audio_langs: BTreeSet::new(),
            sub_langs: BTreeSet::new(),
            fragment_mode: FragmentMode::None,
            custom_start: String::new(),
            custom_end: String::new(),
            workers: 4,
        }
    }
}

impl Draft {
    pub fn selected_quality(&self, locale: Locale) -> QualityChoice {
        self.quality
            .clone()
            .or_else(|| default_quality(locale).into_iter().next())
            .unwrap_or_default()
    }

    pub fn video_profiles(&self, locale: Locale) -> Vec<OutputProfile> {
        let base = quality_profile(&self.selected_quality(locale), locale);
        video_output_profiles(base, locale)
    }

    pub fn audio_profiles(&self, locale: Locale) -> Vec<OutputProfile> {
        audio_output_profiles(locale)
    }

    pub fn profile(&self, locale: Locale) -> OutputProfile {
        let mut profile = match self.mode {
            DownloadMode::Video => {
                let profiles = self.video_profiles(locale);
                profiles
                    .get(self.video_profile_index)
                    .cloned()
                    .unwrap_or_else(|| profiles.first().cloned().unwrap_or_default())
            }
            DownloadMode::Audio => {
                let profiles = self.audio_profiles(locale);
                profiles
                    .get(self.audio_profile_index)
                    .cloned()
                    .unwrap_or_else(|| profiles.first().cloned().unwrap_or_default())
            }
            DownloadMode::Thumbnail => thumbnail_profile(locale),
        };
        if self.mode == DownloadMode::Video {
            profile.audio_langs = self.audio_langs.iter().cloned().collect();
            profile.sub_langs = self.sub_langs.iter().cloned().collect();
            profile.sub_mode = if profile.sub_langs.is_empty() {
                volvid_core::SubtitleMode::Off
            } else {
                volvid_core::SubtitleMode::Embed
            };
        } else if self.mode == DownloadMode::Audio {
            profile.audio_langs = self.audio_langs.iter().cloned().collect();
        }
        profile
    }

    pub fn fragment(
        &self,
        target: Option<&ParsedTarget>,
        duration: i64,
    ) -> Result<Option<DownloadFragment>, FragmentError> {
        match self.fragment_mode {
            FragmentMode::None => Ok(None),
            FragmentMode::FromUrl => {
                let target = target.ok_or(FragmentError::Format)?;
                if target.url_start_at <= 0 {
                    return Ok(None);
                }
                let fragment = DownloadFragment {
                    start_at: target.url_start_at,
                    end_at: None,
                };
                volvid_core::validate_fragment_duration(&fragment, duration)?;
                Ok(Some(fragment))
            }
            FragmentMode::Custom => {
                let raw = if self.custom_end.trim().is_empty() {
                    format!("{}+", self.custom_start.trim())
                } else {
                    format!("{}-{}", self.custom_start.trim(), self.custom_end.trim())
                };
                let fragment = parse_bounded_fragment_range(&raw, duration)?;
                Ok(Some(fragment))
            }
        }
    }

    pub fn fragment_label(&self, target: Option<&ParsedTarget>, duration: i64) -> String {
        match self.fragment(target, duration) {
            Ok(fragment) => format_fragment_label(fragment.as_ref()),
            Err(_) => String::new(),
        }
    }

    pub fn can_use_url_start(&self, target: Option<&ParsedTarget>, duration: i64) -> bool {
        target.is_some_and(|target| {
            target.has_url_start
                && target.url_start_at > 0
                && duration > 0
                && target.url_start_at < duration
        })
    }
}

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum JobStatus {
    Queued,
    #[default]
    Active,
    Paused,
    Completed,
    Failed,
    Cancelled,
}

impl JobStatus {
    pub fn is_pending(self) -> bool {
        matches!(self, Self::Queued | Self::Active | Self::Paused)
    }
}

#[derive(Clone, Debug, Default)]
pub struct SlotView {
    pub title: String,
    pub pct: f64,
    pub done_b: i64,
    pub total_b: i64,
    pub speed: String,
    pub speed_bps: i64,
    pub label: String,
    pub proc: bool,
}

#[derive(Clone, Debug)]
pub struct JobView {
    pub id: u64,
    pub title: String,
    pub url: String,
    pub output_dir: PathBuf,
    pub mode: DownloadMode,
    pub detail: String,
    pub request: Option<DownloadRequest>,
    pub is_playlist: bool,
    pub total: usize,
    pub done: usize,
    pub failed: usize,
    pub slots: Vec<SlotView>,
    pub status: JobStatus,
    pub error: Option<String>,
    pub run_id: u64,
    pub started: bool,
    pub started_at: Instant,
    pub elapsed: Duration,
    pub recorded: bool,
    pub pending_resume: bool,
    /// Instant preview fetched for the queue tile, applied late by the backend.
    pub thumbnail: Option<Arc<PreviewImage>>,
}

impl JobView {
    pub fn expected(&self) -> usize {
        if self.total > 0 { self.total } else { 1 }
    }

    pub fn finished(&self) -> usize {
        self.done + self.failed
    }

    pub fn progress(&self) -> f64 {
        if self.total > 0 {
            self.finished() as f64 / self.total as f64 * 100.0
        } else {
            self.slots.first().map(|slot| slot.pct).unwrap_or(0.0)
        }
    }

    pub fn apply(&mut self, update: &DlUpdate) {
        if update.slot >= self.slots.len() {
            self.slots.resize(update.slot + 1, SlotView::default());
        }
        if let Some(slot) = self.slots.get_mut(update.slot) {
            match update.event_type {
                DlEventType::Start => {
                    slot.title = update.text.clone();
                    slot.label.clear();
                    slot.proc = false;
                }
                DlEventType::Dest => slot.title = update.text.clone(),
                DlEventType::Progress => {
                    slot.pct = update.pct;
                    slot.done_b = update.done_b;
                    slot.total_b = update.total_b;
                    slot.speed = update.speed.clone();
                    slot.speed_bps = update.speed_bps;
                    slot.proc = false;
                }
                DlEventType::Proc | DlEventType::Fallback => {
                    slot.proc = true;
                    slot.label = update.text.clone();
                }
                DlEventType::Reset => *slot = SlotView::default(),
                DlEventType::Done => {
                    slot.proc = false;
                    slot.pct = 100.0;
                    if !update.ok {
                        slot.label = update.err_text.clone();
                    }
                }
            }
        }
        if update.event_type == DlEventType::Done {
            if update.ok {
                self.done += 1;
            } else {
                self.failed += 1;
                if self.error.is_none() && !update.err_text.trim().is_empty() {
                    self.error = Some(update.err_text.clone());
                }
            }
            if self.finished() >= self.expected() {
                self.status = if self.failed == 0 {
                    JobStatus::Completed
                } else {
                    JobStatus::Failed
                };
                self.elapsed = self.started_at.elapsed();
            }
        }
    }
}

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(default)]
pub struct HistoryItem {
    pub label: String,
    pub url: String,
    pub ok: bool,
    pub mode: DownloadMode,
    pub output_dir: PathBuf,
    pub timestamp: i64,
    pub items: usize,
    pub failed: usize,
}

impl HistoryItem {
    pub fn from_job(job: &JobView) -> Self {
        Self {
            label: job.title.clone(),
            url: job.url.clone(),
            ok: job.status == JobStatus::Completed,
            mode: job.mode,
            output_dir: job.output_dir.clone(),
            timestamp: SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map(|duration| duration.as_secs() as i64)
                .unwrap_or(0),
            items: job.total.max(1),
            failed: job.failed,
        }
    }
}

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum ToastKind {
    #[default]
    Info,
    Success,
    Warn,
    Error,
}

#[derive(Clone, Debug)]
pub struct Toast {
    pub text: String,
    pub kind: ToastKind,
    pub created: Instant,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum ThumbnailStage {
    Preview,
    Full,
}

#[derive(Clone, Debug)]
pub enum AppEvent {
    Deps(CheckDepsResult),
    DepsEnriched(CheckDepsResult),
    UpdateChecked(Option<UpdateInfo>),
    Playlist {
        op: u64,
        result: Result<PlaylistInfo, String>,
    },
    Probe {
        op: u64,
        result: Result<MediaProbe, String>,
    },
    Thumbnail {
        op: u64,
        stage: ThumbnailStage,
        result: Result<Arc<PreviewImage>, String>,
    },
    Quality {
        op: u64,
        result: Result<Vec<QualityChoice>, String>,
    },
    Search {
        op: u64,
        result: Result<Vec<SearchResult>, String>,
    },
    Download {
        job_id: u64,
        run_id: u64,
        update: DlUpdate,
    },
    JobThumbnail {
        job_id: u64,
        result: Result<Arc<PreviewImage>, String>,
    },
    DownloadFinished {
        job_id: u64,
        run_id: u64,
        result: Result<(), String>,
    },
    InstallProgress {
        key: String,
        progress: FileProgress,
    },
    InstallFinished {
        key: String,
        result: Result<(), String>,
    },
    UpdateProgress(FileProgress),
    UpdateFinished(Result<(), String>),
    FolderPicked(Result<Option<PathBuf>, String>),
    OpenFolder(Result<(), String>),
    ConfigSaved(Result<(), String>),
    HistorySaved(Result<(), String>),
    Toast(String, ToastKind),
}

#[derive(Clone, Debug)]
pub struct MixedChoice {
    pub target: ParsedTarget,
    pub url: String,
}

pub struct AppState {
    pub route: Route,
    pub locale: Locale,
    pub config: Config,
    pub downloads_dir: PathBuf,
    pub logs_dir: PathBuf,

    pub deps: CheckDepsResult,
    pub deps_checking: bool,
    pub deps_progress: Option<FileProgress>,
    pub installing_dep: Option<String>,

    pub update: Option<UpdateInfo>,
    pub update_checking: bool,
    pub update_modal: bool,
    pub update_progress: Option<FileProgress>,
    pub updating: bool,

    pub search_modal: bool,

    pub url: String,
    pub url_error: Option<String>,
    pub target: Option<ParsedTarget>,
    pub mixed_choice: Option<MixedChoice>,
    pub force_single: bool,

    pub playlist: Option<PlaylistInfo>,
    pub playlist_loading: bool,
    pub playlist_scroll_reset: bool,
    pub playlist_filter: String,
    pub playlist_filtered: Arc<Vec<usize>>,
    pub playlist_filter_key: String,
    pub selected_entries: BTreeSet<usize>,
    pub range_input: String,
    pub range_error: Option<String>,

    pub draft: Draft,
    pub media_duration: i64,
    pub media_title: String,
    pub media_thumbnail: String,
    pub preview_image: Option<Arc<PreviewImage>>,
    pub preview_generation: u64,
    pub thumbnail_loading: bool,
    pub thumbnail_error: bool,
    pub audio_tracks: Vec<AudioTrack>,
    pub sub_tracks: Vec<SubtitleTrack>,
    pub quality_choices: Vec<QualityChoice>,
    pub quality_loading: bool,
    pub probe_loading: bool,
    pub quality_error: Option<String>,
    pub tracks_error: Option<String>,

    pub search_query: String,
    pub search_results: Vec<SearchResult>,
    pub search_loading: bool,
    pub search_error: Option<String>,
    pub search_op: u64,

    pub jobs: Vec<JobView>,
    pub history: Vec<HistoryItem>,
    pub toast: Option<Toast>,
    pub op_id: u64,
}

impl AppState {
    pub fn new(config: Config, paths: &AppPaths, history: Vec<HistoryItem>) -> Self {
        let downloads_dir = config.downloads_dir(paths);
        let locale = config.locale;
        let workers = config.playlist_workers;
        Self {
            route: Route::Home,
            locale,
            downloads_dir,
            logs_dir: paths.logs_dir(),
            config,
            deps: CheckDepsResult::default(),
            deps_checking: true,
            deps_progress: None,
            installing_dep: None,
            update: None,
            update_checking: true,
            update_modal: false,
            update_progress: None,
            updating: false,
            search_modal: false,
            url: String::new(),
            url_error: None,
            target: None,
            mixed_choice: None,
            force_single: false,
            playlist: None,
            playlist_loading: false,
            playlist_scroll_reset: true,
            playlist_filter: String::new(),
            playlist_filtered: Arc::new(Vec::new()),
            playlist_filter_key: String::new(),
            selected_entries: BTreeSet::new(),
            range_input: String::new(),
            range_error: None,
            draft: Draft {
                workers,
                ..Draft::default()
            },
            media_duration: 0,
            media_title: String::new(),
            media_thumbnail: String::new(),
            preview_image: None,
            preview_generation: 0,
            thumbnail_loading: false,
            thumbnail_error: false,
            audio_tracks: Vec::new(),
            sub_tracks: Vec::new(),
            quality_choices: Vec::new(),
            quality_loading: false,
            probe_loading: false,
            quality_error: None,
            tracks_error: None,
            search_query: String::new(),
            search_results: Vec::new(),
            search_loading: false,
            search_error: None,
            search_op: 0,
            jobs: Vec::new(),
            history,
            toast: None,
            op_id: 0,
        }
    }

    pub fn s(&self) -> &'static volvid_i18n::Strings {
        strings(self.locale)
    }

    pub fn toast(&mut self, text: impl Into<String>, kind: ToastKind) {
        self.toast = Some(Toast {
            text: text.into(),
            kind,
            created: Instant::now(),
        });
    }

    pub fn apply(&mut self, event: AppEvent) -> bool {
        match event {
            AppEvent::Deps(deps) => {
                self.deps_checking = false;
                self.deps = deps;
                true
            }
            AppEvent::DepsEnriched(deps) => {
                self.deps_checking = false;
                self.deps = deps;
                true
            }
            AppEvent::UpdateChecked(info) => {
                self.update_checking = false;
                self.update = info;
                true
            }
            AppEvent::Playlist { op, result } => {
                if op != self.op_id {
                    return false;
                }
                self.playlist_loading = false;
                match result {
                    Ok(info) => {
                        self.playlist = Some(info);
                        self.selected_entries.clear();
                        self.playlist_filter.clear();
                        self.playlist_filter_key = "\0".to_owned();
                        self.playlist_scroll_reset = true;
                        self.refresh_playlist_filter();
                        self.route = Route::Playlist;
                    }
                    Err(err) => {
                        self.toast(err, ToastKind::Error);
                        self.route = Route::Home;
                    }
                }
                true
            }
            AppEvent::Probe { op, result } => {
                if op != self.op_id {
                    return false;
                }
                self.probe_loading = false;
                match result {
                    Ok(probe) => {
                        self.media_duration = probe.duration;
                        self.media_title = probe.title;
                        self.media_thumbnail = probe.thumbnail;
                        self.audio_tracks = probe.audio_tracks;
                        self.sub_tracks = probe.subtitles;
                        self.tracks_error = None;
                    }
                    Err(err) => {
                        self.tracks_error = Some(err.clone());
                        self.toast(err, ToastKind::Warn);
                    }
                }
                true
            }
            AppEvent::Thumbnail { op, stage, result } => {
                if op != self.op_id {
                    return false;
                }
                self.thumbnail_loading = false;
                match result {
                    Ok(image) => {
                        tracing::debug!(
                            stage = ?stage,
                            width = image.width,
                            height = image.height,
                            "thumbnail loaded"
                        );
                        self.preview_image = Some(image);
                        self.preview_generation = self.preview_generation.wrapping_add(1);
                        self.thumbnail_error = false;
                    }
                    Err(err) => {
                        tracing::debug!(?stage, %err, "thumbnail unavailable");
                        if self.preview_image.is_none() {
                            self.thumbnail_error = true;
                        }
                    }
                }
                true
            }
            AppEvent::Quality { op, result } => {
                if op != self.op_id {
                    return false;
                }
                self.quality_loading = false;
                match result {
                    Ok(choices) if !choices.is_empty() => {
                        self.quality_error = None;
                        self.quality_choices = choices;
                        self.draft.quality = self.quality_choices.first().cloned();
                    }
                    Ok(_) => {
                        self.quality_error = Some(self.s().quality_failed.to_owned());
                        self.quality_choices = default_quality(self.locale);
                        self.draft.quality = self.quality_choices.first().cloned();
                    }
                    Err(err) => {
                        self.quality_error = Some(format!("{}: {err}", self.s().quality_failed));
                        self.quality_choices = default_quality(self.locale);
                        self.draft.quality = self.quality_choices.first().cloned();
                    }
                }
                true
            }
            AppEvent::Search { op, result } => {
                if op != self.search_op {
                    return false;
                }
                self.search_loading = false;
                match result {
                    Ok(results) => {
                        self.search_results = results;
                        self.search_error = None;
                    }
                    Err(err) => {
                        self.search_error = Some(err);
                        self.search_results.clear();
                    }
                }
                true
            }
            AppEvent::Download {
                job_id,
                run_id,
                update,
            } => {
                if let Some(job) = self.jobs.iter_mut().find(|job| job.id == job_id)
                    && job.run_id == run_id
                    && job.status == JobStatus::Active
                {
                    job.apply(&update);
                }
                true
            }
            AppEvent::JobThumbnail { job_id, result } => {
                if let Some(job) = self.jobs.iter_mut().find(|job| job.id == job_id)
                    && let Ok(image) = result
                {
                    job.thumbnail = Some(image);
                }
                true
            }
            AppEvent::DownloadFinished {
                job_id,
                run_id,
                result,
            } => {
                let failure_text = self.s().download_failed.to_owned();
                let history_item =
                    if let Some(job) = self.jobs.iter_mut().find(|job| job.id == job_id) {
                        if job.run_id != run_id {
                            None
                        } else if job.status == JobStatus::Paused && job.pending_resume {
                            job.pending_resume = false;
                            job.status = JobStatus::Queued;
                            job.elapsed = job.started_at.elapsed();
                            None
                        } else if !matches!(
                            job.status,
                            JobStatus::Active | JobStatus::Completed | JobStatus::Failed
                        ) {
                            None
                        } else {
                            if job.status == JobStatus::Active {
                                job.status = if result.is_ok() && job.failed == 0 {
                                    JobStatus::Completed
                                } else {
                                    JobStatus::Failed
                                };
                            }
                            job.elapsed = job.started_at.elapsed();
                            if let Err(err) = result
                                && job.error.is_none()
                            {
                                job.error = Some(err);
                            }
                            if job.status == JobStatus::Failed && job.error.is_none() {
                                job.error = Some(failure_text);
                            }
                            if matches!(job.status, JobStatus::Failed | JobStatus::Completed)
                                && !job.recorded
                            {
                                job.recorded = true;
                                Some(HistoryItem::from_job(job))
                            } else {
                                None
                            }
                        }
                    } else {
                        None
                    };
                if let Some(item) = history_item {
                    self.history.push(item);
                    if self.history.len() > 100 {
                        self.history.drain(..self.history.len() - 100);
                    }
                }
                true
            }
            AppEvent::InstallProgress { key, progress } => {
                self.installing_dep = Some(key);
                self.deps_progress = Some(progress);
                true
            }
            AppEvent::InstallFinished { key, result } => {
                self.installing_dep = None;
                self.deps_progress = None;
                match result {
                    Ok(()) => {
                        self.toast(format!("{} {}", self.s().success, key), ToastKind::Success);
                    }
                    Err(err) => self.toast(
                        format!("{}: {err}", self.s().install_failed),
                        ToastKind::Error,
                    ),
                }
                true
            }
            AppEvent::UpdateProgress(progress) => {
                self.updating = !progress.done;
                self.update_progress = Some(progress);
                true
            }
            AppEvent::UpdateFinished(result) => {
                self.updating = false;
                self.update_progress = None;
                match result {
                    Ok(()) => {
                        self.toast(self.s().update_applied, ToastKind::Success);
                        self.update_modal = false;
                    }
                    Err(err) => self.toast(
                        format!("{}: {err}", self.s().update_failed),
                        ToastKind::Error,
                    ),
                }
                true
            }
            AppEvent::FolderPicked(result) => {
                match result {
                    Ok(Some(path)) => {
                        self.downloads_dir = path.clone();
                        self.config.set_downloads_dir(path);
                    }
                    Ok(None) => {}
                    Err(err) => self.toast(
                        format!("{}: {err}", self.s().pick_folder_failed),
                        ToastKind::Error,
                    ),
                }
                true
            }
            AppEvent::OpenFolder(result) => {
                if let Err(err) = result {
                    self.toast(
                        format!("{}: {err}", self.s().open_folder_failed),
                        ToastKind::Error,
                    );
                }
                true
            }
            AppEvent::ConfigSaved(result) => {
                if let Err(err) = result {
                    self.toast(
                        format!("{}: {err}", self.s().config_save_failed),
                        ToastKind::Error,
                    );
                }
                true
            }
            AppEvent::HistorySaved(result) => {
                if let Err(err) = result {
                    self.toast(
                        format!("{}: {err}", self.s().history_save_failed),
                        ToastKind::Error,
                    );
                }
                true
            }
            AppEvent::Toast(text, kind) => {
                self.toast(text, kind);
                true
            }
        }
    }

    pub fn playlist_entries(&self) -> &[PlaylistEntry] {
        self.playlist
            .as_ref()
            .map(|playlist| playlist.entries.as_slice())
            .unwrap_or_default()
    }

    pub fn refresh_playlist_filter(&mut self) {
        if self.playlist_filter_key == self.playlist_filter {
            return;
        }
        let filter = self.playlist_filter.trim().to_lowercase();
        let filtered = self
            .playlist_entries()
            .iter()
            .enumerate()
            .filter(|(_, entry)| filter.is_empty() || entry.title.to_lowercase().contains(&filter))
            .map(|(index, _)| index)
            .collect();
        self.playlist_filtered = Arc::new(filtered);
        self.playlist_filter_key = self.playlist_filter.clone();
    }

    pub fn selected_playlist_entries(&self) -> Vec<PlaylistEntry> {
        self.playlist_entries()
            .iter()
            .filter(|entry| self.selected_entries.contains(&entry.index))
            .cloned()
            .collect()
    }

    pub fn toggle_playlist_entry(&mut self, index: usize) {
        if !self.selected_entries.remove(&index) {
            self.selected_entries.insert(index);
        }
    }

    pub fn select_all_playlist(&mut self) {
        self.selected_entries = self
            .playlist_entries()
            .iter()
            .map(|entry| entry.index)
            .collect();
    }

    pub fn clear_playlist_selection(&mut self) {
        self.selected_entries.clear();
    }

    pub fn active_jobs(&self) -> usize {
        self.jobs
            .iter()
            .filter(|job| job.status == JobStatus::Active)
            .count()
    }

    pub fn queued_jobs(&self) -> usize {
        self.jobs
            .iter()
            .filter(|job| job.status == JobStatus::Queued)
            .count()
    }

    pub fn paused_jobs(&self) -> usize {
        self.jobs
            .iter()
            .filter(|job| job.status == JobStatus::Paused)
            .count()
    }

    pub fn pending_jobs(&self) -> usize {
        self.jobs
            .iter()
            .filter(|job| job.status.is_pending())
            .count()
    }

    pub fn localize_playlist_title(&self) -> String {
        let title = self
            .playlist
            .as_ref()
            .map(|playlist| playlist.title.trim())
            .unwrap_or_default();
        if title.is_empty() || title.eq_ignore_ascii_case("playlist") {
            self.s().playlist_title.to_owned()
        } else {
            title.to_owned()
        }
    }

    pub fn title_for_configure(&self) -> String {
        if self
            .target
            .as_ref()
            .is_some_and(|target| target.is_playlist() && !self.force_single)
            && let Some(playlist) = &self.playlist
            && !playlist.title.trim().is_empty()
        {
            return playlist.title.clone();
        }
        if !self.media_title.trim().is_empty() {
            return self.media_title.clone();
        }
        if let Some(target) = &self.target {
            return target.canonical_url.clone();
        }
        String::new()
    }

    pub fn theme(&self) -> Theme {
        self.config.theme
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use volvid_engine::{AppPaths, Config};

    fn state() -> AppState {
        AppState::new(Config::default(), &AppPaths::detect(), Vec::new())
    }

    #[test]
    fn starts_on_home() {
        assert_eq!(state().route, Route::Home);
    }

    #[test]
    fn single_download_completes() {
        let mut job = JobView {
            id: 1,
            title: "Test".to_owned(),
            url: "https://youtu.be/test".to_owned(),
            output_dir: PathBuf::from("/tmp"),
            mode: DownloadMode::Video,
            detail: "Video · MP4".to_owned(),
            request: None,
            is_playlist: false,
            total: 0,
            done: 0,
            failed: 0,
            slots: vec![SlotView::default()],
            status: JobStatus::Active,
            error: None,
            run_id: 1,
            started: true,
            started_at: Instant::now(),
            elapsed: Duration::ZERO,
            recorded: false,
            pending_resume: false,
            thumbnail: None,
        };
        job.apply(&DlUpdate {
            event_type: DlEventType::Done,
            slot: 0,
            ok: true,
            pct: 100.0,
            ..DlUpdate::default()
        });
        assert_eq!(job.status, JobStatus::Completed);
        assert_eq!(job.progress(), 100.0);
    }

    fn test_job(status: JobStatus) -> JobView {
        JobView {
            id: 7,
            title: "Test".to_owned(),
            url: "https://youtu.be/test".to_owned(),
            output_dir: PathBuf::from("/tmp"),
            mode: DownloadMode::Video,
            detail: "Video · MP4".to_owned(),
            request: None,
            is_playlist: false,
            total: 0,
            done: 0,
            failed: 0,
            slots: vec![SlotView::default()],
            status,
            error: None,
            run_id: 1,
            started: true,
            started_at: Instant::now(),
            elapsed: Duration::ZERO,
            recorded: false,
            pending_resume: false,
            thumbnail: None,
        }
    }

    #[test]
    fn failed_download_is_recorded_as_failure() {
        let mut state = state();
        state.jobs.push(test_job(JobStatus::Active));
        state.apply(AppEvent::DownloadFinished {
            job_id: 7,
            run_id: 1,
            result: Err("boom".to_owned()),
        });
        assert_eq!(state.jobs[0].status, JobStatus::Failed);
        assert_eq!(state.history.len(), 1);
        assert!(!state.history[0].ok);
    }

    #[test]
    fn cancelled_download_is_not_history() {
        let mut state = state();
        state.jobs.push(test_job(JobStatus::Cancelled));
        state.apply(AppEvent::DownloadFinished {
            job_id: 7,
            run_id: 1,
            result: Err("operation cancelled".to_owned()),
        });
        assert!(state.history.is_empty());
    }

    #[test]
    fn stale_run_updates_are_ignored() {
        let mut state = state();
        state.jobs.push(test_job(JobStatus::Active));
        state.apply(AppEvent::Download {
            job_id: 7,
            run_id: 0,
            update: DlUpdate {
                event_type: DlEventType::Progress,
                pct: 50.0,
                ..DlUpdate::default()
            },
        });
        assert_eq!(state.jobs[0].slots[0].pct, 0.0);
    }

    #[test]
    fn playlist_selection_toggles() {
        let mut state = state();
        state.playlist = Some(PlaylistInfo {
            title: "list".to_owned(),
            entries: vec![
                PlaylistEntry {
                    index: 1,
                    title: "one".to_owned(),
                    url: "https://youtu.be/one".to_owned(),
                    duration: 10,
                },
                PlaylistEntry {
                    index: 2,
                    title: "two".to_owned(),
                    url: "https://youtu.be/two".to_owned(),
                    duration: 20,
                },
            ],
        });
        state.toggle_playlist_entry(1);
        assert_eq!(state.selected_entries.len(), 1);
        state.select_all_playlist();
        assert_eq!(state.selected_entries.len(), 2);
        state.clear_playlist_selection();
        assert!(state.selected_entries.is_empty());
    }
}
