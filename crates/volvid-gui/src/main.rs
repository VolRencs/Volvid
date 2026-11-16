// Hide the console window in release builds on Windows; debug builds keep it
// to make troubleshooting output visible.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod app;
mod format;
mod modals;
mod pages;
mod text_input;
mod theme;

use std::borrow::Cow;

use gpui::{
    App, AppContext, Application, AssetSource, Bounds, SharedString, WindowBounds, WindowOptions,
    px, size,
};
use volvid_app::{Controller, Route};
use volvid_engine::AppPaths;

use crate::app::VolvidApp;

struct Assets;

/// Assets are embedded so release binaries do not depend on the build tree.
macro_rules! asset {
    ($path:literal) => {
        Cow::Borrowed(include_bytes!(concat!("../../../assets/", $path)).as_slice())
    };
}

impl AssetSource for Assets {
    fn load(&self, path: &str) -> gpui::Result<Option<Cow<'static, [u8]>>> {
        let bytes = match path {
            "icons/alert.svg" => asset!("icons/alert.svg"),
            "icons/arrow-left.svg" => asset!("icons/arrow-left.svg"),
            "icons/check.svg" => asset!("icons/check.svg"),
            "icons/close.svg" => asset!("icons/close.svg"),
            "icons/download.svg" => asset!("icons/download.svg"),
            "icons/folder.svg" => asset!("icons/folder.svg"),
            "icons/history.svg" => asset!("icons/history.svg"),
            "icons/info.svg" => asset!("icons/info.svg"),
            "icons/link.svg" => asset!("icons/link.svg"),
            "icons/new-download.svg" => asset!("icons/new-download.svg"),
            "icons/pause.svg" => asset!("icons/pause.svg"),
            "icons/play.svg" => asset!("icons/play.svg"),
            "icons/queue.svg" => asset!("icons/queue.svg"),
            "icons/refresh.svg" => asset!("icons/refresh.svg"),
            "icons/search.svg" => asset!("icons/search.svg"),
            "icons/settings.svg" => asset!("icons/settings.svg"),
            "icons/volvid-logo.svg" => asset!("icons/volvid-logo.svg"),
            "icons/volvid-logo-light.svg" => asset!("icons/volvid-logo-light.svg"),
            "icons/volvid-256.png" => asset!("icons/volvid-256.png"),
            _ => return Ok(None),
        };
        Ok(Some(bytes))
    }

    fn list(&self, path: &str) -> gpui::Result<Vec<SharedString>> {
        if path.trim_matches('/').is_empty() || path.trim_matches('/') == "icons" {
            return Ok([
                "alert.svg",
                "arrow-left.svg",
                "check.svg",
                "close.svg",
                "download.svg",
                "folder.svg",
                "history.svg",
                "info.svg",
                "link.svg",
                "new-download.svg",
                "pause.svg",
                "play.svg",
                "queue.svg",
                "refresh.svg",
                "search.svg",
                "settings.svg",
                "volvid-logo.svg",
                "volvid-logo-light.svg",
                "volvid-256.png",
            ]
            .into_iter()
            .map(SharedString::from)
            .collect());
        }
        Ok(Vec::new())
    }
}

fn update_check(json: bool) {
    let runtime = match tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
    {
        Ok(runtime) => runtime,
        Err(err) => {
            eprintln!("could not start the update checker: {err}");
            std::process::exit(1);
        }
    };
    let engine = match volvid_engine::Engine::new() {
        Ok(engine) => engine,
        Err(err) => {
            eprintln!("could not initialize Volvid: {err}");
            std::process::exit(1);
        }
    };
    match runtime.block_on(engine.check_update()) {
        Ok(Some(info)) => {
            if json {
                println!(
                    "{}",
                    serde_json::json!({
                        "current": env!("CARGO_PKG_VERSION"),
                        "update": {
                            "version": info.latest,
                            "url": info.download_url,
                            "sha256": info.sha256,
                            "size": info.size,
                        }
                    })
                );
            } else {
                println!("current: {}", env!("CARGO_PKG_VERSION"));
                println!("latest: {}", info.latest);
                println!("url: {}", info.download_url);
                if let Some(sha256) = &info.sha256 {
                    println!("sha256: {sha256}");
                }
                if let Some(size) = info.size {
                    println!("size: {size}");
                }
            }
            std::process::exit(0);
        }
        Ok(None) => {
            if json {
                println!(
                    "{}",
                    serde_json::json!({
                        "current": env!("CARGO_PKG_VERSION"),
                        "update": null,
                    })
                );
            } else {
                println!("current: {}", env!("CARGO_PKG_VERSION"));
                println!("latest: none");
            }
            std::process::exit(0);
        }
        Err(err) => {
            eprintln!("update check failed: {err:#}");
            std::process::exit(1);
        }
    }
}

fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    if args.iter().any(|arg| arg == "--version" || arg == "-V") {
        println!("Volvid {}", env!("CARGO_PKG_VERSION"));
        return;
    }
    if args.iter().any(|arg| arg == "--update-check") {
        update_check(args.iter().any(|arg| arg == "--json"));
        return;
    }

    let paths = AppPaths::detect();
    let _ = std::fs::create_dir_all(paths.logs_dir());
    let file_appender = tracing_appender::rolling::RollingFileAppender::builder()
        .rotation(tracing_appender::rolling::Rotation::DAILY)
        .filename_prefix("volvid.log")
        .max_log_files(7)
        .build(paths.logs_dir())
        .unwrap_or_else(|_| tracing_appender::rolling::daily(paths.logs_dir(), "volvid.log"));
    let (non_blocking, guard) = tracing_appender::non_blocking(file_appender);
    let filter = tracing_subscriber::EnvFilter::try_from_default_env()
        .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("volvid=info,warn"));
    tracing_subscriber::fmt()
        .with_env_filter(filter)
        .with_writer(non_blocking)
        .with_ansi(false)
        .init();

    let mut controller = match Controller::new() {
        Ok(controller) => controller,
        Err(err) => {
            eprintln!("Volvid failed to start: {err}");
            std::process::exit(1);
        }
    };
    if let Some(url) = args.iter().find(|arg| !arg.starts_with('-')) {
        controller.state.url = url.clone();
        controller.analyze_url();
    }
    if args.iter().any(|arg| arg == "--settings") {
        controller.set_route(Route::Settings);
    }
    if let Some(page) = args
        .iter()
        .find_map(|arg| arg.strip_prefix("--page=").map(str::to_owned))
    {
        let route = match page.as_str() {
            "queue" => Route::Downloads,
            "history" => Route::Summary,
            "settings" => Route::Settings,
            "configure" if controller.state.target.is_some() => Route::Configure,
            "playlist"
                if controller.state.playlist.is_some() || controller.state.playlist_loading =>
            {
                Route::Playlist
            }
            _ => Route::Home,
        };
        controller.set_route(route);
    }

    Application::new()
        .with_assets(Assets)
        .run(move |cx: &mut App| {
            let _ = cx.text_system().add_fonts(vec![
                Cow::Borrowed(include_bytes!("../../../assets/fonts/Onest-Regular.ttf")),
                Cow::Borrowed(include_bytes!("../../../assets/fonts/Onest-SemiBold.ttf")),
            ]);
            text_input::bind_keys(cx);

            cx.on_window_closed(|cx| {
                if cx.windows().is_empty() {
                    cx.quit();
                }
            })
            .detach();

            let bounds = Bounds::centered(None, size(px(1280.), px(800.)), cx);
            let guard = Some(guard);
            cx.open_window(
                WindowOptions {
                    window_bounds: Some(WindowBounds::Windowed(bounds)),
                    window_min_size: Some(size(px(940.), px(620.))),
                    titlebar: Some(gpui::TitlebarOptions {
                        title: Some("Volvid".into()),
                        appears_transparent: false,
                        traffic_light_position: None,
                    }),
                    app_id: Some("volvid".to_owned()),
                    focus: true,
                    ..Default::default()
                },
                move |window, cx| {
                    let view = cx.new(|cx| VolvidApp::new(controller, guard, cx));
                    let focus = view.read(cx).root_focus.clone();
                    window.focus(&focus);
                    view
                },
            )
            .expect("failed to open Volvid window");
            cx.activate(true);
        });
}
