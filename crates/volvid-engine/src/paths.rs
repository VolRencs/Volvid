use std::env;
use std::path::{Path, PathBuf};

use directories::{BaseDirs, UserDirs};

pub const APP_DIR_NAME: &str = "Volvid";
pub const ENV_CONFIG_DIR: &str = "VOLVID_CONFIG_DIR";
pub const ENV_DATA_DIR: &str = "VOLVID_DATA_DIR";
pub const ENV_DEPS_DIR: &str = "VOLVID_DEPS_DIR";
pub const ENV_DOWNLOADS_DIR: &str = "VOLVID_DOWNLOADS_DIR";

#[derive(Clone, Debug)]
pub struct AppPaths {
    pub config_dir: PathBuf,
    pub data_dir: PathBuf,
    pub deps_dir: PathBuf,
    pub downloads_dir: PathBuf,
}

impl AppPaths {
    pub fn detect() -> Self {
        let base = BaseDirs::new();
        let app_dir = current_exe_dir();
        let config_dir = env_path(ENV_CONFIG_DIR).unwrap_or_else(|| {
            base.as_ref()
                .map(|dirs| dirs.config_dir().join(APP_DIR_NAME))
                .unwrap_or_else(|| app_dir.join(".volvid/config"))
        });
        let data_dir = env_path(ENV_DATA_DIR).unwrap_or_else(|| {
            if cfg!(windows) {
                env::var_os("LOCALAPPDATA")
                    .map(PathBuf::from)
                    .map(|path| path.join(APP_DIR_NAME))
                    .or_else(|| base.as_ref().map(|dirs| dirs.data_dir().join(APP_DIR_NAME)))
                    .unwrap_or_else(|| app_dir.join(".volvid/data"))
            } else {
                env::var_os("XDG_DATA_HOME")
                    .map(PathBuf::from)
                    .map(|path| path.join(APP_DIR_NAME))
                    .or_else(|| base.as_ref().map(|dirs| dirs.data_dir().join(APP_DIR_NAME)))
                    .unwrap_or_else(|| app_dir.join(".volvid/data"))
            }
        });
        let deps_dir = env_path(ENV_DEPS_DIR).unwrap_or_else(|| data_dir.join("deps"));
        let downloads_dir = env_path(ENV_DOWNLOADS_DIR)
            .or_else(|| UserDirs::new().and_then(|dirs| dirs.download_dir().map(Path::to_path_buf)))
            .or_else(|| base.as_ref().map(|dirs| dirs.home_dir().join("Downloads")))
            .unwrap_or_else(|| data_dir.join("downloads"));

        Self {
            config_dir,
            data_dir,
            deps_dir,
            downloads_dir,
        }
    }

    pub fn config_file(&self) -> PathBuf {
        self.config_dir.join("config.toml")
    }

    pub fn history_file(&self) -> PathBuf {
        self.data_dir.join("history.json")
    }

    pub fn logs_dir(&self) -> PathBuf {
        self.data_dir.join("logs")
    }

    pub fn ytrust_bin(&self) -> PathBuf {
        self.deps_dir.join(binary_name("ytrust"))
    }

    pub fn ffmpeg_bin(&self) -> PathBuf {
        self.deps_dir.join(binary_name("ffmpeg"))
    }

    pub fn ffprobe_bin(&self) -> PathBuf {
        self.deps_dir.join(binary_name("ffprobe"))
    }

    pub fn node_bin(&self) -> PathBuf {
        self.deps_dir.join(binary_name("node"))
    }
}

fn current_exe_dir() -> PathBuf {
    env::current_exe()
        .ok()
        .and_then(|exe| exe.parent().map(Path::to_path_buf))
        .or_else(|| env::current_dir().ok())
        .unwrap_or_else(|| PathBuf::from("."))
}

fn env_path(key: &str) -> Option<PathBuf> {
    env::var_os(key)
        .map(PathBuf::from)
        .filter(|path| !path.as_os_str().is_empty())
        .map(clean_abs)
}

pub fn clean_abs(path: impl Into<PathBuf>) -> PathBuf {
    let path = path.into();
    if path.as_os_str().is_empty() {
        return path;
    }
    std::path::absolute(&path).unwrap_or(path)
}

pub fn binary_name(base: &str) -> String {
    if cfg!(windows) {
        format!("{base}.exe")
    } else {
        base.to_owned()
    }
}

/// Replaces `destination` with `source`, keeping a temporary backup for
/// rollback if the final rename fails.
pub fn replace_file(source: &Path, destination: &Path) -> std::io::Result<()> {
    if let Some(parent) = destination.parent() {
        std::fs::create_dir_all(parent)?;
    }
    let backup = destination.with_extension("bak-volvid");
    if destination.exists() {
        let _ = std::fs::remove_file(&backup);
        std::fs::rename(destination, &backup)?;
    }
    match std::fs::rename(source, destination) {
        Ok(()) => {
            let _ = std::fs::remove_file(&backup);
            Ok(())
        }
        Err(err) => {
            if backup.exists() {
                let _ = std::fs::rename(&backup, destination);
            }
            Err(err)
        }
    }
}

pub fn write_atomic(path: &Path, bytes: &[u8]) -> std::io::Result<()> {
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)?;
    }
    let dir = path.parent().unwrap_or_else(|| Path::new("."));
    let mut tmp = tempfile::NamedTempFile::new_in(dir)?;
    std::io::Write::write_all(&mut tmp, bytes)?;
    tmp.as_file().sync_all()?;
    tmp.persist(path)
        .map_err(|err| std::io::Error::other(err.error))?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn binary_name_is_platform_aware() {
        let name = binary_name("ytrust");
        if cfg!(windows) {
            assert_eq!(name, "ytrust.exe");
        } else {
            assert_eq!(name, "ytrust");
        }
    }

    #[test]
    fn replaces_file_and_removes_backup() {
        let dir = tempfile::tempdir().unwrap();
        let source = dir.path().join("source");
        let destination = dir.path().join("destination");
        std::fs::write(&source, b"new").unwrap();
        std::fs::write(&destination, b"old").unwrap();

        replace_file(&source, &destination).unwrap();

        assert_eq!(std::fs::read(&destination).unwrap(), b"new");
        assert!(!source.exists());
        assert!(!destination.with_extension("bak-volvid").exists());
    }
}
