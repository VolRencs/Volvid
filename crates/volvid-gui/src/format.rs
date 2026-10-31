use jiff::tz::TimeZone;

pub fn timestamp(seconds: i64, locale: volvid_core::Locale) -> String {
    let pattern = if locale.is_ru() {
        "%d.%m.%Y %H:%M"
    } else {
        "%Y-%m-%d %H:%M"
    };
    jiff::Timestamp::from_second(seconds)
        .ok()
        .map(|timestamp| {
            timestamp
                .to_zoned(TimeZone::system())
                .strftime(pattern)
                .to_string()
        })
        .unwrap_or_default()
}
