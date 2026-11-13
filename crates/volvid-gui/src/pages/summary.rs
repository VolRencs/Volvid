use gpui::{Context, SharedString, Window, div, prelude::*, px};
use volvid_app::Route;
use volvid_core::DownloadMode;

use crate::app::VolvidApp;
use crate::format;
use crate::theme::{self, Palette};

pub fn summary(
    app: &mut VolvidApp,
    _window: &mut Window,
    cx: &mut Context<VolvidApp>,
    p: &Palette,
) -> impl IntoElement {
    let p = *p;
    let s = app.s();
    let locale = app.controller.state.locale;
    let ok = app
        .controller
        .state
        .history
        .iter()
        .filter(|item| item.ok)
        .count();
    let failed = app.controller.state.history.len().saturating_sub(ok);
    let total = app.controller.state.history.len();
    let history: Vec<_> = app
        .controller
        .state
        .history
        .iter()
        .rev()
        .take(50)
        .cloned()
        .collect();

    let mut page = div()
        .id("page-summary")
        .flex()
        .flex_col()
        .gap_4()
        .h_full()
        .overflow_y_scroll()
        .scrollbar_width(px(8.))
        .pb_5()
        .child(
            div()
                .flex()
                .items_center()
                .gap_3()
                .child(
                    div()
                        .flex()
                        .flex_col()
                        .gap_1()
                        .child(theme::title(s.history_title, &p))
                        .child(theme::muted(s.session_history, &p)),
                )
                .child(div().flex_1())
                .child(
                    theme::secondary_button("history-clear", s.clear_history, &p)
                        .when(total == 0, |this| this.opacity(0.45))
                        .on_click(cx.listener(|this, _, _, cx| {
                            if !this.controller.state.history.is_empty() {
                                this.controller.clear_history();
                            }
                            cx.notify();
                        })),
                )
                .child(
                    theme::primary_button("history-new", s.nav_new, &p).on_click(cx.listener(
                        |this, _, _, cx| {
                            this.controller.set_route(Route::Home);
                            cx.notify();
                        },
                    )),
                ),
        )
        .child(
            theme::card(&p)
                .flex()
                .gap_8()
                .child(stat(s.completed, ok, p.success, &p))
                .child(stat(s.failed, failed, p.danger, &p))
                .child(stat(s.session_history, total, p.accent, &p)),
        );

    if history.is_empty() {
        page = page.child(theme::card(&p).child(theme::empty_state(
            "icons/history.svg",
            s.no_history,
            "",
            &p,
        )));
        return page;
    }

    for (index, item) in history.into_iter().enumerate() {
        let color = if item.ok { p.success } else { p.danger };
        let timestamp = format::timestamp(item.timestamp, locale);
        let output = item.output_dir.clone();
        let mode = match item.mode {
            DownloadMode::Video => s.mode_video,
            DownloadMode::Audio => s.mode_audio,
            DownloadMode::Thumbnail => s.mode_thumbnail,
        };
        page = page.child(
            theme::card(&p)
                .flex()
                .flex_col()
                .gap_2()
                .child(
                    div()
                        .flex()
                        .items_center()
                        .gap_2()
                        .child(div().size(px(8.)).rounded_full().bg(color))
                        .child(
                            div()
                                .flex_1()
                                .min_w(px(0.))
                                .truncate()
                                .text_size(px(14.))
                                .text_color(p.text)
                                .child(item.label),
                        )
                        .when(item.failed > 0, |this| {
                            this.child(theme::badge(
                                s.failed_items.replace("%d", &item.failed.to_string()),
                                p.danger,
                                p.card_hover,
                            ))
                        })
                        .child(theme::muted(timestamp, &p)),
                )
                .when(!item.url.is_empty(), |this| {
                    this.child(
                        div()
                            .truncate()
                            .text_size(px(11.5))
                            .text_color(p.muted)
                            .child(item.url.clone()),
                    )
                })
                .child(
                    div()
                        .flex()
                        .items_center()
                        .gap_2()
                        .child(theme::badge(mode, p.accent, p.card_hover))
                        .when(item.items > 1, |this| {
                            this.child(theme::muted(
                                s.job_items.replace("%d", &item.items.to_string()),
                                &p,
                            ))
                        })
                        .child(div().flex_1())
                        .child(
                            theme::ghost_button(
                                SharedString::from(format!(
                                    "history-open-{}-{index}",
                                    item.timestamp
                                )),
                                s.open_folder,
                                &p,
                            )
                            .on_click(cx.listener(
                                move |this, _, _, _| {
                                    this.controller.open_folder(output.clone());
                                },
                            )),
                        ),
                ),
        );
    }
    page
}

fn stat(label: &str, value: usize, color: gpui::Hsla, p: &Palette) -> impl IntoElement {
    div()
        .flex()
        .flex_col()
        .gap_1()
        .child(
            div()
                .text_size(px(28.))
                .font_weight(gpui::FontWeight::BOLD)
                .text_color(color)
                .child(value.to_string()),
        )
        .child(theme::muted(label.to_owned(), p))
}
