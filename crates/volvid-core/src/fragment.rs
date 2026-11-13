use crate::clock::{format_clock, parse_clock};
use thiserror::Error;

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct DownloadFragment {
    pub start_at: i64,
    pub end_at: Option<i64>,
}

impl DownloadFragment {
    pub fn is_valid(&self) -> bool {
        if self.start_at < 0 {
            return false;
        }
        match self.end_at {
            Some(end) => end > self.start_at,
            None => true,
        }
    }
}

/// Renders a fragment as `mm:ss+` or `mm:ss-mm:ss`.
pub fn format_fragment_label(fragment: Option<&DownloadFragment>) -> String {
    let Some(fragment) = fragment else {
        return String::new();
    };
    if !fragment.is_valid() {
        return String::new();
    }
    let start = format_clock(fragment.start_at);
    match fragment.end_at {
        Some(end) => format!("{start}-{}", format_clock(end)),
        None => format!("{start}+"),
    }
}

#[derive(Debug, Error, Clone, Copy, PartialEq, Eq)]
pub enum FragmentError {
    #[error("invalid fragment format")]
    Format,
    #[error("invalid fragment bounds")]
    Bounds,
    #[error("fragment duration is required")]
    DurationRequired,
    #[error("fragment exceeds media duration")]
    OutOfRange,
}

pub fn parse_fragment_range(raw: &str) -> Result<DownloadFragment, FragmentError> {
    let value = raw.trim();
    if value.is_empty() {
        return Err(FragmentError::Format);
    }

    if let Some(rest) = value.strip_suffix('+') {
        let start_at = parse_clock(rest.trim()).map_err(|_| FragmentError::Format)?;
        return Ok(DownloadFragment {
            start_at,
            end_at: None,
        });
    }

    if value.matches('-').count() != 1 {
        return Err(FragmentError::Format);
    }

    let (start_raw, end_raw) = value.split_once('-').ok_or(FragmentError::Format)?;
    let start_at = parse_clock(start_raw.trim()).map_err(|_| FragmentError::Format)?;
    let end_raw = end_raw.trim();
    if end_raw.is_empty() {
        return Ok(DownloadFragment {
            start_at,
            end_at: None,
        });
    }
    let end_at = parse_clock(end_raw).map_err(|_| FragmentError::Format)?;
    if start_at >= end_at {
        return Err(FragmentError::Bounds);
    }
    Ok(DownloadFragment {
        start_at,
        end_at: Some(end_at),
    })
}

pub fn validate_fragment_duration(
    fragment: &DownloadFragment,
    media_duration: i64,
) -> Result<(), FragmentError> {
    if !fragment.is_valid() {
        return Err(FragmentError::Bounds);
    }
    if media_duration <= 0 {
        return Err(FragmentError::DurationRequired);
    }
    if fragment.start_at >= media_duration {
        return Err(FragmentError::OutOfRange);
    }
    if let Some(end) = fragment.end_at
        && end > media_duration
    {
        return Err(FragmentError::OutOfRange);
    }
    Ok(())
}

pub fn parse_bounded_fragment_range(
    raw: &str,
    media_duration: i64,
) -> Result<DownloadFragment, FragmentError> {
    let fragment = parse_fragment_range(raw)?;
    validate_fragment_duration(&fragment, media_duration)?;
    Ok(fragment)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_ranges() {
        assert_eq!(
            parse_fragment_range("1:00-2:30").unwrap(),
            DownloadFragment {
                start_at: 60,
                end_at: Some(150)
            }
        );
        assert_eq!(
            parse_fragment_range("1:00+").unwrap(),
            DownloadFragment {
                start_at: 60,
                end_at: None
            }
        );
        assert_eq!(
            parse_fragment_range("1:00-").unwrap(),
            DownloadFragment {
                start_at: 60,
                end_at: None
            }
        );
    }

    #[test]
    fn validates_bounds() {
        let fragment = parse_fragment_range("1:00-2:30").unwrap();
        assert!(validate_fragment_duration(&fragment, 100).is_err());
        assert!(validate_fragment_duration(&fragment, 200).is_ok());
    }
}
