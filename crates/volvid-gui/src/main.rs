#![allow(clippy::collapsible_if, clippy::too_many_arguments)]

mod app;
mod pages;
mod theme;

use std::sync::Arc;

use eframe::egui;
use volvid_app::{Controller, Route};
use volvid_engine::Theme;

use crate::app::VolvidApp;

fn main() -> eframe::Result {
    let paths = volvid_engine::AppPaths::detect();
    let _ = std::fs::create_dir_all(paths.logs_dir());
    let file_appender = tracing_appender::rolling::daily(paths.logs_dir(), "volvid.log");
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
    if let Some(url) = std::env::args().skip(1).find(|arg| !arg.starts_with('-')) {
        controller.state.url = url;
        controller.analyze_url();
    }
    if std::env::args().any(|arg| arg == "--settings") {
        controller.set_route(Route::Settings);
    }
    if let Some(page) =
        std::env::args().find_map(|arg| arg.strip_prefix("--page=").map(str::to_owned))
    {
        controller.set_route(match page.as_str() {
            "queue" => Route::Downloads,
            "history" => Route::Summary,
            "settings" => Route::Settings,
            "configure" => Route::Configure,
            "playlist" => Route::Playlist,
            _ => Route::Home,
        });
    }
    let app = app::VolvidApp::new(controller, Some(guard));

    let icon = load_icon();
    let mut viewport = egui::ViewportBuilder::default()
        .with_title("Volvid")
        .with_inner_size([1180.0, 760.0])
        .with_min_inner_size([940.0, 620.0])
        .with_app_id("volvid");
    if let Some(icon) = icon {
        viewport = viewport.with_icon(Arc::new(icon));
    }
    if std::env::var_os("VOLVID_TEST_FRONT").is_some() {
        viewport = viewport.with_always_on_top();
    }
    let options = eframe::NativeOptions {
        renderer: eframe::Renderer::Wgpu,
        viewport,
        centered: true,
        ..Default::default()
    };
    eframe::run_native(
        "Volvid",
        options,
        Box::new(move |cc| {
            theme::setup(cc, app.theme());
            Ok(Box::new(app))
        }),
    )
}

fn load_icon() -> Option<egui::IconData> {
    let bytes = include_bytes!("../../../assets/icons/volvid-256.png");
    let image = image::load_from_memory(bytes).ok()?.to_rgba8();
    let (width, height) = image.dimensions();
    Some(egui::IconData {
        rgba: image.into_raw(),
        width,
        height,
    })
}

impl VolvidApp {
    pub fn theme(&self) -> Theme {
        self.controller.state.theme()
    }
}
