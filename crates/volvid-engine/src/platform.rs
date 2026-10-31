use std::path::Path;
use std::process::Command;

use anyhow::{Context, Result, bail};

pub fn open_folder(path: &Path) -> Result<()> {
    std::fs::create_dir_all(path).with_context(|| format!("prepare folder {}", path.display()))?;
    let program = if cfg!(windows) {
        "explorer.exe"
    } else if cfg!(target_os = "macos") {
        "open"
    } else {
        "xdg-open"
    };
    Command::new(program)
        .arg(path)
        .spawn()
        .with_context(|| format!("open {} with {program}", path.display()))?;
    Ok(())
}

pub async fn pick_folder(
    current: Option<std::path::PathBuf>,
    title: &str,
) -> Result<Option<std::path::PathBuf>> {
    let mut dialog = rfd::AsyncFileDialog::new().set_title(title);
    if let Some(current) = current.filter(|path| path.is_dir()) {
        dialog = dialog.set_directory(current);
    }
    let Some(handle) = dialog.pick_folder().await else {
        return Ok(None);
    };
    let path = handle.path().to_path_buf();
    if !path.is_dir() {
        bail!("selected path is not a directory");
    }
    Ok(Some(path))
}
