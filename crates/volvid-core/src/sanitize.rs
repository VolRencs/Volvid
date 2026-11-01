const MAX_SANITIZED_FILENAME_LEN: usize = 180;

const WINDOWS_RESERVED: &[&str] = &[
    "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8",
    "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
];

fn sanitize(name: &str, fallback: &str, trim_dots: bool, check_reserved: bool) -> String {
    let mut name: String = name
        .trim()
        .chars()
        .filter(|ch| !ch.is_control())
        .map(|ch| match ch {
            '<' | '>' | ':' | '"' | '/' | '\\' | '|' | '?' | '*' => '_',
            _ => ch,
        })
        .collect();

    if trim_dots {
        name = name.trim_end_matches([' ', '.']).to_owned();
        if name.len() > MAX_SANITIZED_FILENAME_LEN {
            name = truncate_utf8(&name, MAX_SANITIZED_FILENAME_LEN)
                .trim_end_matches([' ', '.'])
                .to_owned();
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

/// Truncates `text` to at most `max_bytes`, never splitting a UTF-8 character.
pub fn truncate_utf8(text: &str, max_bytes: usize) -> &str {
    if text.len() <= max_bytes {
        return text;
    }
    let mut end = max_bytes;
    while end > 0 && !text.is_char_boundary(end) {
        end -= 1;
    }
    &text[..end]
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

    #[test]
    fn limits_utf8_bytes_and_control_chars() {
        let name = "Ж".repeat(200);
        let sanitized = sanitize_dirname(&name);
        assert!(sanitized.len() <= 180);
        assert!(sanitized.is_char_boundary(sanitized.len()));
        assert_eq!(sanitize_dirname("a\nb\tc"), "abc");
    }

    #[test]
    fn truncates_utf8_on_char_boundary() {
        let text = "ошибка";
        for limit in 0..=text.len() {
            let truncated = truncate_utf8(text, limit);
            assert!(truncated.len() <= limit);
            assert!(text.starts_with(truncated));
        }
    }
}
