use serde::{Deserialize, Serialize};

/// UI language. Kept in the domain because download requests carry it.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Locale {
    #[default]
    En,
    Ru,
}

impl Locale {
    pub fn code(self) -> &'static str {
        match self {
            Self::En => "en",
            Self::Ru => "ru",
        }
    }

    pub fn is_ru(self) -> bool {
        matches!(self, Self::Ru)
    }
}

impl std::fmt::Display for Locale {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.code())
    }
}

/// Maps free-form input ("ru", "rus", ...) to a locale.
pub fn parse_locale(raw: &str) -> Locale {
    match raw.trim().to_lowercase().as_str() {
        "ru" | "rus" | "russian" => Locale::Ru,
        _ => Locale::En,
    }
}

/// Toggles between the supported UI languages.
pub fn next_locale(locale: Locale) -> Locale {
    match locale {
        Locale::En => Locale::Ru,
        Locale::Ru => Locale::En,
    }
}
