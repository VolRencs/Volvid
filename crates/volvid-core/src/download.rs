/// Download engine events (single file or playlist slot).
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum DlEventType {
    #[default]
    Start,
    Progress,
    Proc,
    Done,
    Reset,
    Fallback,
}

#[derive(Clone, Debug, Default)]
pub struct DlUpdate {
    pub event_type: DlEventType,
    pub slot: usize,
    pub text: String,
    pub err_text: String,
    pub pct: f64,
    pub done_b: i64,
    pub total_b: i64,
    pub speed: String,
    pub speed_bps: i64,
    pub eta_secs: i64,
    pub ok: bool,
}

/// Progress for managed-binary / update downloads.
#[derive(Clone, Debug, Default)]
pub struct FileProgress {
    pub pct: f64,
    pub done_b: i64,
    pub total_b: i64,
    pub speed: String,
    pub done: bool,
}

impl FileProgress {
    pub fn failed() -> Self {
        Self {
            done: true,
            ..Self::default()
        }
    }
}
