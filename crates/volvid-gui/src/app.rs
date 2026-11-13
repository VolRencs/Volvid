use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;

use gpui::{
    AnyElement, Context, Entity, FocusHandle, KeyDownEvent, Pixels, Point, Render,
    UniformListScrollHandle, Window, WindowAppearance, div, linear_color_stop, linear_gradient,
    prelude::*, px,
};
use tracing_appender::non_blocking::WorkerGuard;

use crate::pages;
use crate::text_input::TextInput;
use crate::theme::{self, Palette};
use volvid_app::{Controller, Route, ToastKind};

const SIDEBAR_WIDTH: f32 = 216.0;
const TOAST_LIFETIME: Duration = Duration::from_secs(7);
const POLL_INTERVAL: Duration = Duration::from_millis(80);

pub struct VolvidApp {
    pub(crate) controller: Controller,
    _log_guard: Option<WorkerGuard>,
    pub(crate) url_input: Entity<TextInput>,
    pub(crate) search_input: Entity<TextInput>,
    pub(crate) filter_input: Entity<TextInput>,
    pub(crate) range_input: Entity<TextInput>,
    pub(crate) fragment_start_input: Entity<TextInput>,
    pub(crate) fragment_end_input: Entity<TextInput>,
    url_cache: String,
    search_cache: String,
    filter_cache: String,
    range_cache: String,
    fragment_start_cache: String,
    fragment_end_cache: String,
    placeholder_locale: volvid_core::Locale,
    pub(crate) root_focus: FocusHandle,
    preview: Option<Arc<gpui::RenderImage>>,
    preview_generation: u64,
    job_images: HashMap<u64, (u64, Arc<gpui::RenderImage>)>,
    pub(crate) playlist_scroll: UniformListScrollHandle,
    pub(crate) playlist_drag: Option<Point<Pixels>>,
}

impl VolvidApp {
    pub fn new(
        controller: Controller,
        log_guard: Option<WorkerGuard>,
        cx: &mut Context<Self>,
    ) -> Self {
        let root_focus = cx.focus_handle();
        let locale = controller.state.locale;
        let strings = volvid_i18n::strings(locale);
        let initial_appearance = match controller.state.config.theme {
            volvid_engine::Theme::Light => WindowAppearance::Light,
            volvid_engine::Theme::Dark | volvid_engine::Theme::System => WindowAppearance::Dark,
        };
        let palette = theme::palette(controller.state.config.theme, initial_appearance);
        let style = theme::input_style(&palette);
        let url = controller.state.url.clone();
        let search = controller.state.search_query.clone();
        let filter = controller.state.playlist_filter.clone();
        let range = controller.state.range_input.clone();
        let fragment_start = controller.state.draft.custom_start.clone();
        let fragment_end = controller.state.draft.custom_end.clone();
        let url_input =
            cx.new(|cx| TextInput::new(url.clone(), strings.url_placeholder, style, cx));
        let search_input =
            cx.new(|cx| TextInput::new(search.clone(), strings.search_placeholder, style, cx));
        let filter_input =
            cx.new(|cx| TextInput::new(filter.clone(), strings.filter_placeholder, style, cx));
        let range_input =
            cx.new(|cx| TextInput::new(range.clone(), strings.range_placeholder, style, cx));
        let fragment_start_input =
            cx.new(|cx| TextInput::new(fragment_start.clone(), "1:00", style, cx));
        let fragment_end_input =
            cx.new(|cx| TextInput::new(fragment_end.clone(), "2:30", style, cx));

        let app = Self {
            controller,
            _log_guard: log_guard,
            url_input,
            search_input,
            filter_input,
            range_input,
            fragment_start_input,
            fragment_end_input,
            url_cache: url,
            search_cache: search,
            filter_cache: filter,
            range_cache: range,
            fragment_start_cache: fragment_start,
            fragment_end_cache: fragment_end,
            placeholder_locale: locale,
            root_focus: root_focus.clone(),
            preview: None,
            preview_generation: 0,
            job_images: HashMap::new(),
            playlist_scroll: UniformListScrollHandle::new(),
            playlist_drag: None,
        };

        cx.spawn(async move |weak, cx| {
            loop {
                cx.background_executor().timer(POLL_INTERVAL).await;
                if weak
                    .update(cx, |this, cx| {
                        let changed = this.controller.poll();
                        this.sync_all_inputs(cx);
                        let toast_expired = this
                            .controller
                            .state
                            .toast
                            .as_ref()
                            .is_some_and(|toast| toast.created.elapsed() > TOAST_LIFETIME);
                        if toast_expired {
                            this.controller.state.toast = None;
                        }
                        let animating = this.controller.state.restarting
                            || (this.controller.state.restart_pending
                                && this.controller.state.update_modal)
                            || this.controller.state.updating
                            || this.controller.state.update_checking
                            || this.controller.state.installing_dep.is_some()
                            || this.controller.state.playlist_loading
                            || this.controller.state.search_loading
                            || this.controller.state.probe_loading
                            || this.controller.state.quality_loading
                            || this.controller.state.active_jobs() > 0;
                        let restart_due = this.controller.state.restart_pending
                            && this
                                .controller
                                .state
                                .restart_at
                                .is_some_and(|at| std::time::Instant::now() >= at);
                        this.sync_images();
                        if changed || toast_expired || animating || restart_due {
                            cx.notify();
                        }
                    })
                    .is_err()
                {
                    break;
                }
            }
        })
        .detach();

        app
    }

    fn sync_images(&mut self) {
        let state = &self.controller.state;
        if self.preview_generation != state.preview_generation {
            self.preview_generation = state.preview_generation;
            self.preview = state
                .preview_image
                .as_ref()
                .and_then(|image| preview_image(image));
        }

        let active: std::collections::HashSet<u64> = state.jobs.iter().map(|job| job.id).collect();
        self.job_images.retain(|id, _| active.contains(id));
        for job in &state.jobs {
            let Some(image) = job.thumbnail.as_ref() else {
                continue;
            };
            let source = Arc::as_ptr(image) as *const () as u64;
            if self
                .job_images
                .get(&job.id)
                .is_some_and(|(current, _)| *current == source)
            {
                continue;
            }
            if let Some(render) = preview_image(image) {
                self.job_images.insert(job.id, (source, render));
            }
        }
    }

    pub(crate) fn preview(&self) -> Option<&Arc<gpui::RenderImage>> {
        self.preview.as_ref()
    }

    pub(crate) fn job_image(&self, id: u64) -> Option<&Arc<gpui::RenderImage>> {
        self.job_images.get(&id).map(|(_, image)| image)
    }

    fn maybe_restart(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        let state = &mut self.controller.state;
        if !state.restart_pending || state.restarting || state.pending_jobs() > 0 {
            return;
        }
        let now = std::time::Instant::now();
        let due = match state.restart_at {
            Some(due) => due,
            None => {
                let due = now + Duration::from_secs(2);
                state.restart_at = Some(due);
                due
            }
        };
        if now < due {
            return;
        }
        state.restarting = true;
        self.controller.save_before_restart();
        match volvid_engine::update::spawn_restart() {
            Ok(()) => {
                window.remove_window();
            }
            Err(err) => {
                let message = self.s().restart_failed.to_owned();
                self.controller.state.restarting = false;
                self.controller.state.restart_pending = false;
                self.controller.state.restart_at = None;
                self.controller
                    .state
                    .toast(format!("{message}: {err}"), ToastKind::Error);
                cx.notify();
            }
        }
    }

    pub(crate) fn s(&self) -> &'static volvid_i18n::Strings {
        self.controller.state.s()
    }

    pub(crate) fn palette(&self, appearance: WindowAppearance) -> Palette {
        theme::palette(self.controller.state.config.theme, appearance)
    }

    pub(crate) fn open_search(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        self.controller.state.search_modal = true;
        self.controller.state.search_error = None;
        let handle = self.search_input.read(cx).handle();
        window.focus(&handle);
        cx.notify();
    }

    pub(crate) fn url_field(
        &mut self,
        _window: &Window,
        cx: &mut Context<Self>,
        p: &Palette,
    ) -> AnyElement {
        self.sync_url(cx);
        self.url_input
            .update(cx, |input, cx| input.set_style(theme::input_style(p), cx));
        self.url_input.clone().into_any_element()
    }

    pub(crate) fn search_field(
        &mut self,
        _window: &Window,
        cx: &mut Context<Self>,
        p: &Palette,
    ) -> AnyElement {
        self.sync_search(cx);
        self.search_input
            .update(cx, |input, cx| input.set_style(theme::input_style(p), cx));
        self.search_input.clone().into_any_element()
    }

    pub(crate) fn filter_field(
        &mut self,
        _window: &Window,
        cx: &mut Context<Self>,
        p: &Palette,
    ) -> AnyElement {
        self.sync_filter(cx);
        self.filter_input
            .update(cx, |input, cx| input.set_style(theme::input_style(p), cx));
        self.filter_input.clone().into_any_element()
    }

    pub(crate) fn range_field(
        &mut self,
        _window: &Window,
        cx: &mut Context<Self>,
        p: &Palette,
    ) -> AnyElement {
        self.sync_range(cx);
        self.range_input
            .update(cx, |input, cx| input.set_style(theme::input_style(p), cx));
        self.range_input.clone().into_any_element()
    }

    pub(crate) fn fragment_start_field(
        &mut self,
        _window: &Window,
        cx: &mut Context<Self>,
        p: &Palette,
    ) -> AnyElement {
        self.sync_fragment_start(cx);
        self.fragment_start_input
            .update(cx, |input, cx| input.set_style(theme::input_style(p), cx));
        self.fragment_start_input.clone().into_any_element()
    }

    pub(crate) fn fragment_end_field(
        &mut self,
        _window: &Window,
        cx: &mut Context<Self>,
        p: &Palette,
    ) -> AnyElement {
        self.sync_fragment_end(cx);
        self.fragment_end_input
            .update(cx, |input, cx| input.set_style(theme::input_style(p), cx));
        self.fragment_end_input.clone().into_any_element()
    }

    fn sync_all_inputs(&mut self, cx: &mut Context<Self>) {
        self.sync_url(cx);
        self.sync_search(cx);
        self.sync_filter(cx);
        self.sync_range(cx);
        self.sync_fragment_start(cx);
        self.sync_fragment_end(cx);
        self.sync_placeholders(cx);
    }

    fn sync_placeholders(&mut self, cx: &mut Context<Self>) {
        let locale = self.controller.state.locale;
        if self.placeholder_locale == locale {
            return;
        }
        self.placeholder_locale = locale;
        let s = self.s();
        self.url_input
            .update(cx, |input, cx| input.set_placeholder(s.url_placeholder, cx));
        self.search_input.update(cx, |input, cx| {
            input.set_placeholder(s.search_placeholder, cx)
        });
        self.filter_input.update(cx, |input, cx| {
            input.set_placeholder(s.filter_placeholder, cx)
        });
        self.range_input.update(cx, |input, cx| {
            input.set_placeholder(s.range_placeholder, cx)
        });
    }

    pub(crate) fn sync_url(&mut self, cx: &mut Context<Self>) {
        let before = self.url_cache.clone();
        sync_field(
            &self.url_input,
            &mut self.url_cache,
            &mut self.controller.state.url,
            cx,
        );
        if self.url_cache != before {
            self.controller.state.url_error = None;
        }
    }

    pub(crate) fn sync_search(&mut self, cx: &mut Context<Self>) {
        sync_field(
            &self.search_input,
            &mut self.search_cache,
            &mut self.controller.state.search_query,
            cx,
        );
    }

    pub(crate) fn sync_filter(&mut self, cx: &mut Context<Self>) {
        sync_field(
            &self.filter_input,
            &mut self.filter_cache,
            &mut self.controller.state.playlist_filter,
            cx,
        );
    }

    pub(crate) fn sync_range(&mut self, cx: &mut Context<Self>) {
        let before = self.range_cache.clone();
        sync_field(
            &self.range_input,
            &mut self.range_cache,
            &mut self.controller.state.range_input,
            cx,
        );
        if self.range_cache != before {
            self.controller.state.range_error = None;
        }
    }

    pub(crate) fn sync_fragment_start(&mut self, cx: &mut Context<Self>) {
        sync_field(
            &self.fragment_start_input,
            &mut self.fragment_start_cache,
            &mut self.controller.state.draft.custom_start,
            cx,
        );
    }

    pub(crate) fn sync_fragment_end(&mut self, cx: &mut Context<Self>) {
        sync_field(
            &self.fragment_end_input,
            &mut self.fragment_end_cache,
            &mut self.controller.state.draft.custom_end,
            cx,
        );
    }

    pub(crate) fn paste_url(&mut self, cx: &mut Context<Self>) {
        if let Some(text) = cx.read_from_clipboard().and_then(|item| item.text()) {
            let text = text.trim().to_owned();
            if text.is_empty() {
                self.controller
                    .state
                    .toast(self.s().clipboard_empty, ToastKind::Warn);
            } else {
                self.controller.state.url = extract_youtube_url(&text).unwrap_or(text);
                self.controller.state.url_error = None;
                self.controller.set_route(Route::Home);
            }
        }
        cx.notify();
    }

    fn on_root_key(&mut self, event: &KeyDownEvent, window: &mut Window, cx: &mut Context<Self>) {
        let url_focused = self.url_input.read(cx).handle().is_focused(window);
        let search_focused = self.search_input.read(cx).handle().is_focused(window);
        let filter_focused = self.filter_input.read(cx).handle().is_focused(window);
        let range_focused = self.range_input.read(cx).handle().is_focused(window);
        let text_focused = url_focused || search_focused || filter_focused || range_focused;
        let modal_open = self.controller.state.search_modal
            || self.controller.state.update_modal
            || self.controller.state.mixed_choice.is_some();
        let keystroke = &event.keystroke;
        let secondary = keystroke.modifiers.secondary();
        let key = keystroke.key.as_str();

        if key == "enter" {
            if url_focused {
                self.sync_url(cx);
                self.controller.analyze_url();
                window.focus(&self.root_focus);
            } else if search_focused {
                self.sync_search(cx);
                self.controller.search();
            } else if filter_focused {
                self.sync_filter(cx);
                self.controller.state.refresh_playlist_filter();
            } else if range_focused {
                self.sync_range(cx);
                self.controller.apply_playlist_range();
            }
            if text_focused {
                cx.notify();
                return;
            }
        }
        if key == "escape" {
            if text_focused {
                if search_focused {
                    self.controller.state.search_modal = false;
                }
                window.focus(&self.root_focus);
                cx.notify();
                return;
            }
            if self.controller.state.mixed_choice.is_some() {
                self.controller.state.mixed_choice = None;
            } else if self.controller.state.update_modal {
                if self.controller.state.restart_pending {
                    self.controller.postpone_restart();
                } else {
                    self.controller.state.update_modal = false;
                }
            } else if self.controller.state.search_modal {
                self.controller.state.search_modal = false;
            } else if self.controller.state.route != Route::Home {
                self.controller.set_route(Route::Home);
            }
            cx.notify();
            return;
        }
        if modal_open || text_focused || !secondary {
            return;
        }
        match key {
            "v" => self.paste_url(cx),
            "g" => self.open_search(window, cx),
            "o" => self.controller.pick_folder(),
            "u" | "," => self.controller.set_route(Route::Settings),
            _ => {}
        }
        cx.notify();
    }

    fn render_sidebar(
        &mut self,
        _window: &Window,
        cx: &mut Context<Self>,
        p: &Palette,
    ) -> AnyElement {
        let route = self.controller.state.route;
        let pending = self.controller.state.pending_jobs();
        let s = self.s();
        let update_latest = self
            .controller
            .state
            .update
            .as_ref()
            .map(|update| update.latest.clone());
        let update_label = if self.controller.state.restart_pending {
            s.restart_now
        } else {
            s.update_available
        };
        let logo = "icons/volvid-logo.svg";
        let version = format!("v{}", self.controller.app_version());

        let nav = |id: &'static str, label: &'static str, icon_path: &'static str, active: bool| {
            let fill = if active {
                linear_gradient(
                    90.,
                    linear_color_stop(p.accent, 0.0),
                    linear_color_stop(p.accent_hover, 1.0),
                )
            } else {
                p.panel.into()
            };
            div()
                .id(id)
                .h(px(44.))
                .w_full()
                .px_3()
                .flex()
                .items_center()
                .gap_3()
                .rounded_lg()
                .bg(fill)
                .text_color(if active { p.on_accent } else { p.muted })
                .text_size(px(14.5))
                .font_weight(gpui::FontWeight::SEMIBOLD)
                .cursor_pointer()
                .hover(|this| {
                    if active {
                        this.opacity(0.9)
                    } else {
                        this.bg(p.card_hover).text_color(p.text)
                    }
                })
                .child(theme::icon(
                    icon_path,
                    18.,
                    if active { p.on_accent } else { p.muted },
                ))
                .child(label)
        };

        div()
            .w(px(SIDEBAR_WIDTH))
            .h_full()
            .flex()
            .flex_col()
            .p_4()
            .gap_2()
            .bg(p.panel)
            .border_r_1()
            .border_color(p.divider)
            .child(
                div()
                    .px_2()
                    .py_3()
                    .child(
                        gpui::svg()
                            .path(logo)
                            .w(px(146.))
                            .h(px(31.))
                            .text_color(p.text),
                    )
                    .child(div().text_size(px(11.)).text_color(p.muted).child(version)),
            )
            .child(div().h(px(14.)))
            .child(
                nav(
                    "nav-new",
                    s.nav_new,
                    "icons/new-download.svg",
                    route == Route::Home,
                )
                .on_click(cx.listener(|this, _, _, cx| {
                    this.controller.set_route(Route::Home);
                    cx.notify();
                })),
            )
            .child(
                nav(
                    "nav-queue",
                    s.nav_queue,
                    "icons/queue.svg",
                    route == Route::Downloads,
                )
                .when(pending > 0, |this| {
                    this.child(
                        div()
                            .px_2()
                            .rounded_full()
                            .bg(p.accent)
                            .text_color(p.on_accent)
                            .text_size(px(10.))
                            .child(pending.to_string()),
                    )
                })
                .on_click(cx.listener(|this, _, _, cx| {
                    this.controller.set_route(Route::Downloads);
                    cx.notify();
                })),
            )
            .child(
                nav(
                    "nav-history",
                    s.nav_history,
                    "icons/history.svg",
                    route == Route::Summary,
                )
                .on_click(cx.listener(|this, _, _, cx| {
                    this.controller.set_route(Route::Summary);
                    cx.notify();
                })),
            )
            .child(
                nav(
                    "nav-settings",
                    s.nav_settings,
                    "icons/settings.svg",
                    route == Route::Settings,
                )
                .on_click(cx.listener(|this, _, _, cx| {
                    this.controller.set_route(Route::Settings);
                    cx.notify();
                })),
            )
            .when_some(update_latest, |this, latest| {
                this.child(
                    div()
                        .id("sidebar-update")
                        .mt_2()
                        .p_3()
                        .rounded_lg()
                        .bg(p.warning.opacity(0.12))
                        .border_1()
                        .border_color(p.warning.opacity(0.35))
                        .flex()
                        .items_center()
                        .gap_2()
                        .cursor_pointer()
                        .hover(|this| this.bg(p.warning.opacity(0.2)))
                        .child(theme::icon("icons/refresh.svg", 16., p.warning))
                        .child(
                            div()
                                .flex()
                                .flex_col()
                                .gap_1()
                                .child(
                                    div()
                                        .text_size(px(12.))
                                        .font_weight(gpui::FontWeight::SEMIBOLD)
                                        .text_color(p.warning)
                                        .child(update_label),
                                )
                                .child(div().text_size(px(11.)).text_color(p.muted).child(latest)),
                        )
                        .on_click(cx.listener(|this, _, _, cx| {
                            this.controller.state.update_modal = true;
                            cx.notify();
                        })),
                )
            })
            .child(div().flex_1())
            .into_any_element()
    }

    fn render_page(
        &mut self,
        window: &mut Window,
        cx: &mut Context<Self>,
        p: &Palette,
    ) -> AnyElement {
        match self.controller.state.route {
            Route::Home => pages::home(self, window, cx, p).into_any_element(),
            Route::Configure => pages::configure(self, window, cx, p).into_any_element(),
            Route::Playlist => pages::playlist(self, window, cx, p).into_any_element(),
            Route::Downloads => pages::downloads(self, window, cx, p).into_any_element(),
            Route::Summary => pages::summary(self, window, cx, p).into_any_element(),
            Route::Settings => pages::settings(self, window, cx, p).into_any_element(),
        }
    }

    fn render_toast(&self, cx: &mut Context<Self>, p: &Palette) -> Option<AnyElement> {
        let toast = self.controller.state.toast.as_ref()?;
        if toast.created.elapsed() > TOAST_LIFETIME {
            return None;
        }
        let color = match toast.kind {
            ToastKind::Info => p.accent,
            ToastKind::Success => p.success,
            ToastKind::Warn => p.warning,
            ToastKind::Error => p.danger,
        };
        Some(
            div()
                .absolute()
                .right(px(22.))
                .bottom(px(22.))
                .max_w(px(460.))
                .p_4()
                .rounded_lg()
                .bg(p.card)
                .border_1()
                .border_color(color.opacity(0.6))
                .shadow_lg()
                .flex()
                .items_center()
                .gap_3()
                .child(div().size(px(8.)).rounded_full().bg(color))
                .child(
                    div()
                        .flex_1()
                        .text_size(px(13.))
                        .text_color(p.text)
                        .child(toast.text.clone()),
                )
                .child(
                    theme::ghost_button("toast-close", "×", p).on_click(cx.listener(
                        |this, _, _, cx| {
                            this.controller.state.toast = None;
                            cx.notify();
                        },
                    )),
                )
                .into_any_element(),
        )
    }
}

impl Drop for VolvidApp {
    fn drop(&mut self) {
        self.controller.backend.shutdown();
    }
}

impl Render for VolvidApp {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        self.sync_images();
        self.maybe_restart(window, cx);
        let route = self.controller.state.route;
        if route != Route::Home && self.url_input.read(cx).handle().is_focused(window) {
            window.focus(&self.root_focus);
        }
        if route != Route::Playlist
            && (self.filter_input.read(cx).handle().is_focused(window)
                || self.range_input.read(cx).handle().is_focused(window))
        {
            window.focus(&self.root_focus);
        }
        if route != Route::Configure
            && (self
                .fragment_start_input
                .read(cx)
                .handle()
                .is_focused(window)
                || self.fragment_end_input.read(cx).handle().is_focused(window))
        {
            window.focus(&self.root_focus);
        }
        if !self.controller.state.search_modal
            && self.search_input.read(cx).handle().is_focused(window)
        {
            window.focus(&self.root_focus);
        }
        let blocking_modal =
            self.controller.state.update_modal || self.controller.state.mixed_choice.is_some();
        if blocking_modal
            && (self.url_input.read(cx).handle().is_focused(window)
                || self.search_input.read(cx).handle().is_focused(window)
                || self.filter_input.read(cx).handle().is_focused(window)
                || self.range_input.read(cx).handle().is_focused(window))
        {
            window.focus(&self.root_focus);
        }
        let p = self.palette(window.appearance());

        let page = self.render_page(window, cx, &p);
        let sidebar = self.render_sidebar(window, cx, &p);
        let toast = self.render_toast(cx, &p);
        let overlay = crate::modals::render(self, window, cx, &p);

        div()
            .relative()
            .size_full()
            .font_family(theme::FONT_FAMILY)
            .text_color(p.text)
            .bg(linear_gradient(
                180.,
                linear_color_stop(p.bg, 0.0),
                linear_color_stop(p.bg_bottom, 1.0),
            ))
            .track_focus(&self.root_focus)
            .on_key_down(cx.listener(Self::on_root_key))
            .child(
                div().flex().size_full().child(sidebar).child(
                    div()
                        .flex_1()
                        .h_full()
                        .overflow_hidden()
                        .p(px(24.))
                        .child(page),
                ),
            )
            .when(self.controller.state.toast.is_some(), |this| {
                this.children(toast)
            })
            .children(overlay)
    }
}

fn sync_field(
    input: &Entity<TextInput>,
    cache: &mut String,
    desired: &mut String,
    cx: &mut Context<VolvidApp>,
) {
    let current = input.read(cx).text().to_owned();
    if current != *cache {
        *desired = current.clone();
        *cache = current;
    } else if *desired != *cache {
        let value = desired.clone();
        input.update(cx, |input, cx| input.set_text(value, cx));
        *cache = desired.clone();
    }
}

fn extract_youtube_url(text: &str) -> Option<String> {
    text.split_whitespace().find_map(|raw| {
        let token = raw.trim_matches(|c: char| {
            matches!(
                c,
                '"' | '\'' | '(' | ')' | '[' | ']' | '<' | '>' | ',' | ';' | '!' | '.'
            )
        });
        let lower = token.to_ascii_lowercase();
        ((lower.starts_with("http://") || lower.starts_with("https://"))
            && (lower.contains("youtube.com") || lower.contains("youtu.be")))
        .then(|| token.to_owned())
    })
}

fn preview_image(image: &volvid_engine::PreviewImage) -> Option<Arc<gpui::RenderImage>> {
    let buffer = image::ImageBuffer::<image::Rgba<u8>, Vec<u8>>::from_raw(
        image.width,
        image.height,
        image.rgba.to_vec(),
    )?;
    let frame = image::Frame::new(buffer);
    Some(Arc::new(gpui::RenderImage::new(smallvec::smallvec![frame])))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn extracts_youtube_urls_from_text() {
        assert_eq!(
            extract_youtube_url("watch this: https://youtu.be/dQw4w9WgXcQ."),
            Some("https://youtu.be/dQw4w9WgXcQ".to_owned())
        );
        assert_eq!(
            extract_youtube_url("https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42"),
            Some("https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42".to_owned())
        );
        assert_eq!(extract_youtube_url("https://example.com/video"), None);
        assert_eq!(extract_youtube_url("not a link"), None);
    }
}
