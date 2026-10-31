/// Download engine events (single file or playlist slot).
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum DlEventType {
    #[default]
    Start,
    Dest,
    Progress,
    Proc,
    Done,
    Reset,
    Fallback,
}

#[derive(Clone, Debug)]
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
    pub ok: bool,
}

impl Default for DlUpdate {
    fn default() -> Self {
        Self {
            event_type: DlEventType::Start,
            slot: 0,
            text: String::new(),
            err_text: String::new(),
            pct: 0.0,
            done_b: 0,
            total_b: 0,
            speed: String::new(),
            speed_bps: 0,
            ok: false,
        }
    }
}

/// Progress for managed-binary / update downloads.
#[derive(Clone, Debug, Default)]
pub struct FileProgress {
    pub pct: f64,
    pub done_b: i64,
    pub total_b: i64,
    pub speed: String,
    pub done: bool,
    pub err: Option<String>,
}
