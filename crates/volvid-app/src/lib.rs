//! UI-independent application state and controller for Volvid.

pub mod backend;
pub mod controller;
pub mod history;
pub mod state;

pub use controller::Controller;
pub use state::{
    AppEvent, AppState, Draft, FragmentMode, HistoryItem, JobStatus, JobView, Route, SlotView,
    ToastKind,
};
