use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::process::Stdio;
use std::sync::OnceLock;
use std::time::{Duration, SystemTime};

use anyhow::{Context, Result, bail};
use semver::Version;
use serde::Deserialize;
use serde_json::Value;
use tokio::sync::mpsc;
use tokio_util::sync::CancellationToken;
use url::Url;
use volvid_core::{FileProgress, UpdateInfo};

use crate::hash::{is_sha256, verify_sha256};
use crate::http::{download_file, fetch_json_limited};
use crate::process::{command_output, std_command};

/// Stable asset name published by the release workflow. GitHub serves the
/// manifest from the latest non-draft, non-prerelease release without the
/// rate-limited API.
const LATEST_MANIFEST_URL: &str =
    "https://github.com/VolRencs/Volvid/releases/latest/download/latest.json";
const GITHUB_API: &str = "https://api.github.com/repos/VolRencs/Volvid/releases/latest";
const MANIFEST_ENV: &str = "VOLVID_UPDATE_MANIFEST";
const RELEASE_PATH_PREFIX: &str = "VolRencs/Volvid/releases/download/";

const DOWNLOAD_ATTEMPTS: usize = 3;
const HEALTH_ATTEMPTS: usize = 3;
const HEALTH_TIMEOUT: Duration = Duration::from_secs(20);
const DOWNLOAD_RETRY_DELAY: Duration = Duration::from_secs(1);
const MAX_METADATA_BYTES: usize = 1 << 20;

const OLD_FILE_AGE: Duration = Duration::from_secs(5 * 60);
const TEMP_FILE_AGE: Duration = Duration::from_secs(60 * 60);
const LEGACY_FILE_AGE: Duration = Duration::from_secs(24 * 60 * 60);

/// Linux resolves `/proc/self/exe` to the relocated inode after the old
/// executable has been renamed aside, so remember the install path explicitly.
static INSTALLED_EXE: OnceLock<PathBuf> = OnceLock::new();

fn installed_exe() -> Result<PathBuf> {
    if let Some(path) = INSTALLED_EXE.get() {
        return Ok(path.clone());
    }
    std::env::current_exe().context("locate executable")
}

#[derive(Debug, Deserialize)]
struct ReleaseManifest {
    version: String,
    #[serde(default)]
    assets: HashMap<String, ManifestAsset>,
}

#[derive(Debug, Deserialize)]
struct ManifestAsset {
    url: String,
    #[serde(default)]
    size: Option<u64>,
    #[serde(default)]
    sha256: Option<String>,
}

pub async fn check_update(client: &reqwest::Client) -> Result<Option<UpdateInfo>> {
    match check_manifest(client).await {
        Ok(info) => Ok(info),
        Err(err) => {
            tracing::warn!(
                err = ?err,
                "latest.json update check failed; falling back to the GitHub API"
            );
            check_api(client).await.with_context(|| {
                format!("GitHub API fallback failed after manifest error: {err:#}")
            })
        }
    }
}

async fn check_manifest(client: &reqwest::Client) -> Result<Option<UpdateInfo>> {
    let url = std::env::var(MANIFEST_ENV)
        .ok()
        .map(|value| value.trim().to_owned())
        .filter(|value| !value.is_empty())
        .unwrap_or_else(|| LATEST_MANIFEST_URL.to_owned());
    let data = fetch_json_limited(client, &url, MAX_METADATA_BYTES)
        .await
        .with_context(|| format!("fetch update manifest {url}"))?;
    let manifest: ReleaseManifest =
        serde_json::from_value(data).context("parse update manifest")?;
    manifest_to_info(
        &manifest,
        env!("CARGO_PKG_VERSION"),
        &platform_key(),
        update_asset_name(),
    )
}

fn manifest_to_info(
    manifest: &ReleaseManifest,
    current: &str,
    platform: &str,
    asset_name: &str,
) -> Result<Option<UpdateInfo>> {
    let latest = normalize_version(&manifest.version)?;
    if !version_gt(&latest, current) {
        return Ok(None);
    }
    let Some(asset) = manifest.assets.get(platform) else {
        bail!("release {latest} has no build for {platform} yet; it may still be uploading");
    };
    if !valid_update_url(&asset.url, asset_name) {
        bail!("release {latest} contains an invalid download URL for {platform}");
    }
    if asset.size == Some(0) {
        bail!("release {latest} contains an empty build for {platform}");
    }
    let sha256 = normalize_sha256(asset.sha256.as_deref())?;
    Ok(Some(UpdateInfo {
        latest,
        download_url: asset.url.trim().to_owned(),
        sha256,
        size: asset.size,
    }))
}

async fn check_api(client: &reqwest::Client) -> Result<Option<UpdateInfo>> {
    let data = fetch_json_limited(client, GITHUB_API, MAX_METADATA_BYTES)
        .await
        .context("fetch latest GitHub release")?;
    let Some(tag) = data.get("tag_name").and_then(Value::as_str) else {
        bail!("GitHub release metadata has no tag name");
    };
    let latest = normalize_version(tag)?;
    if !version_gt(&latest, env!("CARGO_PKG_VERSION")) {
        return Ok(None);
    }
    let asset_name = update_asset_name();
    let assets = data
        .get("assets")
        .and_then(Value::as_array)
        .context("GitHub release metadata has no asset list")?;
    let Some(asset) = assets
        .iter()
        .find(|asset| asset.get("name").and_then(Value::as_str) == Some(asset_name))
    else {
        bail!("release {latest} does not include {asset_name} yet; it may still be uploading");
    };
    let url = asset
        .get("browser_download_url")
        .and_then(Value::as_str)
        .unwrap_or_default();
    if !valid_update_url(url, asset_name) {
        bail!("release {latest} contains an invalid download URL");
    }
    let size = asset.get("size").and_then(Value::as_u64);
    if size == Some(0) {
        bail!("release {latest} contains an empty build");
    }
    let sha256 = normalize_sha256(asset.get("digest").and_then(Value::as_str))?;
    Ok(Some(UpdateInfo {
        latest,
        download_url: url.trim().to_owned(),
        sha256,
        size,
    }))
}

fn normalize_version(raw: &str) -> Result<String> {
    let trimmed = raw.trim().trim_start_matches('v');
    Version::parse(trimmed).with_context(|| format!("invalid release version {raw:?}"))?;
    Ok(trimmed.to_owned())
}

fn normalize_sha256(raw: Option<&str>) -> Result<Option<String>> {
    let Some(raw) = raw else {
        return Ok(None);
    };
    let value = raw
        .trim()
        .strip_prefix("sha256:")
        .unwrap_or(raw.trim())
        .trim()
        .to_ascii_lowercase();
    if value.is_empty() {
        return Ok(None);
    }
    if !is_sha256(&value) {
        bail!("release metadata contains an invalid SHA-256 checksum");
    }
    Ok(Some(value))
}

fn platform_key() -> String {
    let os = if cfg!(windows) {
        "windows"
    } else if cfg!(target_os = "linux") {
        "linux"
    } else {
        "unknown"
    };
    format!("{os}-{}", std::env::consts::ARCH)
}

fn update_asset_name() -> &'static str {
    if cfg!(windows) {
        "Volvid.exe"
    } else {
        "Volvid"
    }
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
    if !url.username().is_empty() || url.password().is_some() {
        return false;
    }
    let path = url.path().trim_start_matches('/');
    path.starts_with(RELEASE_PATH_PREFIX) && path.rsplit('/').next() == Some(asset_name)
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
    if !valid_update_url(&info.download_url, update_asset_name()) {
        bail!("invalid update URL");
    }
    let exe = std::env::current_exe().context("locate executable")?;
    let install_dir = exe.parent().context("executable has no parent directory")?;
    ensure_install_dir_writable(install_dir)?;

    let staged = staged_path(&exe, &info.latest);
    remove_file_force(&staged);
    tracing::info!(
        version = %info.latest,
        path = %staged.display(),
        "downloading Volvid update"
    );

    let result = async {
        download_verified(
            client,
            info,
            &exe,
            &staged,
            locale,
            &progress,
            cancel.clone(),
        )
        .await?;
        verify_update_runs(&staged, &info.latest, cancel.clone()).await?;
        if cancel.as_ref().is_some_and(CancellationToken::is_cancelled) {
            bail!("operation cancelled");
        }
        install_staged(&exe, &staged).await?;
        let _ = INSTALLED_EXE.set(exe.clone());
        Ok::<(), anyhow::Error>(())
    }
    .await;

    match result {
        Ok(()) => {
            let _ = progress.send(FileProgress {
                pct: 100.0,
                done_b: info.size.unwrap_or_default() as i64,
                total_b: info.size.unwrap_or_default() as i64,
                speed: String::new(),
                done: true,
            });
            tracing::info!(
                version = %info.latest,
                "update verified and installed; restart required"
            );
            Ok(())
        }
        Err(err) => {
            remove_file_force(&staged);
            let _ = progress.send(FileProgress::failed());
            tracing::error!(version = %info.latest, err = ?err, "update failed");
            Err(err)
        }
    }
}

async fn download_verified(
    client: &reqwest::Client,
    info: &UpdateInfo,
    exe: &Path,
    destination: &Path,
    locale: volvid_core::Locale,
    progress: &mpsc::UnboundedSender<FileProgress>,
    cancel: Option<CancellationToken>,
) -> Result<()> {
    let mut last_error = None;
    for attempt in 1..=DOWNLOAD_ATTEMPTS {
        if cancel.as_ref().is_some_and(CancellationToken::is_cancelled) {
            bail!("operation cancelled");
        }
        remove_file_force(destination);
        let download = download_file(
            client,
            &info.download_url,
            destination,
            locale,
            Some(progress),
            cancel.clone(),
        )
        .await;
        match download {
            Ok(_) => match verify_staged(destination, exe, info).await {
                Ok(()) => return Ok(()),
                Err(err) => {
                    tracing::warn!(attempt, err = ?err, "downloaded update failed verification");
                    last_error = Some(err);
                }
            },
            Err(err) => {
                if cancel.as_ref().is_some_and(CancellationToken::is_cancelled) {
                    bail!("operation cancelled");
                }
                tracing::warn!(attempt, err = ?err, "update download failed");
                last_error = Some(err);
            }
        }
        remove_file_force(destination);
        if attempt < DOWNLOAD_ATTEMPTS {
            tokio::time::sleep(DOWNLOAD_RETRY_DELAY).await;
        }
    }
    Err(last_error.unwrap_or_else(|| anyhow::anyhow!("update download failed")))
}

async fn verify_staged(path: &Path, exe: &Path, info: &UpdateInfo) -> Result<()> {
    let path = path.to_path_buf();
    let exe = exe.to_path_buf();
    let expected_size = info.size;
    let expected_sha = info.sha256.clone();
    tokio::task::spawn_blocking(move || {
        let size = std::fs::metadata(&path)
            .with_context(|| format!("inspect downloaded update {}", path.display()))?
            .len();
        if let Some(expected) = expected_size
            && size != expected
        {
            bail!("downloaded update has the wrong size: {size} bytes, expected {expected}");
        }
        if let Some(expected) = expected_sha {
            verify_sha256(&path, &expected)?;
        }
        verify_executable_format(&path)?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;

            let mode = std::fs::metadata(&exe)
                .map(|metadata| metadata.permissions().mode() & 0o777)
                .unwrap_or(0o755);
            std::fs::set_permissions(&path, std::fs::Permissions::from_mode(mode))
                .context("set update permissions")?;
        }
        #[cfg(not(unix))]
        if let Ok(metadata) = std::fs::metadata(&exe) {
            std::fs::set_permissions(&path, metadata.permissions())
                .context("set update permissions")?;
        }
        Ok::<(), anyhow::Error>(())
    })
    .await
    .context("update verification task failed")??;
    Ok(())
}

fn verify_executable_format(path: &Path) -> Result<()> {
    use std::io::Read;

    let mut file = std::fs::File::open(path)?;
    let mut magic = [0u8; 4];
    let read = file.read(&mut magic)?;
    #[cfg(windows)]
    if read < 2 || &magic[..2] != b"MZ" {
        bail!("downloaded update is not a Windows executable");
    }
    #[cfg(unix)]
    if read < 4 || &magic != b"\x7fELF" {
        bail!("downloaded update is not a Linux executable");
    }
    #[cfg(not(any(windows, unix)))]
    let _ = read;
    Ok(())
}

async fn verify_update_runs(
    path: &Path,
    expected: &str,
    cancel: Option<CancellationToken>,
) -> Result<()> {
    let mut last_error = None;
    for attempt in 1..=HEALTH_ATTEMPTS {
        if cancel.as_ref().is_some_and(CancellationToken::is_cancelled) {
            bail!("operation cancelled");
        }
        match run_version(path, cancel.clone()).await {
            Ok(reported) if reported == expected => return Ok(()),
            Ok(reported) => {
                last_error = Some(anyhow::anyhow!(
                    "downloaded update reported version {reported:?}, expected {expected:?}"
                ));
            }
            Err(err) => last_error = Some(err),
        }
        tracing::warn!(
            attempt,
            error = %last_error.as_ref().map(ToString::to_string).unwrap_or_default(),
            "downloaded update health check failed"
        );
        if attempt < HEALTH_ATTEMPTS {
            tokio::time::sleep(Duration::from_millis(500)).await;
        }
    }
    Err(last_error.unwrap_or_else(|| anyhow::anyhow!("downloaded update did not start")))
}

async fn run_version(path: &Path, cancel: Option<CancellationToken>) -> Result<String> {
    let output = command_output(
        path,
        &["--version".to_owned()],
        Some(HEALTH_TIMEOUT),
        cancel,
    )
    .await
    .context("start the downloaded update")?;
    if !output.success {
        bail!(
            "downloaded update exited unsuccessfully: {}",
            output.stderr.trim()
        );
    }
    let stdout = String::from_utf8_lossy(&output.stdout);
    let reported = stdout
        .split_whitespace()
        .last()
        .unwrap_or_default()
        .trim_start_matches('v')
        .to_owned();
    if reported.is_empty() {
        bail!("downloaded update did not report its version");
    }
    Ok(reported)
}

async fn install_staged(exe: &Path, staged: &Path) -> Result<()> {
    let exe = exe.to_path_buf();
    let staged = staged.to_path_buf();
    tokio::task::spawn_blocking(move || install_staged_sync(&exe, &staged))
        .await
        .context("update install task failed")??;
    Ok(())
}

fn install_staged_sync(exe: &Path, staged: &Path) -> Result<()> {
    let old = old_path(exe, std::process::id());
    remove_file_force(&old);
    retry_rename(exe, &old)
        .with_context(|| format!("move current executable to {}", old.display()))?;
    if let Err(err) = retry_rename(staged, exe) {
        let rollback = retry_rename(&old, exe).or_else(|first| {
            std::fs::copy(&old, exe).map(|_| ()).map_err(|second| {
                std::io::Error::other(format!("rename failed: {first}; copy failed: {second}"))
            })
        });
        if rollback.is_ok() {
            remove_file_force(&old);
        }
        let rollback_text = rollback
            .map(|()| "restored".to_owned())
            .unwrap_or_else(|rollback| rollback.to_string());
        return Err(anyhow::anyhow!(
            "install downloaded executable: {err}; previous executable: {rollback_text}"
        ));
    }
    Ok(())
}

fn retry_rename(from: &Path, to: &Path) -> std::io::Result<()> {
    let mut last_error = None;
    for attempt in 0..10 {
        match std::fs::rename(from, to) {
            Ok(()) => return Ok(()),
            Err(err) => {
                last_error = Some(err);
                if attempt < 9 {
                    std::thread::sleep(Duration::from_millis(100));
                }
            }
        }
    }
    Err(last_error.unwrap_or_else(|| std::io::Error::other("rename failed")))
}

/// Removes an updater artifact. Windows keeps the read-only attribute on files
/// copied from a read-only installation, which blocks deletion.
fn remove_file_force(path: &Path) {
    #[cfg(windows)]
    if let Ok(metadata) = std::fs::metadata(path)
        && metadata.permissions().readonly()
    {
        let mut permissions = metadata.permissions();
        permissions.set_readonly(false);
        let _ = std::fs::set_permissions(path, permissions);
    }
    let _ = std::fs::remove_file(path);
}

fn ensure_install_dir_writable(dir: &Path) -> Result<()> {
    let probe = dir.join(format!(".volvid-write-test-{}", std::process::id()));
    let result = std::fs::write(&probe, b"")
        .and_then(|()| std::fs::remove_file(&probe))
        .map_err(|err| anyhow::anyhow!("{err}"));
    match result {
        Ok(()) => Ok(()),
        Err(err) => bail!(
            "Volvid is installed in a read-only directory ({}): {err}. Download the new version from GitHub.",
            dir.display()
        ),
    }
}

/// Restores the previous executable after a failed restart.
pub fn rollback_update() -> Result<()> {
    let exe = installed_exe().context("locate executable for rollback")?;
    let old = old_path(&exe, std::process::id());
    let failed = failed_path(&exe, std::process::id());
    rollback_at(&exe, &old, &failed)
}

fn rollback_at(exe: &Path, old: &Path, failed: &Path) -> Result<()> {
    if !old.is_file() {
        bail!("no previous Volvid executable to restore");
    }
    remove_file_force(failed);
    if exe.is_file() {
        retry_rename(exe, failed).context("move failed update aside")?;
    }
    retry_rename(old, exe)
        .or_else(|first| {
            std::fs::copy(old, exe).map(|_| ()).map_err(|second| {
                std::io::Error::other(format!("rename failed: {first}; copy failed: {second}"))
            })
        })
        .context("restore previous executable")?;
    remove_file_force(old);
    remove_file_force(failed);
    tracing::warn!(path = %exe.display(), "restored the previous Volvid executable");
    Ok(())
}

fn staged_path(exe: &Path, version: &str) -> PathBuf {
    let name = exe
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("Volvid");
    let stem = name
        .strip_suffix(std::env::consts::EXE_SUFFIX)
        .unwrap_or(name);
    exe.with_file_name(format!(
        "{stem}.update-{version}{}",
        std::env::consts::EXE_SUFFIX
    ))
}

fn old_path(exe: &Path, pid: u32) -> PathBuf {
    let name = exe
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("Volvid");
    exe.with_file_name(format!("{name}.old-{pid}"))
}

fn failed_path(exe: &Path, pid: u32) -> PathBuf {
    let name = exe
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("Volvid");
    exe.with_file_name(format!("{name}.failed-{pid}"))
}

/// Starts a fresh copy of the current executable. The caller is responsible
/// for saving state and closing the current window so the process can exit.
pub fn spawn_restart() -> Result<()> {
    let exe = installed_exe().context("locate executable for restart")?;
    if !exe.is_file() {
        bail!("executable not found: {}", exe.display());
    }
    let mut command = std_command(&exe);
    command
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null());
    #[cfg(unix)]
    {
        use std::os::unix::process::CommandExt;
        command.process_group(0);
    }
    let mut child = command
        .spawn()
        .with_context(|| format!("restart {}", exe.display()))?;
    let pid = child.id();
    // Catch a binary that cannot even stay alive (missing runtime library,
    // immediate startup panic) before the old process exits. The caller rolls
    // back to the previous executable when this fails.
    std::thread::sleep(Duration::from_millis(700));
    match child.try_wait() {
        Ok(Some(status)) => bail!("updated Volvid exited immediately ({status})"),
        Ok(None) => {}
        Err(err) => bail!("could not check the updated Volvid process: {err}"),
    }
    tracing::info!(pid, path = %exe.display(), "started updated Volvid");
    Ok(())
}

/// Removes temporary files left by the updater in the executable directory.
/// Windows may keep a relocated executable locked until the previous process
/// exits, so failures are intentionally ignored and retried on a later start.
pub fn cleanup_stale_update_files() {
    let Ok(exe) = std::env::current_exe() else {
        return;
    };
    let Some(dir) = exe.parent() else {
        return;
    };
    let Some(name) = exe.file_name().and_then(|name| name.to_str()) else {
        return;
    };
    let stem = name
        .strip_suffix(std::env::consts::EXE_SUFFIX)
        .unwrap_or(name);
    let old_prefix = format!("{name}.old-");
    let failed_prefix = format!("{name}.failed-");
    let update_prefix = format!("{stem}.update-");
    let name_new = format!("{name}.new");
    let stem_new = format!("{stem}.new");
    let legacy_prefix = format!(".{stem}.");
    let Ok(entries) = std::fs::read_dir(dir) else {
        return;
    };
    let mut removed = 0usize;
    for entry in entries.flatten() {
        let file_name = entry.file_name();
        let Some(file_name) = file_name.to_str() else {
            continue;
        };
        if !entry
            .file_type()
            .map(|kind| kind.is_file())
            .unwrap_or(false)
        {
            continue;
        }
        let path = entry.path();
        let remove = if file_name.starts_with(&old_prefix) || file_name.starts_with(&failed_prefix)
        {
            older_than(&path, OLD_FILE_AGE)
        } else if file_name.starts_with(&update_prefix)
            || file_name == name_new
            || file_name == stem_new
            || file_name.starts_with(".volvid-download-")
        {
            older_than(&path, TEMP_FILE_AGE)
        } else if file_name.starts_with(&legacy_prefix)
            && (file_name.ends_with(".__relocated__.exe")
                || file_name.ends_with(".__temp__.exe")
                || file_name.ends_with(".__selfdelete__.exe"))
        {
            older_than(&path, LEGACY_FILE_AGE)
        } else {
            false
        };
        if remove {
            let existed = path.is_file();
            remove_file_force(&path);
            if existed && !path.exists() {
                removed += 1;
            }
        }
    }
    if removed > 0 {
        tracing::info!(removed, "removed stale update files");
    }
}

fn older_than(path: &Path, age: Duration) -> bool {
    std::fs::metadata(path)
        .and_then(|metadata| metadata.modified())
        .ok()
        .is_some_and(|modified| {
            SystemTime::now()
                .duration_since(modified)
                .is_ok_and(|elapsed| elapsed > age)
        })
}

#[cfg(test)]
mod tests {
    use super::*;

    const SHA: &str = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";

    fn manifest(platform: &str) -> ReleaseManifest {
        serde_json::from_value(serde_json::json!({
            "version": "8.1.3",
            "assets": {
                platform: {
                    "url": "https://github.com/VolRencs/Volvid/releases/download/8.1.3/Volvid",
                    "size": 12,
                    "sha256": SHA
                }
            }
        }))
        .unwrap()
    }

    #[test]
    fn compares_versions() {
        assert!(version_gt("8.0.0", "7.4.5"));
        assert!(!version_gt("7.4.5", "7.4.5"));
        assert!(!version_gt("v8.0.0", "8.0.1"));
        assert_eq!(normalize_version("v8.1.3").unwrap(), "8.1.3");
        assert!(normalize_version("not-a-version").is_err());
    }

    #[test]
    fn validates_github_update_urls() {
        assert!(valid_update_url(
            "https://github.com/VolRencs/Volvid/releases/download/v8.0.0/Volvid",
            "Volvid"
        ));
        assert!(!valid_update_url("https://example.com/Volvid", "Volvid"));
        assert!(!valid_update_url(
            "https://github.com/VolRencs/Volvid/releases/download/8.1.3/Volvid.exe",
            "Volvid"
        ));
    }

    #[test]
    fn reads_manifest_assets() {
        let info = manifest_to_info(&manifest("linux-x86_64"), "8.1.2", "linux-x86_64", "Volvid")
            .unwrap()
            .unwrap();
        assert_eq!(info.latest, "8.1.3");
        assert_eq!(info.size, Some(12));
        assert_eq!(info.sha256.as_deref(), Some(SHA));
        assert!(
            manifest_to_info(&manifest("linux-x86_64"), "8.1.3", "linux-x86_64", "Volvid")
                .unwrap()
                .is_none()
        );
    }

    #[test]
    fn reports_missing_platform_builds() {
        let err = manifest_to_info(
            &manifest("linux-x86_64"),
            "8.1.2",
            "windows-x86_64",
            "Volvid.exe",
        )
        .unwrap_err()
        .to_string();
        assert!(err.contains("no build for windows-x86_64"));
    }

    #[test]
    fn normalizes_checksums() {
        assert_eq!(
            normalize_sha256(Some(&format!("sha256:{SHA}"))).unwrap(),
            Some(SHA.to_owned())
        );
        assert!(normalize_sha256(Some("nope")).is_err());
        assert_eq!(normalize_sha256(None).unwrap(), None);
    }

    #[test]
    fn swaps_and_rolls_back_files() {
        let dir = tempfile::tempdir().unwrap();
        let exe = dir.path().join("Volvid");
        let staged = dir.path().join("Volvid.update-8.1.3");
        std::fs::write(&exe, b"old").unwrap();
        std::fs::write(&staged, b"new").unwrap();

        install_staged_sync(&exe, &staged).unwrap();
        assert_eq!(std::fs::read(&exe).unwrap(), b"new");
        let old = old_path(&exe, std::process::id());
        assert!(old.is_file());

        rollback_at(&exe, &old, &failed_path(&exe, std::process::id())).unwrap();
        assert_eq!(std::fs::read(&exe).unwrap(), b"old");
        assert!(!old.exists());
    }

    #[test]
    fn names_staged_files_next_to_the_executable() {
        let exe = if cfg!(windows) {
            PathBuf::from("C:/Volvid/Volvid.exe")
        } else {
            PathBuf::from("/opt/volvid/Volvid")
        };
        let staged = staged_path(&exe, "8.1.3");
        assert_eq!(
            staged.file_name().unwrap().to_string_lossy(),
            if cfg!(windows) {
                "Volvid.update-8.1.3.exe"
            } else {
                "Volvid.update-8.1.3"
            }
        );
    }

    #[test]
    fn validates_executable_formats() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("Volvid");
        #[cfg(windows)]
        std::fs::write(&path, b"MZ executable").unwrap();
        #[cfg(unix)]
        std::fs::write(&path, b"\x7fELF executable").unwrap();
        verify_executable_format(&path).unwrap();

        std::fs::write(&path, b"not an executable").unwrap();
        assert!(verify_executable_format(&path).is_err());
    }

    #[tokio::test]
    async fn rejects_staged_files_with_the_wrong_size() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("Volvid.update-8.1.3");
        let exe = dir.path().join("Volvid");
        std::fs::write(&path, b"data").unwrap();
        std::fs::write(&exe, b"old").unwrap();
        let info = UpdateInfo {
            latest: "8.1.3".to_owned(),
            download_url: String::new(),
            sha256: None,
            size: Some(5),
        };
        assert!(verify_staged(&path, &exe, &info).await.is_err());
    }

    #[tokio::test]
    async fn rejects_staged_files_with_the_wrong_checksum() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("Volvid.update-8.1.3");
        let exe = dir.path().join("Volvid");
        std::fs::write(&path, b"data").unwrap();
        std::fs::write(&exe, b"old").unwrap();
        let info = UpdateInfo {
            latest: "8.1.3".to_owned(),
            download_url: String::new(),
            sha256: Some("0".repeat(64)),
            size: Some(4),
        };
        assert!(verify_staged(&path, &exe, &info).await.is_err());
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn reads_the_version_of_a_downloaded_binary() {
        use std::os::unix::fs::PermissionsExt;

        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("Volvid.update-8.1.3");
        std::fs::write(&path, "#!/bin/sh\nprintf 'Volvid 8.1.3\\n'\n").unwrap();
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o755)).unwrap();
        assert_eq!(run_version(&path, None).await.unwrap(), "8.1.3");
    }
}
