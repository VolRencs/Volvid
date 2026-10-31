const MAX_SANITIZED_FILENAME_LEN: usize = 180;

const WINDOWS_RESERVED: &[&str] = &[
    "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8",
    "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
];

fn sanitize(name: &str, fallback: &str, trim_dots: bool, check_reserved: bool) -> String {
    let mut name: String = name
        .trim()
        .chars()
        .map(|ch| match ch {
            '<' | '>' | ':' | '"' | '/' | '\\' | '|' | '?' | '*' => '_',
            _ => ch,
        })
        .collect();

    if trim_dots {
        name = name.trim_end_matches([' ', '.']).to_owned();
        if name.chars().count() > MAX_SANITIZED_FILENAME_LEN {
            name = name.chars().take(MAX_SANITIZED_FILENAME_LEN).collect();
        }
    }
    if name.is_empty() || name == "." {
        return fallback.to_owned();
    }
    if check_reserved {
        let base = name.split('.').next().unwrap_or_default();
        if WINDOWS_RESERVED
            .iter()
            .any(|reserved| base.eq_ignore_ascii_case(reserved))
        {
            name.insert(0, '_');
        }
    }
    name
}

/// Makes a playlist title safe as a directory name.
pub fn sanitize_dirname(name: &str) -> String {
    sanitize(name, "playlist", true, true)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn replaces_invalid_chars() {
        assert_eq!(sanitize_dirname("a/b:c?*"), "a_b_c__");
    }

    #[test]
    fn resets_reserved_names() {
        assert_eq!(sanitize_dirname("CON"), "_CON");
        assert_eq!(sanitize_dirname("   "), "playlist");
    }
}
