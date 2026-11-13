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
pub mod sanitize;
pub mod selection;
pub mod target;
pub mod types;

pub use clock::{ClockError, format_clock, format_duration, parse_clock, parse_flexible_timestamp};
pub use deps::{
    BrowserCookiesInfo, CheckDepsResult, DependencyInfo, DependencySource, JSRuntimeInfo,
    STATUS_ACTIVE, STATUS_NO_PROFILE, STATUS_NOT_FOUND,
};
pub use download::{DlEventType, DlUpdate, FileProgress};
pub use fragment::{
    DownloadFragment, FragmentError, format_fragment_label, parse_bounded_fragment_range,
    parse_fragment_range, validate_fragment_duration,
};
pub use locale::{Locale, parse_locale};
pub use profile::{
    BEST_VIDEO_FORMAT, DownloadMode, DownloadRequest, OutputProfile, QualityChoice, SubtitleMode,
    default_quality_choices,
};
pub use sanitize::{sanitize_dirname, truncate_utf8};
pub use selection::{SelectionError, parse_selection};
pub use target::{ParsedTarget, TargetError, TargetKind, parse_target};
pub use types::{
    AudioTrack, MediaFormat, MediaProbe, PlaylistEntry, PlaylistInfo, SearchResult, SubtitleTrack,
    UpdateInfo,
};
