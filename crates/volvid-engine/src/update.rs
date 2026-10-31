use anyhow::{Context, Result, bail};
use semver::Version;
use tokio::sync::mpsc;
use tokio_util::sync::CancellationToken;
use url::Url;
use volvid_core::{FileProgress, UpdateInfo};

use crate::http::{download_file, fetch_json};

const GITHUB_API: &str = "https://api.github.com/repos/VolRencs/Volvid/releases/latest";

pub async fn check_update(client: &reqwest::Client) -> Result<Option<UpdateInfo>> {
    let data = fetch_json(client, GITHUB_API).await?;
    let latest = data
        .get("tag_name")
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default()
        .trim_start_matches('v')
        .to_owned();
    let current = env!("CARGO_PKG_VERSION");
    if latest.is_empty() || !version_gt(&latest, current) {
        return Ok(None);
    }
    let asset_name = if cfg!(windows) {
        "Volvid.exe"
    } else {
        "Volvid"
    };
    let Some(assets) = data.get("assets").and_then(serde_json::Value::as_array) else {
        return Ok(None);
    };
    for asset in assets {
        if asset.get("name").and_then(serde_json::Value::as_str) != Some(asset_name) {
            continue;
        }
        let url = asset
            .get("browser_download_url")
            .and_then(serde_json::Value::as_str)
            .unwrap_or_default();
        if !valid_update_url(url, asset_name) {
            continue;
        }
        return Ok(Some(UpdateInfo {
            latest,
            download_url: url.to_owned(),
        }));
    }
    Ok(None)
}

fn valid_update_url(raw: &str, asset_name: &str) -> bool {
    let Ok(url) = Url::parse(raw.trim()) else {
        return false;
    };
    if url.scheme() != "https"
        || !url
            .host_str()
            .is_some_and(|host| host.eq_ignore_ascii_case("github.com"))
    {
        return false;
    }
    if url.username() != "" || url.password().is_some() {
        return false;
    }
    let path = url.path().trim_start_matches('/');
    path.starts_with("VolRencs/Volvid/releases/download/")
        && path.rsplit('/').next() == Some(asset_name)
}

fn version_gt(a: &str, b: &str) -> bool {
    let parse = |value: &str| Version::parse(value.trim_start_matches('v'));
    match (parse(a), parse(b)) {
        (Ok(a), Ok(b)) => a > b,
        _ => false,
    }
}

pub async fn apply_update(
    client: &reqwest::Client,
    info: &UpdateInfo,
    locale: volvid_core::Locale,
    cancel: Option<CancellationToken>,
    progress: mpsc::UnboundedSender<FileProgress>,
) -> Result<()> {
    if info.download_url.trim().is_empty() {
        bail!("update info is empty");
    }
    let exe = std::env::current_exe().context("locate executable")?;
    let tmp = exe.with_extension("new");
    let result = download_file(
        client,
        &info.download_url,
        &tmp,
        locale,
        Some(&progress),
        cancel,
    )
    .await;
    if let Err(err) = result {
        let _ = std::fs::remove_file(&tmp);
        let _ = progress.send(FileProgress {
            done: true,
            err: Some(err.to_string()),
            ..FileProgress::default()
        });
        return Err(err);
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let _ = std::fs::set_permissions(&tmp, std::fs::Permissions::from_mode(0o755));
    }
    if let Err(err) = self_replace::self_replace(&tmp) {
        let _ = std::fs::remove_file(&tmp);
        let _ = progress.send(FileProgress {
            done: true,
            err: Some(err.to_string()),
            ..FileProgress::default()
        });
        bail!("replace executable: {err}");
    }
    Ok(())
}

pub fn cleanup_stale_update_files() {
    if let Ok(exe) = std::env::current_exe() {
        let _ = std::fs::remove_file(exe.with_extension("new"));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn compares_versions() {
        assert!(version_gt("8.0.0", "7.4.5"));
        assert!(!version_gt("7.4.5", "7.4.5"));
        assert!(!version_gt("v8.0.0", "8.0.1"));
    }

    #[test]
    fn validates_github_update_urls() {
        assert!(valid_update_url(
            "https://github.com/VolRencs/Volvid/releases/download/v8.0.0/Volvid",
            "Volvid"
        ));
        assert!(!valid_update_url("https://example.com/Volvid", "Volvid"));
    }
}
