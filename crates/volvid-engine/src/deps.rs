use std::time::Duration;

use semver::Version;
use tokio_util::sync::CancellationToken;
use volvid_core::{
    BrowserCookiesInfo, CheckDepsResult, DependencyInfo, DependencySource, JSRuntimeInfo,
    STATUS_ACTIVE, STATUS_NOT_FOUND,
};

use crate::cookies::detect_browser_cookies;
use crate::paths::{AppPaths, clean_abs};
use crate::process::command_output;

/// The app relies on ytrust 1.1.1 adaptive-stream fallback, cookie scoping and
/// progress JSON fields, so older builds are rejected instead of silently
/// producing incomplete downloads.
pub const YTRUST_MIN_VERSION: &str = "1.1.1";
/// Automatic installs must not cross a breaking major release.
pub const YTRUST_MAX_MAJOR: u64 = 1;
pub const MIN_NODE_MAJOR: u32 = 20;
/// ytrust defaults to 500 entries; Volvid expects complete playlists.
pub const PLAYLIST_LIMIT: u64 = 100_000;

const VERSION_TIMEOUT: Duration = Duration::from_millis(3000);

#[derive(Clone, Debug)]
pub struct DepSpec {
    pub key: &'static str,
    pub name: &'static str,
    pub required: bool,
    pub downloadable: bool,
    pub prefer_managed: bool,
    pub version_args: &'static [&'static str],
    pub managed: fn(&AppPaths) -> std::path::PathBuf,
}

pub const YTRUST_SPEC: DepSpec = DepSpec {
    key: "ytrust",
    name: "ytrust",
    required: true,
    downloadable: true,
    // The CLI protocol changes quickly; use the vetted managed build when it
    // exists and fall back to a system installation otherwise.
    prefer_managed: true,
    version_args: &["--version"],
    managed: AppPaths::ytrust_bin,
};

pub const FFMPEG_SPEC: DepSpec = DepSpec {
    key: "ffmpeg",
    name: "ffmpeg",
    required: true,
    downloadable: true,
    prefer_managed: false,
    version_args: &["-version"],
    managed: AppPaths::ffmpeg_bin,
};

pub const NODE_SPEC: DepSpec = DepSpec {
    key: "node",
    name: "node",
    required: false,
    downloadable: true,
    prefer_managed: false,
    version_args: &["--version"],
    managed: AppPaths::node_bin,
};

pub fn detect_deps(paths: &AppPaths) -> CheckDepsResult {
    let ytrust = detect_dependency(&YTRUST_SPEC, paths);
    let ffmpeg = detect_dependency(&FFMPEG_SPEC, paths);
    let node = detect_dependency(&NODE_SPEC, paths);
    let home = std::env::var_os("HOME")
        .or_else(|| std::env::var_os("USERPROFILE"))
        .map(std::path::PathBuf::from)
        .unwrap_or_default();
    let cookies = if home.as_os_str().is_empty() {
        BrowserCookiesInfo {
            status: STATUS_NOT_FOUND.to_owned(),
            ..BrowserCookiesInfo::default()
        }
    } else {
        detect_browser_cookies(&home)
    };
    CheckDepsResult {
        ytrust,
        ffmpeg,
        node,
        cookies,
        // Runtime usability depends on the node version and is resolved by
        // `enrich_deps`; reporting it as active before probing lied to the UI.
        runtime: JSRuntimeInfo {
            status: STATUS_NOT_FOUND.to_owned(),
            ..JSRuntimeInfo::default()
        },
    }
}

pub fn detect_dependency(spec: &DepSpec, paths: &AppPaths) -> DependencyInfo {
    let mut dep = DependencyInfo {
        key: spec.key.to_owned(),
        name: spec.name.to_owned(),
        required: spec.required,
        downloadable: spec.downloadable,
        source: DependencySource::Missing,
        ..DependencyInfo::default()
    };
    let managed = (spec.managed)(paths);
    let managed_available = managed.is_file();
    if spec.prefer_managed && managed_available {
        dep.path = managed.to_string_lossy().into_owned();
        dep.source = DependencySource::Managed;
        dep.available = true;
        return dep;
    }
    if let Ok(path) = which::which(spec.name) {
        dep.path = clean_abs(path).to_string_lossy().into_owned();
        dep.source = DependencySource::System;
        dep.available = true;
    } else if managed_available {
        dep.path = managed.to_string_lossy().into_owned();
        dep.source = DependencySource::Managed;
        dep.available = true;
    }
    dep
}

pub async fn enrich_deps(
    mut deps: CheckDepsResult,
    cancel: Option<CancellationToken>,
) -> CheckDepsResult {
    let (ffmpeg, node) = tokio::join!(
        probe_dep(&deps.ffmpeg, FFMPEG_SPEC.version_args, cancel.clone()),
        probe_dep(&deps.node, NODE_SPEC.version_args, cancel.clone()),
    );
    if deps.ytrust.version.is_empty() {
        deps.ytrust.version =
            probe_dep(&deps.ytrust, YTRUST_SPEC.version_args, cancel.clone()).await;
    }
    if deps.ffmpeg.version.is_empty() {
        deps.ffmpeg.version = ffmpeg;
    }
    if deps.node.version.is_empty() {
        deps.node.version = node;
    }

    if !ytrust_version_supported(&deps.ytrust.version)
        && let Some(path) = alternate_dependency_path(&deps.ytrust, &YTRUST_SPEC)
    {
        let version = probe_version(&path, YTRUST_SPEC.version_args, cancel.clone()).await;
        if ytrust_version_supported(&version) {
            tracing::warn!(
                previous = %deps.ytrust.path,
                replacement = %path,
                version = %version,
                "switching to a supported ytrust installation"
            );
            deps.ytrust.path = path;
            deps.ytrust.version = version;
            deps.ytrust.source = DependencySource::System;
            deps.ytrust.available = true;
        }
    }
    if deps.ytrust.available
        && (deps.ytrust.version.is_empty() || !ytrust_version_supported(&deps.ytrust.version))
    {
        tracing::warn!(
            version = %deps.ytrust.version,
            "installed ytrust is not supported; {YTRUST_MIN_VERSION} or newer 1.x is required"
        );
        deps.ytrust.available = false;
        deps.ytrust.source = DependencySource::Missing;
        deps.ytrust.path.clear();
    }

    // A system Node older than 20 cannot solve ciphered formats. Prefer a
    // managed Node 20+ build when one is already installed.
    if node_major(&deps.node.version).is_none_or(|major| major < MIN_NODE_MAJOR)
        && deps.node.source == DependencySource::System
    {
        let managed = (NODE_SPEC.managed)(&AppPaths::detect());
        if managed.is_file() {
            let version = probe_version(
                &managed.to_string_lossy(),
                NODE_SPEC.version_args,
                cancel.clone(),
            )
            .await;
            if node_major(&version).is_some_and(|major| major >= MIN_NODE_MAJOR) {
                deps.node.path = managed.to_string_lossy().into_owned();
                deps.node.version = version;
                deps.node.source = DependencySource::Managed;
                deps.node.available = true;
            }
        }
    }

    deps.runtime = runtime_info(&deps.node);
    deps
}

fn alternate_dependency_path(dep: &DependencyInfo, spec: &DepSpec) -> Option<String> {
    let current = dep.path.trim();
    let candidate = match dep.source {
        DependencySource::Managed => which::which(spec.name)
            .ok()
            .map(|path| clean_abs(path).to_string_lossy().into_owned()),
        DependencySource::System => {
            let managed = (spec.managed)(&AppPaths::detect());
            managed
                .is_file()
                .then(|| managed.to_string_lossy().into_owned())
        }
        DependencySource::Missing => None,
    }?;
    (!candidate.is_empty() && candidate != current).then_some(candidate)
}

pub fn ytrust_version_supported(version: &str) -> bool {
    let Ok(version) = Version::parse(version.trim()) else {
        return false;
    };
    let Ok(minimum) = Version::parse(YTRUST_MIN_VERSION) else {
        return false;
    };
    version >= minimum && version.major <= YTRUST_MAX_MAJOR
}

fn node_major(version: &str) -> Option<u32> {
    version
        .trim()
        .trim_start_matches('v')
        .split('.')
        .next()
        .and_then(|major| major.parse().ok())
}

fn runtime_info(node: &DependencyInfo) -> JSRuntimeInfo {
    let usable =
        node.available && node_major(&node.version).is_some_and(|major| major >= MIN_NODE_MAJOR);
    if usable {
        JSRuntimeInfo {
            status: STATUS_ACTIVE.to_owned(),
            name: "node".to_owned(),
            path: node.path.clone(),
        }
    } else {
        JSRuntimeInfo {
            status: STATUS_NOT_FOUND.to_owned(),
            ..JSRuntimeInfo::default()
        }
    }
}

async fn probe_dep(
    dep: &DependencyInfo,
    args: &[&str],
    cancel: Option<CancellationToken>,
) -> String {
    if dep.available && dep.version.is_empty() && !dep.path.trim().is_empty() {
        probe_version(&dep.path, args, cancel).await
    } else {
        String::new()
    }
}

pub async fn probe_version(path: &str, args: &[&str], cancel: Option<CancellationToken>) -> String {
    if path.trim().is_empty() {
        return String::new();
    }
    let owned_args: Vec<String> = args.iter().map(|arg| (*arg).to_owned()).collect();
    for _ in 0..2 {
        let output = command_output(path, &owned_args, Some(VERSION_TIMEOUT), cancel.clone()).await;
        if let Ok(output) = output
            && let Some(line) = first_non_empty_line(&String::from_utf8_lossy(&output.stdout))
        {
            return parse_version(&line, args);
        }
        if cancel.as_ref().is_some_and(|token| token.is_cancelled()) {
            break;
        }
    }
    String::new()
}

fn parse_version(line: &str, args: &[&str]) -> String {
    if args.first().is_some_and(|arg| *arg == "-version")
        && let Some(rest) = line.to_ascii_lowercase().strip_prefix("ffmpeg version ")
    {
        let version = rest
            .split_whitespace()
            .next()
            .unwrap_or_default()
            .trim_matches(|c: char| " \t\r\n,;:()[]{}\"'".contains(c));
        // Nightly/static builds report versions such as `n9.0.2`.
        if let Some(version) = version.strip_prefix('n')
            && version.chars().next().is_some_and(|ch| ch.is_ascii_digit())
        {
            return version.to_owned();
        }
        return version.to_owned();
    }
    if let Some(version) = line.trim().strip_prefix("ytrust ") {
        return version.trim().to_owned();
    }
    line.trim().to_owned()
}

pub fn first_non_empty_line(text: &str) -> Option<String> {
    text.replace("\r\n", "\n")
        .lines()
        .map(str::trim)
        .find(|line| !line.is_empty())
        .map(str::to_owned)
}

pub fn ytrust_base_args(deps: &CheckDepsResult) -> Vec<String> {
    let mut args = vec![
        "--no-warnings".to_owned(),
        "--encoding".to_owned(),
        "utf-8".to_owned(),
    ];
    args.extend(ffmpeg_args(deps));
    if deps.cookies.status == STATUS_ACTIVE {
        let mut cookie_arg = deps.cookies.browser.trim().to_owned();
        let profile = deps.cookies.profile_arg.trim();
        if !profile.is_empty() {
            cookie_arg.push(':');
            cookie_arg.push_str(profile);
        }
        if !cookie_arg.is_empty() {
            args.push("--cookies-from-browser".to_owned());
            args.push(cookie_arg);
        }
    }
    if deps.runtime.status == STATUS_ACTIVE && !deps.runtime.path.trim().is_empty() {
        args.push("--js-runtimes".to_owned());
        args.push(format!("node:{}", deps.runtime.path.trim()));
    }
    args
}

pub fn ytrust_command_args(deps: &CheckDepsResult, base: &[String]) -> Vec<String> {
    let mut args = ytrust_base_args(deps);
    args.extend(base.iter().cloned());
    args
}

pub fn ytrust_bin(deps: &CheckDepsResult) -> Option<&str> {
    let path = deps.ytrust.path.trim();
    (!path.is_empty()).then_some(path)
}

/// Renders the shared `--ffmpeg-location` prefix understood by ytrust.
fn ffmpeg_args(deps: &CheckDepsResult) -> Vec<String> {
    let bin = deps.ffmpeg.path.trim();
    if bin.is_empty() {
        Vec::new()
    } else {
        vec!["--ffmpeg-location".to_owned(), bin.to_owned()]
    }
}

pub fn ffmpeg_bin(deps: &CheckDepsResult, paths: &AppPaths) -> String {
    let path = deps.ffmpeg.path.trim();
    if !path.is_empty() {
        path.to_owned()
    } else {
        paths.ffmpeg_bin().to_string_lossy().into_owned()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_ffmpeg_version() {
        assert_eq!(
            parse_version(
                "ffmpeg version 8.0.1 Copyright (c) 2000-2025",
                &["-version"]
            ),
            "8.0.1"
        );
        assert_eq!(
            parse_version(
                "ffmpeg version n9.0.2 Copyright (c) 2000-2025",
                &["-version"]
            ),
            "9.0.2"
        );
    }

    #[test]
    fn parses_ytrust_version() {
        assert_eq!(parse_version("ytrust 1.1.1", &["--version"]), "1.1.1");
    }

    #[test]
    fn rejects_old_and_future_ytrust_majors() {
        assert!(ytrust_version_supported("1.1.1"));
        assert!(ytrust_version_supported("1.7.3"));
        assert!(!ytrust_version_supported("1.1.0"));
        assert!(!ytrust_version_supported("1.0.1"));
        assert!(!ytrust_version_supported("0.4.3"));
        assert!(!ytrust_version_supported("2.0.0"));
        assert!(!ytrust_version_supported("garbage"));
    }

    #[test]
    fn requires_node_20_for_js_challenges() {
        assert_eq!(node_major("v26.13.1"), Some(26));
        assert_eq!(node_major("20.0.0"), Some(20));
        assert_eq!(node_major("v18.19.0"), Some(18));
    }

    #[test]
    fn builds_base_args_with_cookies() {
        let mut deps = CheckDepsResult::default();
        deps.cookies.status = STATUS_ACTIVE.to_owned();
        deps.cookies.browser = "firefox".to_owned();
        deps.cookies.profile_arg = "/home/u/.mozilla/firefox/x.default".to_owned();
        let args = ytrust_base_args(&deps);
        assert!(args.iter().any(|arg| arg == "--cookies-from-browser"));
        assert!(!args.iter().any(|arg| arg == "--ignore-config"));
        let encoding = args.iter().position(|arg| arg == "--encoding").unwrap();
        assert_eq!(args.get(encoding + 1).map(String::as_str), Some("utf-8"));
    }
}
