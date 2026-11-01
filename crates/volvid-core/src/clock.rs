use thiserror::Error;

/// Formats seconds as `mm:ss` or `hh:mm:ss`.
pub fn format_clock(total_seconds: i64) -> String {
    let total = total_seconds.max(0);
    let hours = total / 3600;
    let minutes = (total % 3600) / 60;
    let seconds = total % 60;
    if hours > 0 {
        format!("{hours:02}:{minutes:02}:{seconds:02}")
    } else {
        format!("{minutes:02}:{seconds:02}")
    }
}

/// Formats seconds as `m:ss` / `h:mm:ss`; `??:??` when unknown.
pub fn format_duration(secs: i64) -> String {
    if secs <= 0 {
        return "??:??".to_owned();
    }
    let h = secs / 3600;
    let m = (secs % 3600) / 60;
    let s = secs % 60;
    if h > 0 {
        format!("{h}:{m:02}:{s:02}")
    } else {
        format!("{m}:{s:02}")
    }
}

fn parse_digits(value: &str) -> Option<i64> {
    if value.is_empty() || !value.chars().all(|c| c.is_ascii_digit()) {
        return None;
    }
    value.parse().ok()
}

/// Parses strict `mm:ss` or `hh:mm:ss`.
pub fn parse_clock(raw: &str) -> Result<i64, ClockError> {
    let value = raw.trim();
    if value.is_empty() {
        return Err(ClockError::Empty);
    }
    let parts: Vec<&str> = value.split(':').collect();
    if parts.len() != 2 && parts.len() != 3 {
        return Err(ClockError::Format);
    }
    let numbers: Vec<i64> = parts
        .iter()
        .map(|part| parse_digits(part).ok_or(ClockError::Token))
        .collect::<Result<_, _>>()?;

    let (hours, minutes, seconds) = if numbers.len() == 2 {
        (0, numbers[0], numbers[1])
    } else {
        (numbers[0], numbers[1], numbers[2])
    };

    if seconds >= 60 {
        return Err(ClockError::Seconds);
    }
    if hours > 0 && minutes >= 60 {
        return Err(ClockError::Minutes);
    }
    let total = hours
        .checked_mul(3600)
        .and_then(|v| minutes.checked_mul(60).and_then(|m| v.checked_add(m)))
        .and_then(|v| v.checked_add(seconds))
        .ok_or(ClockError::TooLarge)?;
    Ok(total)
}

/// Parses timestamps in forms accepted by YouTube links: `90`, `1:30`,
/// `1h30m`, `2m10s`, `1h2m3s`.
pub fn parse_flexible_timestamp(raw: &str) -> Option<i64> {
    let value = raw.trim().to_ascii_lowercase();
    if value.is_empty() {
        return None;
    }
    if value.contains(':') {
        return parse_clock(&value).ok();
    }
    if let Some(seconds) = parse_digits(&value) {
        return Some(seconds);
    }

    let mut total = 0i64;
    let mut current = String::new();
    let mut consumed = false;
    for ch in value.chars() {
        if ch.is_ascii_digit() {
            current.push(ch);
            continue;
        }
        let multiplier = match ch {
            'h' => 3600,
            'm' => 60,
            's' => 1,
            _ => return None,
        };
        let number = parse_digits(&current)?;
        total = total.checked_add(number.checked_mul(multiplier)?)?;
        current.clear();
        consumed = true;
    }
    if !current.is_empty() {
        if consumed {
            return None;
        }
        return parse_digits(&current);
    }
    consumed.then_some(total)
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Error)]
pub enum ClockError {
    #[error("empty timestamp")]
    Empty,
    #[error("timestamp must be mm:ss or hh:mm:ss")]
    Format,
    #[error("invalid timestamp token")]
    Token,
    #[error("seconds must be < 60")]
    Seconds,
    #[error("minutes and seconds must be < 60")]
    Minutes,
    #[error("timestamp is too large")]
    TooLarge,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn formats_clock() {
        assert_eq!(format_clock(0), "00:00");
        assert_eq!(format_clock(65), "01:05");
        assert_eq!(format_clock(3661), "01:01:01");
    }

    #[test]
    fn parses_clock() {
        assert_eq!(parse_clock("1:30").unwrap(), 90);
        assert_eq!(parse_clock("01:02:03").unwrap(), 3723);
        assert!(parse_clock("1:60").is_err());
    }

    #[test]
    fn parses_flexible() {
        assert_eq!(parse_flexible_timestamp("90"), Some(90));
        assert_eq!(parse_flexible_timestamp("1h30m"), Some(5400));
        assert_eq!(parse_flexible_timestamp("1:30"), Some(90));
        assert_eq!(parse_flexible_timestamp("nope"), None);
    }
}
