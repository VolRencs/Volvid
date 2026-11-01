use std::path::{Path, PathBuf};
use std::time::SystemTime;

use volvid_core::{BrowserCookiesInfo, STATUS_ACTIVE, STATUS_NO_PROFILE, STATUS_NOT_FOUND};

#[derive(Clone, Debug)]
struct Candidate {
    browser: String,
    profile: PathBuf,
    modified: SystemTime,
}

/// Detects the newest usable browser cookie database. This intentionally
/// mirrors the old Go heuristic: use the most recently touched profile.
pub fn detect_browser_cookies(home: &Path) -> BrowserCookiesInfo {
    let mut candidates = Vec::new();
    let mut found_root = false;
    for (browser, roots) in browser_roots(home) {
        for root in roots {
            if root.is_dir() {
                found_root = true;
                scan_browser_root(browser, &root, &mut candidates);
            }
        }
    }

    let Some(candidate) = candidates.into_iter().max_by_key(|c| c.modified) else {
        return BrowserCookiesInfo {
            status: if found_root {
                STATUS_NO_PROFILE.to_owned()
            } else {
                STATUS_NOT_FOUND.to_owned()
            },
            ..BrowserCookiesInfo::default()
        };
    };

    let profile_name = candidate
        .profile
        .file_name()
        .map(|name| name.to_string_lossy().into_owned())
        .unwrap_or_default();
    let ytdlp_profile = if cfg!(windows) {
        profile_name.clone()
    } else {
        candidate.profile.to_string_lossy().into_owned()
    };
    BrowserCookiesInfo {
        status: STATUS_ACTIVE.to_owned(),
        browser: candidate.browser,
        profile_name,
        ytdlp_profile,
    }
}

#[cfg(unix)]
fn browser_roots(home: &Path) -> Vec<(&'static str, Vec<PathBuf>)> {
    let config_home = std::env::var_os("XDG_CONFIG_HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|| home.join(".config"));
    vec![
        (
            "firefox",
            vec![
                config_home.join("mozilla/firefox"),
                home.join(".mozilla/firefox"),
                home.join(".librewolf"),
                home.join(".var/app/org.mozilla.firefox/config/mozilla/firefox"),
                home.join(".var/app/org.mozilla.firefox/.mozilla/firefox"),
                home.join("snap/firefox/common/.mozilla/firefox"),
            ],
        ),
        (
            "chrome",
            vec![
                home.join(".config/google-chrome"),
                config_home.join("google-chrome"),
                config_home.join("google-chrome-beta"),
                config_home.join("google-chrome-unstable"),
                home.join(".var/app/com.google.Chrome/config/google-chrome"),
            ],
        ),
        (
            "chromium",
            vec![
                config_home.join("chromium"),
                home.join(".var/app/org.chromium.Chromium/config/chromium"),
                home.join("snap/chromium/common/chromium"),
            ],
        ),
        (
            "brave",
            vec![
                config_home.join("BraveSoftware/Brave-Browser"),
                config_home.join("BraveSoftware/Brave-Origin"),
                home.join(".var/app/com.brave.Browser/config/BraveSoftware/Brave-Browser"),
                home.join(".var/app/com.brave.Browser/config/BraveSoftware/Brave-Origin"),
            ],
        ),
        ("vivaldi", vec![config_home.join("vivaldi")]),
        (
            "opera",
            vec![
                config_home.join("opera"),
                config_home.join("opera-developer"),
                config_home.join("opera-beta"),
            ],
        ),
    ]
}

#[cfg(windows)]
fn browser_roots(_home: &Path) -> Vec<(&'static str, Vec<PathBuf>)> {
    let appdata = std::env::var_os("APPDATA").map(PathBuf::from);
    let local = std::env::var_os("LOCALAPPDATA").map(PathBuf::from);
    let mut roots: Vec<(&'static str, Vec<PathBuf>)> = Vec::new();
    if let Some(appdata) = &appdata {
        roots.push(("firefox", vec![appdata.join("Mozilla/Firefox/Profiles")]));
        roots.push((
            "opera",
            vec![
                appdata.join("Opera Software/Opera Stable"),
                appdata.join("Opera Software/Opera GX Stable"),
                appdata.join("Opera Software/Opera Developer"),
            ],
        ));
    }
    if let Some(local) = &local {
        roots.push((
            "firefox",
            vec![
                local.join("Packages/Mozilla.Firefox/LocalCache/Roaming/Mozilla/Firefox/Profiles"),
            ],
        ));
        roots.push((
            "chrome",
            vec![
                local.join("Google/Chrome/User Data"),
                local.join("Google/Chrome Beta/User Data"),
                local.join("Google/Chrome SxS/User Data"),
            ],
        ));
        roots.push(("chromium", vec![local.join("Chromium/User Data")]));
        roots.push((
            "brave",
            vec![local.join("BraveSoftware/Brave-Browser/User Data")],
        ));
        roots.push(("vivaldi", vec![local.join("Vivaldi/User Data")]));
    }
    roots
}

#[cfg(not(any(unix, windows)))]
fn browser_roots(_home: &Path) -> Vec<(&'static str, Vec<PathBuf>)> {
    Vec::new()
}

fn scan_browser_root(browser: &str, root: &Path, out: &mut Vec<Candidate>) {
    let mut profiles = Vec::new();
    if root.join("cookies.sqlite").exists() || root.join("Network/Cookies").exists() {
        profiles.push(root.to_path_buf());
    }
    for name in ["Default", "Guest Profile", "System Profile"] {
        let path = root.join(name);
        if path.is_dir() {
            profiles.push(path);
        }
    }
    if let Ok(entries) = std::fs::read_dir(root) {
        for entry in entries.flatten() {
            let path = entry.path();
            if !path.is_dir() {
                continue;
            }
            let name = entry.file_name().to_string_lossy().into_owned();
            if name.starts_with("Profile ")
                || name.ends_with(".default")
                || name.contains(".default-")
            {
                profiles.push(path.clone());
            }
            if name == "Profiles"
                && let Ok(sub) = std::fs::read_dir(path)
            {
                for sub_entry in sub.flatten() {
                    if sub_entry.path().is_dir() {
                        profiles.push(sub_entry.path());
                    }
                }
            }
        }
    }
    profiles.sort();
    profiles.dedup();
    for profile in profiles {
        if let Some(modified) = cookie_modified(&profile) {
            out.push(Candidate {
                browser: browser.to_owned(),
                profile,
                modified,
            });
        }
    }
}

fn cookie_modified(profile: &Path) -> Option<SystemTime> {
    for candidate in [
        profile.join("Network/Cookies"),
        profile.join("Cookies"),
        profile.join("cookies.sqlite"),
    ] {
        if let Ok(metadata) = std::fs::metadata(&candidate)
            && metadata.is_file()
        {
            return Some(metadata.modified().unwrap_or(SystemTime::UNIX_EPOCH));
        }
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn missing_home_reports_not_found() {
        let info = detect_browser_cookies(Path::new("/definitely/not/a/home"));
        assert_eq!(info.status, STATUS_NOT_FOUND);
    }
}
