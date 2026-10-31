use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, Instant};

use arboard::Clipboard;
use eframe::egui;
use egui::{Align, Align2, Color32, Key, Layout, Modifiers, Stroke};
use tracing_appender::non_blocking::WorkerGuard;
use volvid_app::{Controller, Route, ToastKind};
use volvid_engine::Theme as ConfigTheme;

use crate::pages;
use crate::theme::{self, Palette};

const SIDEBAR_WIDTH: f32 = 228.0;

/// A queue preview texture plus the identity of the image it was built from.
struct JobPreview {
    source: u64,
    texture: egui::TextureHandle,
}

pub struct VolvidApp {
    pub controller: Controller,
    clipboard: Option<Clipboard>,
    _log_guard: Option<WorkerGuard>,
    last_config_theme: ConfigTheme,
    current_theme: egui::Theme,
    started: Instant,
    preview_texture: Option<egui::TextureHandle>,
    preview_generation: u64,
    job_previews: HashMap<u64, JobPreview>,
    last_route: Route,
    pub(crate) scroll_reset: bool,
    dragging_url: bool,
    session_nonce: u64,
    startup_focus_cleared: bool,
    repaint_installed: bool,
}

impl VolvidApp {
    pub fn new(controller: Controller, log_guard: Option<WorkerGuard>) -> Self {
        let config_theme = controller.state.theme();
        let current_theme = match config_theme {
            ConfigTheme::Light => egui::Theme::Light,
            _ => egui::Theme::Dark,
        };
        let clipboard = Clipboard::new().ok();
        Self {
            controller,
            clipboard,
            _log_guard: log_guard,
            last_config_theme: config_theme,
            current_theme,
            started: Instant::now(),
            preview_texture: None,
            preview_generation: 0,
            job_previews: HashMap::new(),
            last_route: Route::Home,
            scroll_reset: true,
            dragging_url: false,
            session_nonce: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .map(|duration| duration.as_nanos() as u64)
                .unwrap_or(1),
            startup_focus_cleared: false,
            repaint_installed: false,
        }
    }

    pub(crate) fn session_nonce(&self) -> u64 {
        self.session_nonce
    }

    fn update_preview(&mut self, ctx: &egui::Context) {
        let state = &self.controller.state;
        if self.preview_generation == state.preview_generation {
            return;
        }
        self.preview_generation = state.preview_generation;
        let Some(image) = &state.preview_image else {
            self.preview_texture = None;
            return;
        };
        let color = egui::ColorImage::from_rgba_unmultiplied(
            [image.width as usize, image.height as usize],
            &image.rgba,
        );
        if let Some(texture) = &mut self.preview_texture {
            texture.set(color, egui::TextureOptions::LINEAR);
        } else {
            self.preview_texture =
                Some(ctx.load_texture("volvid-preview", color, egui::TextureOptions::LINEAR));
        }
        tracing::debug!(
            width = image.width,
            height = image.height,
            "preview texture updated"
        );
    }

    /// Uploads queue tile previews once per fetched image and drops stale ones.
    fn update_job_previews(&mut self, ctx: &egui::Context) {
        let active: Vec<u64> = self
            .controller
            .state
            .jobs
            .iter()
            .map(|job| job.id)
            .collect();
        self.job_previews.retain(|id, _| active.contains(id));
        for job in &self.controller.state.jobs {
            let Some(image) = job.thumbnail.as_ref() else {
                continue;
            };
            let source = Arc::as_ptr(image) as *const () as u64;
            if self
                .job_previews
                .get(&job.id)
                .is_some_and(|preview| preview.source == source)
            {
                continue;
            }
            let color = egui::ColorImage::from_rgba_unmultiplied(
                [image.width as usize, image.height as usize],
                &image.rgba,
            );
            let texture = ctx.load_texture(
                format!("volvid-job-preview-{}", job.id),
                color,
                egui::TextureOptions::LINEAR,
            );
            self.job_previews
                .insert(job.id, JobPreview { source, texture });
            tracing::debug!(
                job = job.id,
                width = image.width,
                height = image.height,
                "job preview ready"
            );
        }
    }

    pub fn preview_texture(&self) -> Option<&egui::TextureHandle> {
        self.preview_texture.as_ref()
    }

    /// Uploaded preview texture for a queue job's tile, if available.
    pub fn job_preview(&self, job_id: u64) -> Option<&egui::TextureHandle> {
        self.job_previews
            .get(&job_id)
            .map(|preview| &preview.texture)
    }

    pub fn s(&self) -> &'static volvid_i18n::Strings {
        self.controller.state.s()
    }

    pub fn palette(&self) -> &'static Palette {
        theme::palette(self.current_theme)
    }

    pub fn is_light(&self) -> bool {
        self.current_theme == egui::Theme::Light
    }

    pub fn paste_clipboard(&mut self) {
        let text = self
            .clipboard
            .as_mut()
            .and_then(|clipboard| clipboard.get_text().ok())
            .unwrap_or_default();
        if text.trim().is_empty() {
            self.controller
                .state
                .toast(self.s().clipboard_empty, ToastKind::Warn);
            return;
        }
        self.controller.state.url = text.trim().to_owned();
        self.controller.state.url_error = None;
    }

    fn handle_shortcuts(&mut self, ctx: &egui::Context) {
        let text_focused = ctx.text_edit_focused();
        let modal_open = self.controller.state.search_modal
            || self.controller.state.update_modal
            || self.controller.state.mixed_choice.is_some();
        let (paste, search, folder, deps, settings, escape) = ctx.input_mut(|input| {
            (
                !text_focused && !modal_open && input.consume_key(Modifiers::CTRL, Key::V),
                input.consume_key(Modifiers::CTRL, Key::G),
                input.consume_key(Modifiers::CTRL, Key::O),
                input.consume_key(Modifiers::CTRL, Key::U),
                input.consume_key(Modifiers::CTRL, Key::Comma),
                !text_focused && !modal_open && input.consume_key(Modifiers::NONE, Key::Escape),
            )
        });
        if escape && self.controller.state.route != Route::Home {
            self.controller.set_route(Route::Home);
        }
        if paste {
            if self.controller.state.route != Route::Home {
                self.controller.set_route(Route::Home);
            }
            self.paste_clipboard();
        }
        if search {
            self.controller.state.search_modal = true;
            self.controller.state.search_error = None;
        }
        if folder {
            self.controller.pick_folder();
        }
        if deps || settings {
            self.controller.set_route(Route::Settings);
        }
    }

    fn handle_drag_and_drop(&mut self, ctx: &egui::Context) {
        let (hovered, dropped) = ctx.input(|input| {
            (
                !input.raw.hovered_files.is_empty(),
                input.raw.dropped_files.clone(),
            )
        });
        self.dragging_url = hovered;
        if dropped.is_empty() {
            return;
        }
        for file in dropped {
            if let Some(text) = dropped_file_text(&file)
                && let Some(url) = extract_url(&text)
            {
                self.controller.state.url = url;
                self.controller.state.url_error = None;
                self.controller.set_route(Route::Home);
                self.controller.analyze_url();
                return;
            }
        }
        self.controller
            .state
            .toast(self.s().unsupported_url, ToastKind::Warn);
    }

    fn drag_overlay(&self, ctx: &egui::Context) {
        if !self.dragging_url {
            return;
        }
        let p = self.palette();
        let rect = ctx.content_rect().shrink(28.0);
        let painter = ctx.layer_painter(egui::LayerId::new(
            egui::Order::Foreground,
            egui::Id::unique("volvid-drop-overlay"),
        ));
        painter.rect_filled(
            ctx.content_rect(),
            0.0,
            Color32::from_black_alpha(if self.is_light() { 90 } else { 150 }),
        );
        painter.rect_filled(rect, egui::CornerRadius::same(20), p.card);
        painter.rect_stroke(
            rect,
            egui::CornerRadius::same(20),
            Stroke::new(2.0, p.accent),
            egui::StrokeKind::Inside,
        );
        let galley = painter.layout(
            self.s().drag_hint.to_owned(),
            theme::semibold(19.0),
            p.text,
            (rect.width() - 48.0).max(1.0),
        );
        painter.galley(rect.center() - galley.size() / 2.0, galley, p.text);
    }

    fn top_controls(&mut self, ui: &mut egui::Ui) {
        let p = self.palette();
        let s = self.s();
        let width = ui.available_width();
        ui.allocate_ui_with_layout(
            egui::vec2(width, 44.0),
            Layout::right_to_left(Align::Center),
            |ui| {
                if let Some(update) = self.controller.state.update.clone()
                    && ui
                        .add(
                            egui::Button::new(
                                theme::strong_text(format!("↑ {}", update.latest), 12.0)
                                    .color(p.warning),
                            )
                            .fill(p.warning.gamma_multiply(0.12))
                            .stroke(Stroke::new(1.0, p.warning.gamma_multiply(0.35)))
                            .corner_radius(egui::CornerRadius::same(255))
                            .min_size(egui::vec2(0.0, 34.0)),
                        )
                        .on_hover_text(s.update_available)
                        .clicked()
                {
                    self.controller.state.update_modal = true;
                }
            },
        );
        ui.add_space(8.0);
    }

    fn sidebar(&mut self, ui: &mut egui::Ui) {
        let p = self.palette();
        let s = self.s();
        let route = self.controller.state.route;
        let pending_jobs = self.controller.state.pending_jobs();
        ui.add_space(10.0);
        let logo_source = if self.is_light() {
            egui::include_image!("../../../assets/icons/volvid-logo-light.svg")
        } else {
            egui::include_image!("../../../assets/icons/volvid-logo.svg")
        };
        ui.horizontal(|ui| {
            ui.add_space(8.0);
            ui.add(egui::Image::new(logo_source).fit_to_exact_size(egui::vec2(150.0, 31.6)));
        });
        ui.add_space(3.0);
        ui.horizontal(|ui| {
            ui.add_space(8.0);
            ui.label(
                egui::RichText::new(format!("v{}", self.controller.app_version()))
                    .size(11.5)
                    .color(p.muted),
            );
        });
        ui.add_space(24.0);

        nav_item(
            ui,
            p,
            route == Route::Home,
            egui::include_image!("../../../assets/icons/new-download.svg"),
            s.nav_new,
            None,
            || self.controller.set_route(Route::Home),
        );
        nav_item(
            ui,
            p,
            route == Route::Downloads,
            egui::include_image!("../../../assets/icons/queue.svg"),
            s.nav_queue,
            (pending_jobs > 0).then_some(pending_jobs),
            || self.controller.set_route(Route::Downloads),
        );
        nav_item(
            ui,
            p,
            route == Route::Summary,
            egui::include_image!("../../../assets/icons/history.svg"),
            s.nav_history,
            None,
            || self.controller.set_route(Route::Summary),
        );
        nav_item(
            ui,
            p,
            route == Route::Settings,
            egui::include_image!("../../../assets/icons/settings.svg"),
            s.nav_settings,
            None,
            || self.controller.set_route(Route::Settings),
        );
    }

    fn central(&mut self, ui: &mut egui::Ui) {
        let route = self.controller.state.route;
        self.top_controls(ui);
        let mut scroll = egui::ScrollArea::vertical()
            .id_salt(format!("volvid-page-{route:?}"))
            .auto_shrink([false, false]);
        let reset_page = self.scroll_reset
            || (route == Route::Playlist && self.controller.state.playlist_scroll_reset);
        if reset_page {
            scroll = scroll.vertical_scroll_offset(0.0);
        }
        scroll.show(ui, |ui| {
            ui.set_min_width(ui.available_width());
            match route {
                Route::Home => pages::home::show(self, ui),
                Route::Playlist => pages::playlist::show(self, ui),
                Route::Configure => pages::configure::show(self, ui),
                Route::Downloads => pages::downloads::show(self, ui),
                Route::Summary => pages::summary::show(self, ui),
                Route::Settings => pages::settings::show(self, ui),
            }
            ui.add_space(8.0);
        });
        self.scroll_reset = false;
    }

    fn modals(&mut self, ctx: &egui::Context) {
        if self.controller.state.search_modal {
            pages::search::show(self, ctx);
        }
        if self.controller.state.update_modal {
            pages::update::show(self, ctx);
        }
        if self.controller.state.mixed_choice.is_some() {
            pages::mixed::show(self, ctx);
        }
    }

    fn toast(&mut self, ctx: &egui::Context) {
        let Some(toast) = self.controller.state.toast.clone() else {
            return;
        };
        if toast.created.elapsed() > Duration::from_secs(7) {
            self.controller.state.toast = None;
            return;
        }
        let p = self.palette();
        let color = match toast.kind {
            ToastKind::Info => p.accent,
            ToastKind::Success => p.success,
            ToastKind::Warn => p.warning,
            ToastKind::Error => p.danger,
        };
        egui::Area::new(egui::Id::unique("volvid-toast"))
            .anchor(Align2::RIGHT_BOTTOM, [-22.0, -22.0])
            .order(egui::Order::Foreground)
            .show(ctx, |ui| {
                egui::Frame::new()
                    .fill(p.card)
                    .stroke(Stroke::new(1.0, color.gamma_multiply(0.5)))
                    .corner_radius(egui::CornerRadius::same(12))
                    .inner_margin(egui::Margin::same(14))
                    .shadow(egui::epaint::Shadow {
                        offset: [0, 10],
                        blur: 28,
                        spread: 0,
                        color: Color32::from_black_alpha(90),
                    })
                    .show(ui, |ui| {
                        ui.set_max_width(440.0);
                        ui.horizontal(|ui| {
                            theme::status_dot(ui, color);
                            ui.allocate_ui_with_layout(
                                egui::vec2(360.0, 0.0),
                                Layout::top_down(Align::LEFT),
                                |ui| {
                                    ui.add(
                                        egui::Label::new(
                                            egui::RichText::new(&toast.text).color(p.text),
                                        )
                                        .truncate(),
                                    );
                                },
                            );
                            if ui
                                .add(
                                    egui::Button::new(egui::RichText::new("×").color(p.muted))
                                        .fill(Color32::TRANSPARENT)
                                        .stroke(Stroke::NONE),
                                )
                                .clicked()
                            {
                                self.controller.state.toast = None;
                            }
                        });
                    });
            });
        ctx.request_repaint_after(Duration::from_millis(250));
    }
}

impl eframe::App for VolvidApp {
    fn logic(&mut self, ctx: &egui::Context, _frame: &mut eframe::Frame) {
        if !self.startup_focus_cleared {
            if let Some(focused) = ctx.memory(|memory| memory.focused()) {
                ctx.memory_mut(|memory| memory.surrender_focus(focused));
            }
            ctx.memory_mut(|memory| memory.stop_text_input());
            self.startup_focus_cleared = true;
        }
        if !self.repaint_installed {
            let repaint_ctx = ctx.clone();
            self.controller
                .set_repaint(Arc::new(move || repaint_ctx.request_repaint()));
            self.repaint_installed = true;
        }
        self.controller.poll();
        self.update_preview(ctx);
        self.update_job_previews(ctx);
        self.handle_drag_and_drop(ctx);
        if self.controller.state.route != self.last_route {
            self.last_route = self.controller.state.route;
            self.scroll_reset = true;
            ctx.request_repaint();
        }
        let config_theme = self.controller.state.theme();
        if config_theme != self.last_config_theme {
            theme::apply_theme(ctx, config_theme);
            self.last_config_theme = config_theme;
        }
        let effective_theme = theme::resolve_theme(ctx, config_theme);
        if effective_theme != self.current_theme {
            self.current_theme = effective_theme;
            ctx.request_repaint();
        }
        if let Some(toast) = &self.controller.state.toast
            && toast.created.elapsed() > Duration::from_secs(7)
        {
            self.controller.state.toast = None;
        }
        let state = &self.controller.state;
        let busy = state.deps_checking
            || state.playlist_loading
            || state.quality_loading
            || state.probe_loading
            || state.thumbnail_loading
            || state.search_loading
            || state.active_jobs() > 0
            || state.installing_dep.is_some()
            || state.updating
            || state.update_checking
            || state.toast.is_some();
        if busy {
            ctx.request_repaint_after(Duration::from_millis(100));
        }
        if self.started.elapsed() < Duration::from_secs(2) {
            ctx.request_repaint_after(Duration::from_millis(100));
        }
    }

    fn ui(&mut self, ui: &mut egui::Ui, _frame: &mut eframe::Frame) {
        let ctx = ui.ctx().clone();
        self.handle_shortcuts(&ctx);
        let p = self.palette();
        ui.painter().rect_filled(ui.max_rect(), 0.0, p.bg);

        egui::Panel::left("volvid-nav")
            .resizable(false)
            .exact_size(SIDEBAR_WIDTH)
            .frame(
                egui::Frame::new()
                    .fill(p.panel)
                    .inner_margin(egui::Margin::symmetric(14, 10)),
            )
            .show(ui, |ui| self.sidebar(ui));

        egui::CentralPanel::default()
            .frame(
                egui::Frame::new()
                    .fill(p.bg)
                    .inner_margin(egui::Margin::symmetric(
                        theme::PAGE_MARGIN_X,
                        theme::PAGE_MARGIN_Y,
                    )),
            )
            .show(ui, |ui| self.central(ui));

        self.drag_overlay(&ctx);
        self.modals(&ctx);
        self.toast(&ctx);
    }

    fn on_exit(&mut self) {
        self.controller.backend.shutdown();
    }
}

fn nav_item(
    ui: &mut egui::Ui,
    p: &Palette,
    active: bool,
    icon: egui::ImageSource<'static>,
    label: &str,
    badge: Option<usize>,
    mut action: impl FnMut(),
) {
    let fill = if active {
        p.accent
    } else {
        Color32::TRANSPARENT
    };
    let text_color = if active { p.on_accent } else { p.muted };
    let image = egui::Image::new(icon)
        .fit_to_exact_size(egui::vec2(18.0, 18.0))
        .tint(text_color);
    let response = ui.add(
        egui::Button::new((image, theme::strong_text(label, 14.5).color(text_color)))
            .image_tint_follows_text_color(true)
            .fill(fill)
            .stroke(Stroke::NONE)
            .corner_radius(egui::CornerRadius::same(11))
            .min_size(egui::vec2(ui.available_width(), 44.0))
            .truncate(),
    );
    theme::press_overlay(ui, &response, 11);
    if response.clicked() {
        action();
    }
    if let Some(badge) = badge {
        let rect = response.rect;
        let color = if active { p.on_accent } else { p.accent };
        let text_color = if active { p.accent } else { Color32::WHITE };
        let text = if badge > 99 {
            "99+".to_owned()
        } else {
            badge.to_string()
        };
        let radius = if badge > 9 { 12.0 } else { 10.0 };
        ui.painter().circle_filled(
            egui::pos2(rect.right() - 18.0, rect.center().y),
            radius,
            color,
        );
        ui.painter().text(
            egui::pos2(rect.right() - 18.0, rect.center().y),
            Align2::CENTER_CENTER,
            text,
            theme::semibold(11.0),
            text_color,
        );
    }
}

fn dropped_file_text(file: &egui::DroppedFileHandle) -> Option<String> {
    let path = file.path();
    if let Ok(metadata) = std::fs::metadata(path)
        && metadata.is_file()
        && metadata.len() <= 1 << 20
        && let Ok(text) = std::fs::read_to_string(path)
    {
        return Some(text);
    }
    file.bytes().ok().and_then(|bytes| {
        (bytes.len() <= 1 << 20)
            .then(|| String::from_utf8(bytes).ok())
            .flatten()
    })
}

fn extract_url(text: &str) -> Option<String> {
    text.lines()
        .map(str::trim)
        .filter(|line| !line.is_empty() && !line.starts_with('#'))
        .find_map(|line| {
            let candidate = line.strip_prefix("URL:").unwrap_or(line).trim();
            volvid_core::parse_target(candidate)
                .ok()
                .map(|_| candidate.to_owned())
        })
}

#[cfg(test)]
mod tests {
    use super::extract_url;

    #[test]
    fn extracts_url_from_uri_list() {
        assert_eq!(
            extract_url("# comment\r\nURL:https://youtu.be/abc\r\n").as_deref(),
            Some("https://youtu.be/abc")
        );
    }
}
