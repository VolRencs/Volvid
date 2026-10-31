use thiserror::Error;

#[derive(Debug, Error, Clone, PartialEq, Eq)]
pub enum SelectionError {
    #[error("selection cannot be empty")]
    Empty,
    #[error("range {start}-{end} outside 1-{max}")]
    RangeOutOfBounds {
        start: usize,
        end: usize,
        max: usize,
    },
    #[error("number {value} outside 1-{max}")]
    NumberOutOfBounds { value: usize, max: usize },
    #[error("invalid token: {token:?}")]
    InvalidToken { token: String },
    #[error("nothing selected")]
    None,
}

/// Parses playlist selection strings such as `1-5, 8, 12-` or `all`.
pub fn parse_selection(raw: &str, max_index: usize) -> Result<Vec<usize>, SelectionError> {
    let raw = raw.trim().to_lowercase();
    if raw.is_empty() {
        return Err(SelectionError::Empty);
    }
    if max_index == 0 {
        return Err(SelectionError::None);
    }
    if matches!(raw.as_str(), "a" | "а" | "all" | "все" | "всё" | "*") {
        return Ok((1..=max_index).collect());
    }
    let raw = raw
        .replace(" - ", "-")
        .replace(" – ", "–")
        .replace(" -", "-")
        .replace("- ", "-");

    let mut seen = vec![false; max_index + 1];
    for part in raw.split([',', ';', ' ', '\t', '\n']) {
        let part = part.trim();
        if part.is_empty() {
            continue;
        }
        if let Some((a, b)) = part.split_once(['-', '–']) {
            let start: usize = a.trim().parse().map_err(|_| SelectionError::InvalidToken {
                token: part.to_owned(),
            })?;
            let end: usize = if b.trim().is_empty() {
                max_index
            } else {
                b.trim().parse().map_err(|_| SelectionError::InvalidToken {
                    token: part.to_owned(),
                })?
            };
            let (start, end) = (start.min(end), start.max(end));
            if start < 1 || end > max_index {
                return Err(SelectionError::RangeOutOfBounds {
                    start,
                    end,
                    max: max_index,
                });
            }
            seen[start..=end].fill(true);
        } else {
            let value: usize = part.parse().map_err(|_| SelectionError::InvalidToken {
                token: part.to_owned(),
            })?;
            if value < 1 || value > max_index {
                return Err(SelectionError::NumberOutOfBounds {
                    value,
                    max: max_index,
                });
            }
            seen[value] = true;
        }
    }

    let selected: Vec<usize> = seen
        .iter()
        .enumerate()
        .skip(1)
        .filter_map(|(index, selected)| selected.then_some(index))
        .collect();
    if selected.is_empty() {
        Err(SelectionError::None)
    } else {
        Ok(selected)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_ranges_and_numbers() {
        assert_eq!(parse_selection("1-3, 5", 6).unwrap(), vec![1, 2, 3, 5]);
        assert_eq!(parse_selection("all", 3).unwrap(), vec![1, 2, 3]);
        assert_eq!(parse_selection("3-", 4).unwrap(), vec![3, 4]);
    }

    #[test]
    fn rejects_bad_input() {
        assert!(parse_selection("0", 3).is_err());
        assert!(parse_selection("2-9", 3).is_err());
        assert!(parse_selection("x", 3).is_err());
    }

    #[test]
    fn accepts_uppercase_cyrillic_all() {
        assert_eq!(parse_selection("ВСЕ", 3).unwrap(), vec![1, 2, 3]);
        assert_eq!(parse_selection("Всё", 2).unwrap(), vec![1, 2]);
    }

    #[test]
    fn accepts_spaces_around_ranges() {
        assert_eq!(parse_selection("1 - 3", 4).unwrap(), vec![1, 2, 3]);
    }
}
