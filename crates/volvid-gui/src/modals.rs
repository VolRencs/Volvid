use std::time::Instant;

use gpui::{AnyElement, Context, SharedString, Window, div, prelude::*, px};

use crate::app::VolvidApp;
use crate::theme::{self, Palette};

pub fn render(
    app: &mut VolvidApp,
    window: &mut Window,
    cx: &mut Context<VolvidApp>,
    p: &Palette,
) -> Vec<AnyElement> {
    let mut overlays = Vec::new();
    if app.controller.state.search_modal {
        overlays.push(search(app, window, cx, p));
    }
    if app.controller.state.update_modal {
        overlays.push(update(app, window, cx, p));
    }
    if app.controller.state.mixed_choice.is_some() {
        overlays.push(mixed(app, window, cx, p));
    }
    overlays
}

fn overlay(content: impl IntoElement) -> AnyElement {
    div()
        .absolute()
        .top(px(0.))
        .left(px(0.))
        .size_full()
        .flex()
        .items_center()
        .justify_center()
        .bg(gpui::black().opacity(0.62))
        .child(content)
        .into_any_element()
}

fn modal_shell(p: &Palette, width: f32) -> gpui::Div {
    div()
        .w(px(width))
        .max_w(px(720.))
        .p_5()
        .rounded_xl()
        .bg(p.card)
        .border_1()
        .border_color(p.border)
        .shadow_xl()
        .flex()
        .flex_col()
        .gap_4()
}

fn modal_header(
    title: impl Into<SharedString>,
    subtitle: impl Into<SharedString>,
    icon: &'static str,
    p: &Palette,
) -> gpui::Div {
    div()
        .flex()
        .items_center()
        .gap_3()
        .child(
            div()
                .size(px(44.))
                .rounded_lg()
                .bg(p.selection.opacity(0.14))
                .border_1()
                .border_color(p.selection.opacity(0.35))
                .flex()
                .items_center()
                .justify_center()
                .child(theme::icon(icon, 22., p.selection)),
        )
        .child(
            div()
                .flex()
                .flex_col()
                .gap_1()
                .flex_1()
                .min_w(px(0.))
                .child(
                    div()
                        .text_size(px(21.))
                        .font_weight(gpui::FontWeight::BOLD)
                        .text_color(p.text)
                        .child(title.into()),
                )
                .child(theme::muted(subtitle, p)),
        )
}

fn close_button(
    id: &'static str,
    cx: &mut Context<VolvidApp>,
    p: &Palette,
    on_close: fn(&mut VolvidApp),
) -> impl IntoElement {
    theme::ghost_button(id, "×", p).on_click(cx.listener(move |this, _, _, cx| {
        on_close(this);
        cx.notify();
    }))
}

fn search(
    app: &mut VolvidApp,
    window: &mut Window,
    cx: &mut Context<VolvidApp>,
    p: &Palette,
) -> AnyElement {
    let s = app.s();
    let field = app.search_field(window, cx, p);
    let results: Vec<_> = app.controller.state.search_results.clone();
    let loading = app.controller.state.search_loading;
    let error = app.controller.state.search_error.clone();
    let has_error = error.is_some();

    let mut body = modal_shell(p, 620.)
        .child(
            div()
                .flex()
                .items_start()
                .gap_2()
                .child(modal_header(
                    s.search_title,
                    s.search_prompt,
                    "icons/search.svg",
                    p,
                ))
                .child(close_button("search-close", cx, p, |app| {
                    app.controller.state.search_modal = false;
                })),
        )
        .child(
            div()
                .flex()
                .items_center()
                .gap_2()
                .child(div().flex_1().min_w(px(0.)).child(field))
                .child(
                    theme::primary_button("search-run", s.search_youtube, p).on_click(cx.listener(
                        |this, _, _, cx| {
                            this.sync_search(cx);
                            this.controller.search();
                            cx.notify();
                        },
                    )),
                ),
        );
    if loading {
        body = body.child(theme::muted(s.searching, p));
    }
    if let Some(error) = error {
        body = body.child(theme::body(error, p).text_color(p.danger));
    }
    let mut list = div()
        .id("search-results")
        .max_h(px(260.))
        .overflow_y_scroll()
        .scrollbar_width(px(8.))
        .flex()
        .flex_col()
        .gap_2();
    if results.is_empty() && !loading && !has_error {
        list = list.child(theme::empty_state("icons/search.svg", s.no_results, "", p));
    } else {
        for (index, result) in results.iter().enumerate() {
            let title = result.title.clone();
            let duration = volvid_core::format_duration(result.duration);
            list = list.child(
                div()
                    .id(("search-result", index))
                    .flex()
                    .items_center()
                    .gap_2()
                    .p_3()
                    .rounded_lg()
                    .bg(p.card_alt)
                    .border_1()
                    .border_color(p.border)
                    .cursor_pointer()
                    .hover(|this| this.bg(p.card_hover).border_color(p.selection.opacity(0.6)))
                    .child(
                        div()
                            .flex_1()
                            .min_w(px(0.))
                            .truncate()
                            .text_size(px(14.))
                            .text_color(p.text)
                            .child(title),
                    )
                    .child(theme::muted(duration, p))
                    .on_click(cx.listener(move |this, _, _, cx| {
                        this.controller.select_search_result(index);
                        cx.notify();
                    })),
            );
        }
    }
    body = body.child(list).child(div().flex().justify_end().child(
        theme::ghost_button("search-cancel", s.close, p).on_click(cx.listener(|this, _, _, cx| {
            this.controller.state.search_modal = false;
            cx.notify();
        })),
    ));
    overlay(body)
}

fn update(
    app: &mut VolvidApp,
    _window: &mut Window,
    cx: &mut Context<VolvidApp>,
    p: &Palette,
) -> AnyElement {
    let s = app.s();
    let Some(update) = app.controller.state.update.clone() else {
        app.controller.state.update_modal = false;
        return div().into_any_element();
    };
    let pending = app.controller.state.restart_pending;
    let restarting = app.controller.state.restarting;
    let updating = app.controller.state.updating;
    let pending_jobs = app.controller.state.pending_jobs();
    let progress = app.controller.state.update_progress.clone();

    let mut body = modal_shell(p, 520.).child(
        div()
            .flex()
            .items_start()
            .gap_2()
            .child(modal_header(
                if pending {
                    s.update_installed
                } else {
                    s.update_available
                },
                format!(
                    "{} {} → {}",
                    s.current_version,
                    app.controller.app_version(),
                    update.latest
                ),
                "icons/refresh.svg",
                p,
            ))
            .child(close_button("update-close", cx, p, |app| {
                if app.controller.state.restart_pending {
                    app.controller.postpone_restart();
                } else {
                    app.controller.state.update_modal = false;
                }
            })),
    );

    if pending {
        if restarting {
            body = body.child(theme::muted(s.restarting, p));
        } else if pending_jobs > 0 {
            body = body.child(theme::muted(s.restart_wait_jobs, p));
        } else {
            let remaining = app
                .controller
                .state
                .restart_at
                .map(|at| at.saturating_duration_since(Instant::now()))
                .unwrap_or_default();
            let seconds = remaining.as_secs().max(1);
            body = body
                .child(theme::body(
                    s.restart_countdown.replace("%d", &seconds.to_string()),
                    p,
                ))
                .child(theme::progress_bar(
                    p,
                    1.0 - (remaining.as_secs_f32() / 2.0).clamp(0.0, 1.0),
                    6.,
                ));
        }
        body = body.child(
            div()
                .flex()
                .gap_2()
                .child(
                    theme::primary_button("update-restart", s.restart_now, p)
                        .when(restarting || pending_jobs > 0, |this| this.opacity(0.45))
                        .on_click(cx.listener(|this, _, _, cx| {
                            if this.controller.state.pending_jobs() == 0 {
                                this.controller.restart_now();
                                cx.notify();
                            }
                        })),
                )
                .child(
                    theme::secondary_button("update-later", s.later, p).on_click(cx.listener(
                        |this, _, _, cx| {
                            this.controller.postpone_restart();
                            cx.notify();
                        },
                    )),
                ),
        );
    } else if updating {
        body = body.child(theme::muted(s.updating, p));
        if let Some(progress) = progress {
            body = body
                .child(theme::progress_bar(p, (progress.pct / 100.0) as f32, 8.))
                .child(theme::muted(
                    format!(
                        "{:.1}%  ·  {}",
                        progress.pct,
                        volvid_i18n::format_bytes(progress.done_b, app.controller.state.locale)
                    ),
                    p,
                ));
        }
    } else {
        body = body.child(
            div()
                .flex()
                .gap_2()
                .child(
                    theme::primary_button("update-install", s.install_update, p).on_click(
                        cx.listener(|this, _, _, cx| {
                            this.controller.apply_update();
                            cx.notify();
                        }),
                    ),
                )
                .child(
                    theme::secondary_button("update-later", s.later, p).on_click(cx.listener(
                        |this, _, _, cx| {
                            this.controller.state.update_modal = false;
                            cx.notify();
                        },
                    )),
                ),
        );
    }
    overlay(body)
}

fn mixed(
    app: &mut VolvidApp,
    _window: &mut Window,
    cx: &mut Context<VolvidApp>,
    p: &Palette,
) -> AnyElement {
    let s = app.s();
    let Some(choice) = app.controller.state.mixed_choice.clone() else {
        return div().into_any_element();
    };
    let body = modal_shell(p, 480.)
        .child(
            div()
                .flex()
                .items_start()
                .gap_2()
                .child(modal_header(
                    s.mixed_title,
                    s.mixed_hint,
                    "icons/queue.svg",
                    p,
                ))
                .child(close_button("mixed-close", cx, p, |app| {
                    app.controller.state.mixed_choice = None;
                })),
        )
        .child(
            div()
                .p_3()
                .rounded_lg()
                .bg(p.card_alt)
                .border_1()
                .border_color(p.border)
                .truncate()
                .text_size(px(12.))
                .text_color(p.muted)
                .child(choice.url),
        )
        .child(
            div()
                .flex()
                .gap_2()
                .child(
                    theme::primary_button("mixed-video", s.video_only, p).on_click(cx.listener(
                        |this, _, _, cx| {
                            this.controller.choose_mixed(true);
                            cx.notify();
                        },
                    )),
                )
                .child(
                    theme::secondary_button("mixed-playlist", s.full_playlist, p).on_click(
                        cx.listener(|this, _, _, cx| {
                            this.controller.choose_mixed(false);
                            cx.notify();
                        }),
                    ),
                ),
        );
    overlay(body)
}
