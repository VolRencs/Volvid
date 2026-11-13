use std::fs::File;
use std::io::{Read, Write};
use std::path::{Path, PathBuf};

use anyhow::{Context, Result, bail};
use semver::Version;
use serde_json::Value;
use sha2::{Digest, Sha256};
use tokio::sync::mpsc;
use tokio_util::sync::CancellationToken;
use volvid_core::{FileProgress, Locale};

use crate::deps::{YTRUST_MAX_MAJOR, YTRUST_MIN_VERSION, probe_version, ytrust_version_supported};
use crate::http::{download_file, fetch_json, fetch_text};
use crate::paths::{AppPaths, binary_name, replace_file};

const YTRUST_RELEASES: &str = "https://api.github.com/repos/VolRencs/ytrust/releases/latest";
const NODE_BASE: &str = "https://nodejs.org/download/release/latest/";
const FFMPEG_WIN: &str = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip";
const FFMPEG_LINUX: &str = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz";

pub async fn install_dependency(
    paths: &AppPaths,
    client: &reqwest::Client,
    key: &str,
    locale: Locale,
    cancel: Option<CancellationToken>,
    progress: mpsc::UnboundedSender<FileProgress>,
) -> Result<()> {
    let result = match key.trim() {
        "ytrust" => install_ytrust(paths, client, locale, cancel, progress.clone()).await,
        "ffmpeg" => install_ffmpeg(paths, client, locale, cancel, progress.clone()).await,
        "node" => install_node(paths, client, locale, cancel, progress.clone()).await,
        other => bail!("unsupported dependency: {other}"),
    };
    if result.is_err() {
        let _ = progress.send(FileProgress::failed());
    }
    result
}

async fn install_ytrust(
    paths: &AppPaths,
    client: &reqwest::Client,
    locale: Locale,
    cancel: Option<CancellationToken>,
    progress: mpsc::UnboundedSender<FileProgress>,
) -> Result<()> {
    let platform = platform_info()?;
    std::fs::create_dir_all(&paths.deps_dir)?;
    let release = fetch_json(client, YTRUST_RELEASES)
        .await
        .context("fetch ytrust release metadata")?;
    let tag = release
        .get("tag_name")
        .and_then(Value::as_str)
        .ok_or_else(|| anyhow::anyhow!("ytrust release has no tag"))?;
    let version_text = tag.trim_start_matches('v');
    let version = Version::parse(version_text)
        .with_context(|| format!("invalid ytrust release version {version_text}"))?;
    if version.major > YTRUST_MAX_MAJOR {
        bail!(
            "ytrust {version} is newer than the supported {YTRUST_MAX_MAJOR}.x series; update Volvid"
        );
    }
    if version < Version::parse(YTRUST_MIN_VERSION)? {
        bail!("ytrust {YTRUST_MIN_VERSION} or newer is required, latest is {version}");
    }

    let (asset_name, asset_url) = release_asset(&release, platform.ytrust_suffix)?;
    let (_, checksums_url) = release_asset(&release, "SHA256SUMS")?;
    let manifest = fetch_text(client, checksums_url).await?;
    let checksum = checksum_from_manifest(&manifest, asset_name)?;

    let staging = tempfile::tempdir_in(&paths.deps_dir)?;
    let archive = staging.path().join(asset_name);
    download_file(
        client,
        asset_url,
        &archive,
        locale,
        Some(&progress),
        cancel.clone(),
    )
    .await?;
    verify_sha256(&archive, &checksum)?;

    let staged = staging.path().join(binary_name("ytrust"));
    extract_binaries(&archive, &[(binary_name("ytrust"), staged.clone())])?;
    make_executable(&staged)?;
    let installed = probe_version(&staged.to_string_lossy(), &["--version"], cancel).await;
    if installed.is_empty() {
        bail!("downloaded ytrust does not run");
    }
    if !ytrust_version_supported(&installed) {
        bail!("downloaded ytrust {installed} is older than {YTRUST_MIN_VERSION}");
    }
    replace_file(&staged, &paths.ytrust_bin())?;
    Ok(())
}

fn release_asset<'a>(release: &'a Value, suffix: &str) -> Result<(&'a str, &'a str)> {
    let assets = release
        .get("assets")
        .and_then(Value::as_array)
        .ok_or_else(|| anyhow::anyhow!("ytrust release has no assets"))?;
    for asset in assets {
        let Some(name) = asset.get("name").and_then(Value::as_str) else {
            continue;
        };
        if !name.ends_with(suffix) {
            continue;
        }
        let url = asset
            .get("browser_download_url")
            .and_then(Value::as_str)
            .ok_or_else(|| anyhow::anyhow!("asset {name} has no download URL"))?;
        return Ok((name, url));
    }
    bail!("ytrust release asset with suffix {suffix} not found")
}

async fn install_ffmpeg(
    paths: &AppPaths,
    client: &reqwest::Client,
    locale: Locale,
    cancel: Option<CancellationToken>,
    progress: mpsc::UnboundedSender<FileProgress>,
) -> Result<()> {
    let platform = platform_info()?;
    std::fs::create_dir_all(&paths.deps_dir)?;
    let staging = tempfile::tempdir_in(&paths.deps_dir)?;
    let archive_name = platform
        .ffmpeg_url
        .rsplit('/')
        .next()
        .unwrap_or("ffmpeg-archive");
    let archive = staging.path().join(archive_name);
    download_file(
        client,
        platform.ffmpeg_url,
        &archive,
        locale,
        Some(&progress),
        cancel.clone(),
    )
    .await?;
    if let Ok(expected) = std::env::var("VOLVID_FFMPEG_SHA256")
        && !expected.trim().is_empty()
    {
        verify_sha256(&archive, expected.trim())?;
    }
    let ffmpeg = staging.path().join(binary_name("ffmpeg"));
    let ffprobe = staging.path().join(binary_name("ffprobe"));
    extract_binaries(
        &archive,
        &[
            (binary_name("ffmpeg"), ffmpeg.clone()),
            (binary_name("ffprobe"), ffprobe.clone()),
        ],
    )?;
    make_executable(&ffmpeg)?;
    make_executable(&ffprobe)?;
    if probe_version(&ffmpeg.to_string_lossy(), &["-version"], cancel.clone())
        .await
        .is_empty()
    {
        bail!("downloaded ffmpeg does not run");
    }
    if probe_version(&ffprobe.to_string_lossy(), &["-version"], cancel)
        .await
        .is_empty()
    {
        bail!("downloaded ffprobe does not run");
    }
    replace_file(&ffmpeg, &paths.ffmpeg_bin())?;
    replace_file(&ffprobe, &paths.ffprobe_bin())?;
    Ok(())
}

async fn install_node(
    paths: &AppPaths,
    client: &reqwest::Client,
    locale: Locale,
    cancel: Option<CancellationToken>,
    progress: mpsc::UnboundedSender<FileProgress>,
) -> Result<()> {
    let platform = platform_info()?;
    let manifest = fetch_text(client, &format!("{NODE_BASE}SHASUMS256.txt")).await?;
    let (asset, checksum) = asset_from_manifest(&manifest, platform.node_suffix)?;
    std::fs::create_dir_all(&paths.deps_dir)?;
    let staging = tempfile::tempdir_in(&paths.deps_dir)?;
    let archive = staging.path().join(&asset);
    download_file(
        client,
        &format!("{NODE_BASE}{asset}"),
        &archive,
        locale,
        Some(&progress),
        cancel.clone(),
    )
    .await?;
    verify_sha256(&archive, &checksum)?;
    let node = staging.path().join(binary_name("node"));
    extract_binaries(&archive, &[(binary_name("node"), node.clone())])?;
    make_executable(&node)?;
    if probe_version(&node.to_string_lossy(), &["--version"], cancel)
        .await
        .is_empty()
    {
        bail!("downloaded node does not run");
    }
    replace_file(&node, &paths.node_bin())?;
    Ok(())
}

// Windows has no executable permission bit, so `path` stays unused there.
#[cfg_attr(windows, allow(unused_variables))]
fn make_executable(path: &Path) -> Result<()> {
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        std::fs::set_permissions(path, std::fs::Permissions::from_mode(0o755))?;
    }
    Ok(())
}

fn verify_sha256(path: &Path, expected: &str) -> Result<()> {
    let expected = expected.trim().to_ascii_lowercase();
    if expected.len() != 64 || !expected.chars().all(|ch| ch.is_ascii_hexdigit()) {
        bail!("invalid SHA-256 checksum");
    }
    let mut file = File::open(path)?;
    let mut hasher = Sha256::new();
    let mut buffer = vec![0u8; 1 << 20];
    loop {
        let read = file.read(&mut buffer)?;
        if read == 0 {
            break;
        }
        hasher.update(&buffer[..read]);
    }
    let actual = hex::encode(hasher.finalize());
    if actual != expected {
        bail!("sha256 mismatch for {}", path.display());
    }
    Ok(())
}

fn checksum_from_manifest(manifest: &str, asset: &str) -> Result<String> {
    for line in manifest.lines() {
        let fields: Vec<&str> = line.split_whitespace().collect();
        if fields.len() >= 2 && fields[fields.len() - 1].trim_start_matches('*') == asset {
            return Ok(fields[0].to_owned());
        }
    }
    bail!("asset {asset} not found in checksum manifest")
}

fn asset_from_manifest(manifest: &str, suffix: &str) -> Result<(String, String)> {
    for line in manifest.lines() {
        let fields: Vec<&str> = line.split_whitespace().collect();
        if fields.len() < 2 {
            continue;
        }
        let asset = fields[fields.len() - 1].trim_start_matches('*');
        if asset.ends_with(suffix) {
            let name = Path::new(asset)
                .file_name()
                .map(|name| name.to_string_lossy().into_owned())
                .unwrap_or_default();
            if name.is_empty() {
                continue;
            }
            return Ok((name, fields[0].to_owned()));
        }
    }
    bail!("asset with suffix {suffix} not found in manifest")
}

fn extract_binaries(archive: &Path, targets: &[(String, PathBuf)]) -> Result<()> {
    let lower = archive.to_string_lossy().to_ascii_lowercase();
    if lower.ends_with(".zip") {
        extract_zip(archive, targets)
    } else if lower.ends_with(".tar.gz") || lower.ends_with(".tgz") {
        extract_tar_gz(archive, targets)
    } else {
        extract_tar_xz(archive, targets)
    }
}

fn extract_zip(archive: &Path, targets: &[(String, PathBuf)]) -> Result<()> {
    let file = File::open(archive)?;
    let mut zip = zip::ZipArchive::new(file)?;
    for index in 0..zip.len() {
        let mut entry = zip.by_index(index)?;
        let entry_name = entry.name()?.into_owned();
        let Some(name) = Path::new(&entry_name)
            .file_name()
            .map(|name| name.to_string_lossy().into_owned())
        else {
            continue;
        };
        let Some((_, target)) = targets.iter().find(|(expected, _)| *expected == name) else {
            continue;
        };
        if let Some(parent) = target.parent() {
            std::fs::create_dir_all(parent)?;
        }
        let mut out = File::create(target)?;
        std::io::copy(&mut entry, &mut out)?;
        out.flush()?;
    }
    for (name, target) in targets {
        if !target.is_file() {
            bail!("{name} not found in archive");
        }
    }
    Ok(())
}

fn extract_tar_xz(archive: &Path, targets: &[(String, PathBuf)]) -> Result<()> {
    let file = File::open(archive)?;
    extract_tar(xz2::read::XzDecoder::new(file), targets)
}

fn extract_tar_gz(archive: &Path, targets: &[(String, PathBuf)]) -> Result<()> {
    let file = File::open(archive)?;
    extract_tar(flate2::read::GzDecoder::new(file), targets)
}

fn extract_tar<R: Read>(reader: R, targets: &[(String, PathBuf)]) -> Result<()> {
    let mut tar = tar::Archive::new(reader);
    let mut found = vec![false; targets.len()];
    for entry in tar.entries()? {
        let mut entry = entry?;
        let path = entry.path()?.to_path_buf();
        let Some(name) = path
            .file_name()
            .map(|name| name.to_string_lossy().into_owned())
        else {
            continue;
        };
        let Some((index, (_, target))) = targets
            .iter()
            .enumerate()
            .find(|(_, (expected, _))| *expected == name)
        else {
            continue;
        };
        if let Some(parent) = target.parent() {
            std::fs::create_dir_all(parent)?;
        }
        let mut out = File::create(target)?;
        std::io::copy(&mut entry, &mut out)?;
        out.flush()?;
        found[index] = true;
    }
    for (index, (name, _)) in targets.iter().enumerate() {
        if !found[index] {
            bail!("{name} not found in archive");
        }
    }
    Ok(())
}

struct PlatformInfo {
    ytrust_suffix: &'static str,
    ffmpeg_url: &'static str,
    node_suffix: &'static str,
}

fn platform_info() -> Result<PlatformInfo> {
    if cfg!(all(target_os = "windows", target_arch = "x86_64")) {
        Ok(PlatformInfo {
            ytrust_suffix: "-x86_64-pc-windows-msvc.zip",
            ffmpeg_url: FFMPEG_WIN,
            node_suffix: "-win-x64.zip",
        })
    } else if cfg!(all(target_os = "linux", target_arch = "x86_64")) {
        Ok(PlatformInfo {
            ytrust_suffix: "-x86_64-unknown-linux-gnu.tar.gz",
            ffmpeg_url: FFMPEG_LINUX,
            node_suffix: "-linux-x64.tar.gz",
        })
    } else {
        bail!("unsupported platform for managed dependencies")
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_checksum_manifests() {
        let manifest = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  ytrust-v1.1.1-x86_64-unknown-linux-gnu.tar.gz\n";
        assert_eq!(
            checksum_from_manifest(manifest, "ytrust-v1.1.1-x86_64-unknown-linux-gnu.tar.gz")
                .unwrap(),
            "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
        );
    }

    #[test]
    fn finds_release_assets_by_suffix() {
        let release: Value = serde_json::from_str(
            r#"{
                "tag_name": "v1.1.1",
                "assets": [
                    {"name": "SHA256SUMS", "browser_download_url": "https://example.com/SHA256SUMS"},
                    {"name": "ytrust-v1.1.1-x86_64-unknown-linux-gnu.tar.gz", "browser_download_url": "https://example.com/linux.tar.gz"}
                ]
            }"#,
        )
        .unwrap();
        let (name, url) = release_asset(&release, "-x86_64-unknown-linux-gnu.tar.gz").unwrap();
        assert_eq!(name, "ytrust-v1.1.1-x86_64-unknown-linux-gnu.tar.gz");
        assert_eq!(url, "https://example.com/linux.tar.gz");
        assert_eq!(
            release_asset(&release, "SHA256SUMS").unwrap().0,
            "SHA256SUMS"
        );
    }

    #[test]
    fn accepts_only_supported_ytrust_releases() {
        assert!(!ytrust_version_supported("1.0.0"));
        assert!(!ytrust_version_supported("1.1.0"));
        assert!(!ytrust_version_supported("1.1.1"));
        assert!(ytrust_version_supported("1.1.2"));
        assert!(ytrust_version_supported("1.2.3"));
        assert!(!ytrust_version_supported("0.4.3"));
        assert!(!ytrust_version_supported("2.0.0"));
    }

    #[test]
    fn finds_node_asset() {
        let manifest = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  node-v99.0.0-linux-x64.tar.gz\n";
        let (name, _) = asset_from_manifest(manifest, "-linux-x64.tar.gz").unwrap();
        assert_eq!(name, "node-v99.0.0-linux-x64.tar.gz");
    }

    #[test]
    fn extracts_gzip_node_archive() {
        let dir = tempfile::tempdir().unwrap();
        let archive = dir.path().join("node-v99-linux-x64.tar.gz");
        let file = File::create(&archive).unwrap();
        let encoder = flate2::write::GzEncoder::new(file, flate2::Compression::fast());
        let mut builder = tar::Builder::new(encoder);
        let data = b"node binary";
        let mut header = tar::Header::new_gnu();
        header.set_size(data.len() as u64);
        header.set_mode(0o755);
        header.set_cksum();
        builder
            .append_data(&mut header, "node-v99/bin/node", &data[..])
            .unwrap();
        builder.into_inner().unwrap().finish().unwrap();

        let target = dir.path().join("node");
        extract_binaries(&archive, &[("node".to_owned(), target.clone())]).unwrap();
        assert!(target.is_file());
    }
}
