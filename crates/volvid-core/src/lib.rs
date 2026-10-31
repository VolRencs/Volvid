//! Pure domain model for Volvid.
//!
//! This crate intentionally contains no I/O, process spawning, HTTP or GUI
//! code. It is the shared vocabulary of the engine, application state and UI.

pub mod clock;
pub mod deps;
pub mod download;
pub mod fragment;
pub mod locale;
pub mod profile;
pub mod quality;
pub mod sanitize;
pub mod selection;
pub mod session;
pub mod target;
pub mod types;

pub use clock::{format_clock, format_duration, parse_clock, parse_flexible_timestamp};
pub use deps::{
    BrowserCookiesInfo, CheckDepsResult, DependencyInfo, DependencySource, FFmpegArgs,
    JSRuntimeInfo, STATUS_ACTIVE, STATUS_NO_PROFILE, STATUS_NOT_FOUND,
};
pub use download::{DlEventType, DlUpdate, FileProgress};
pub use fragment::{
    DownloadFragment, FormatFragmentLabel, FragmentError, ParseBoundedFragmentRange,
    ValidateFragmentDuration, format_fragment_label, parse_bounded_fragment_range,
    parse_fragment_range, validate_fragment_duration,
};
pub use locale::{Locale, NextLocale, ParseLocale, parse_locale};
pub use profile::{
    AudioBitrate, DownloadMode, DownloadRequest, OutputProfile, QualityChoice, SubtitleMode,
    YTDLP_BEST_FORMAT,
};
pub use quality::{
    DefaultQualityChoices, QualityChainAt, default_quality_choices, quality_chain_at,
};
pub use sanitize::{SanitizeDirname, SanitizeFileStem, sanitize_dirname, sanitize_file_stem};
pub use selection::{SelectionError, parse_selection};
pub use session::{Session, SessionItem};
pub use target::{ParseTarget, ParsedTarget, TargetError, TargetKind, parse_target};
pub use types::{
    AudioTrack, MediaFormat, MediaProbe, PlaylistEntry, PlaylistInfo, SearchResult, SubtitleTrack,
    UpdateInfo,
};
