use jiff::tz::TimeZone;

pub fn timestamp(seconds: i64) -> String {
    jiff::Timestamp::from_second(seconds)
        .ok()
        .map(|timestamp| {
            timestamp
                .to_zoned(TimeZone::system())
                .strftime("%Y-%m-%d %H:%M")
                .to_string()
        })
        .unwrap_or_default()
}
