use std::time::Duration;

use tokio_util::sync::CancellationToken;
use volvid_core::{
    BrowserCookiesInfo, CheckDepsResult, DependencyInfo, DependencySource, JSRuntimeInfo,
    STATUS_ACTIVE, STATUS_NOT_FOUND,
};

use crate::cookies::detect_browser_cookies;
use crate::paths::{AppPaths, clean_abs};
use crate::process::command_output;

const VERSION_TIMEOUT: Duration = Duration::from_millis(1500);

#[derive(Clone, Debug)]
pub struct DepSpec {
    pub key: &'static str,
    pub name: &'static str,
    pub required: bool,
    pub downloadable: bool,
    pub version_args: &'static [&'static str],
    pub managed: fn(&AppPaths) -> std::path::PathBuf,
}

pub const YTDLP_SPEC: DepSpec = DepSpec {
    key: "ytdlp",
    name: "yt-dlp",
    required: true,
    downloadable: true,
    version_args: &["--version"],
    managed: AppPaths::ytdlp_bin,
};

pub const FFMPEG_SPEC: DepSpec = DepSpec {
    key: "ffmpeg",
    name: "ffmpeg",
    required: true,
    downloadable: true,
    version_args: &["-version"],
    managed: AppPaths::ffmpeg_bin,
};

pub const NODE_SPEC: DepSpec = DepSpec {
    key: "node",
    name: "node",
    required: false,
    downloadable: true,
    version_args: &["--version"],
    managed: AppPaths::node_bin,
};

pub fn detect_deps(paths: &AppPaths) -> CheckDepsResult {
    let ytdlp = detect_dependency(&YTDLP_SPEC, paths);
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
    let runtime = if node.available {
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
    };
    CheckDepsResult {
        ytdlp,
        ffmpeg,
        node,
        cookies,
        runtime,
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
    if let Ok(path) = which::which(spec.name) {
        dep.path = clean_abs(path).to_string_lossy().into_owned();
        dep.source = DependencySource::System;
        dep.available = true;
    } else {
        let managed = (spec.managed)(paths);
        if managed.is_file() {
            dep.path = managed.to_string_lossy().into_owned();
            dep.source = DependencySource::Managed;
            dep.available = true;
        }
    }
    dep
}

pub async fn enrich_deps(
    mut deps: CheckDepsResult,
    cancel: Option<CancellationToken>,
) -> CheckDepsResult {
    let (ytdlp, ffmpeg, node) = tokio::join!(
        probe_dep(&deps.ytdlp, YTDLP_SPEC.version_args, cancel.clone()),
        probe_dep(&deps.ffmpeg, FFMPEG_SPEC.version_args, cancel.clone()),
        probe_dep(&deps.node, NODE_SPEC.version_args, cancel),
    );
    if deps.ytdlp.version.is_empty() {
        deps.ytdlp.version = ytdlp;
    }
    if deps.ffmpeg.version.is_empty() {
        deps.ffmpeg.version = ffmpeg;
    }
    if deps.node.version.is_empty() {
        deps.node.version = node;
    }
    deps
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
        return rest
            .split_whitespace()
            .next()
            .unwrap_or_default()
            .trim_matches(|c: char| " \t\r\n,;:()[]{}\"'".contains(c))
            .to_owned();
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

pub fn ytdlp_base_args(deps: &CheckDepsResult) -> Vec<String> {
    let mut args = vec![
        "--ignore-config".to_owned(),
        "--no-warnings".to_owned(),
        "--encoding".to_owned(),
        "utf-8".to_owned(),
    ];
    args.extend(volvid_core::ffmpeg_args(deps));
    if deps.cookies.status == STATUS_ACTIVE {
        let mut cookie_arg = deps.cookies.browser.trim().to_owned();
        let profile = deps.cookies.ytdlp_profile.trim();
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

pub fn ytdlp_command_args(deps: &CheckDepsResult, base: &[String]) -> Vec<String> {
    let mut args = ytdlp_base_args(deps);
    args.extend(base.iter().cloned());
    args
}

pub fn ytdlp_bin(deps: &CheckDepsResult) -> Option<&str> {
    let path = deps.ytdlp.path.trim();
    (!path.is_empty()).then_some(path)
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
    }

    #[test]
    fn builds_base_args_with_cookies() {
        let mut deps = CheckDepsResult::default();
        deps.cookies.status = STATUS_ACTIVE.to_owned();
        deps.cookies.browser = "firefox".to_owned();
        deps.cookies.ytdlp_profile = "/home/u/.mozilla/firefox/x.default".to_owned();
        let args = ytdlp_base_args(&deps);
        assert!(args.iter().any(|arg| arg == "--cookies-from-browser"));
        let encoding = args.iter().position(|arg| arg == "--encoding").unwrap();
        assert_eq!(args.get(encoding + 1).map(String::as_str), Some("utf-8"));
    }
}
