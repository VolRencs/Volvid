use crate::state::HistoryItem;
use volvid_engine::{AppPaths, paths::write_atomic};

pub fn load_history(paths: &AppPaths) -> Vec<HistoryItem> {
    let path = paths.history_file();
    let Ok(bytes) = std::fs::read(path) else {
        return Vec::new();
    };
    let mut items: Vec<HistoryItem> = serde_json::from_slice(&bytes).unwrap_or_default();
    if items.len() > 100 {
        items.drain(..items.len() - 100);
    }
    items
}

pub fn save_history(paths: &AppPaths, history: &[HistoryItem]) -> std::io::Result<()> {
    let path = paths.history_file();
    let bytes =
        serde_json::to_vec_pretty(history).map_err(|err| std::io::Error::other(err.to_string()))?;
    write_atomic(&path, &bytes)
}

pub fn clear_history(paths: &AppPaths) -> std::io::Result<()> {
    let path = paths.history_file();
    if path.exists() {
        std::fs::remove_file(path)?;
    }
    Ok(())
}
