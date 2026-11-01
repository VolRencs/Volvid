use std::path::PathBuf;

use serde::{Deserialize, Serialize};
use volvid_core::Locale;

use crate::paths::{AppPaths, clean_abs, write_atomic};

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(default)]
pub struct Config {
    pub locale: Locale,
    pub downloads_dir: Option<PathBuf>,
    pub theme: Theme,
    pub max_parallel_batches: usize,
    pub playlist_workers: usize,
    pub import_legacy: bool,
}

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Theme {
    #[default]
    System,
    Dark,
    Light,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            locale: Locale::En,
            downloads_dir: None,
            theme: Theme::System,
            max_parallel_batches: 1,
            playlist_workers: 4,
            import_legacy: true,
        }
    }
}

impl Config {
    pub fn load(paths: &AppPaths) -> Self {
        let path = paths.config_file();
        let existed = path.is_file();
        let mut parsed_ok = false;
        let mut config = match std::fs::read_to_string(&path) {
            Ok(text) => match toml::from_str::<Self>(&text) {
                Ok(config) => {
                    parsed_ok = true;
                    config
                }
                Err(err) => {
                    tracing::warn!(%err, path = %path.display(), "config parse failed; keeping a backup");
                    let backup = path.with_extension("toml.bak");
                    let _ = std::fs::remove_file(&backup);
                    let _ = std::fs::rename(&path, &backup);
                    Self::default()
                }
            },
            Err(err) if err.kind() == std::io::ErrorKind::NotFound => Self::default(),
            Err(err) => {
                tracing::warn!(%err, path = %path.display(), "could not read config");
                Self::default()
            }
        };
        if config.import_legacy && (!existed || parsed_ok) {
            import_legacy(paths, &mut config);
            config.import_legacy = false;
            let _ = config.save(paths);
        }
        if config.downloads_dir.is_none() {
            config.downloads_dir = Some(paths.downloads_dir.clone());
        }
        config.playlist_workers = config.playlist_workers.clamp(1, 8);
        config.max_parallel_batches = config.max_parallel_batches.clamp(1, 3);
        config
    }

    pub fn save(&self, paths: &AppPaths) -> Result<(), ConfigError> {
        if let Some(parent) = paths.config_file().parent() {
            std::fs::create_dir_all(parent)?;
        }
        let text = toml::to_string_pretty(self).map_err(ConfigError::Serialize)?;
        write_atomic(&paths.config_file(), text.as_bytes())?;
        Ok(())
    }

    pub fn downloads_dir(&self, paths: &AppPaths) -> PathBuf {
        self.downloads_dir
            .clone()
            .unwrap_or_else(|| paths.downloads_dir.clone())
    }

    pub fn set_downloads_dir(&mut self, path: impl Into<PathBuf>) {
        self.downloads_dir = Some(clean_abs(path.into()));
    }
}

fn import_legacy(paths: &AppPaths, config: &mut Config) {
    if let Ok(text) = std::fs::read_to_string(paths.config_dir.join(".volvid_locale")) {
        config.locale = volvid_core::parse_locale(&text);
    }
    if let Ok(text) = std::fs::read_to_string(paths.config_dir.join(".volvid_downloads_dir")) {
        let path = clean_abs(PathBuf::from(text.trim()));
        if path.is_dir() {
            config.downloads_dir = Some(path);
        }
    }
}

#[derive(Debug, thiserror::Error)]
pub enum ConfigError {
    #[error("config I/O error: {0}")]
    Io(#[from] std::io::Error),
    #[error("config serialization error: {0}")]
    Serialize(toml::ser::Error),
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn corrupt_config_is_backed_up_without_overwriting_defaults() {
        let dir = tempfile::tempdir().unwrap();
        let paths = AppPaths {
            config_dir: dir.path().join("config"),
            data_dir: dir.path().join("data"),
            deps_dir: dir.path().join("deps"),
            downloads_dir: dir.path().join("downloads"),
        };
        std::fs::create_dir_all(&paths.config_dir).unwrap();
        std::fs::write(paths.config_file(), "locale = \"not-a-locale\"\n").unwrap();
        let config = Config::load(&paths);
        assert_eq!(config.locale, Locale::En);
        assert!(paths.config_file().with_extension("toml.bak").is_file());
        assert!(!paths.config_file().is_file());
    }
}
