use volvid_core::{DownloadMode, DownloadRequest, parse_target};
use volvid_engine::Engine;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let url = std::env::args()
        .nth(1)
        .unwrap_or_else(|| "https://www.youtube.com/watch?v=dQw4w9WgXcQ".to_owned());
    let engine = Engine::new()?;
    let deps = engine.runtime_deps().await;
    println!(
        "ytrust: {} {:?}\nffmpeg: {} {:?}\nnode: {} {:?}\ncookies: {:?}",
        deps.ytrust.version,
        deps.ytrust.path,
        deps.ffmpeg.version,
        deps.ffmpeg.path,
        deps.node.version,
        deps.node.path,
        deps.cookies
    );
    let probe = engine.probe_media(&url, None).await?;
    println!(
        "probe: title={:?} duration={} formats={} audio={} subs={} thumbnail={:?}",
        probe.title,
        probe.duration,
        probe.formats.len(),
        probe.audio_tracks.len(),
        probe.subtitles.len(),
        probe.thumbnail
    );
    let choices = engine
        .resolve_quality(std::slice::from_ref(&url), None)
        .await?;
    if !probe.thumbnail.is_empty() {
        match engine.fetch_bytes(&probe.thumbnail).await {
            Ok(bytes) => println!("thumbnail bytes: {} ({})", bytes.len(), probe.thumbnail),
            Err(err) => println!("thumbnail error: {err} ({})", probe.thumbnail),
        }
    }
    for choice in &choices {
        println!(
            "quality: {}p size={} chain={:?}",
            choice.height, choice.size_bytes, choice.fmt_chain
        );
    }

    if std::env::args().any(|arg| arg == "--playlist") {
        let playlist = engine
            .fetch_playlist(&url, volvid_core::Locale::En, None)
            .await?;
        println!(
            "playlist: title={:?} entries={}",
            playlist.title,
            playlist.entries.len()
        );
        for entry in playlist.entries.iter().take(5) {
            println!("  {} {} {}", entry.index, entry.title, entry.url);
        }
        return Ok(());
    }

    if std::env::args().any(|arg| arg == "--video") {
        let target = parse_target(&url)?;
        let mut profile = volvid_core::OutputProfile {
            label: "144p test".to_owned(),
            mode: DownloadMode::Video,
            video_fmt_chain: vec!["bestvideo[height<=144]+bestaudio/best[height<=144]".to_owned()],
            video_container: "mp4".to_owned(),
            ..volvid_core::OutputProfile::default()
        };
        if std::env::args().any(|arg| arg == "--transcode") {
            profile.video_codec = "libx264".to_owned();
            profile.video_crf = "28".to_owned();
            profile.audio_codec = "aac".to_owned();
            profile.audio_bitrate = "128k".to_owned();
        }
        let request = DownloadRequest {
            target,
            profile,
            fragment: Some(volvid_core::DownloadFragment {
                start_at: 0,
                end_at: Some(5),
            }),
            media_duration: probe.duration,
            output_dir: "/tmp/volvid-smoke".to_owned(),
            locale: volvid_core::Locale::En,
            ..DownloadRequest::default()
        };
        let (tx, mut rx) = tokio::sync::mpsc::unbounded_channel();
        let cancel = tokio_util::sync::CancellationToken::new();
        let engine = std::sync::Arc::new(engine);
        let runner = {
            let engine = engine.clone();
            tokio::spawn(async move { engine.download(request, deps, tx, cancel).await })
        };
        while let Some(update) = rx.recv().await {
            println!(
                "event: {:?} slot={} pct={:.1} text={:?} err={:?}",
                update.event_type, update.slot, update.pct, update.text, update.err_text
            );
        }
        println!("download result: {:?}", runner.await?);
        return Ok(());
    }

    if std::env::args().any(|arg| arg == "--download") {
        let target = parse_target(&url)?;
        let request = DownloadRequest {
            target,
            profile: volvid_i18n::thumbnail_profile(volvid_core::Locale::En),
            output_dir: "/tmp/volvid-smoke".to_owned(),
            locale: volvid_core::Locale::En,
            ..DownloadRequest::default()
        };
        let (tx, mut rx) = tokio::sync::mpsc::unbounded_channel();
        let cancel = tokio_util::sync::CancellationToken::new();
        let engine = std::sync::Arc::new(engine);
        let runner = {
            let engine = engine.clone();
            tokio::spawn(async move { engine.download(request, deps, tx, cancel).await })
        };
        while let Some(update) = rx.recv().await {
            println!(
                "event: {:?} slot={} pct={:.1} text={:?} err={:?}",
                update.event_type, update.slot, update.pct, update.text, update.err_text
            );
        }
        println!("download result: {:?}", runner.await?);
    }
    Ok(())
}
