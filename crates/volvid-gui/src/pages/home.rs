use gpui::{Context, SharedString, Window, div, prelude::*, px};
use volvid_app::Route;

use crate::app::VolvidApp;
use crate::format;
use crate::theme::{self, Palette};

pub fn home(
    app: &mut VolvidApp,
    window: &mut Window,
    cx: &mut Context<VolvidApp>,
    p: &Palette,
) -> impl IntoElement {
    let p = *p;
    let s = app.s();
    let locale = app.controller.state.locale;
    let missing =
        !app.controller.state.deps_checking && app.controller.state.deps.missing_required();
    let checking = app.controller.state.deps_checking;
    let url_error = app.controller.state.url_error.clone();
    let folder = app.controller.state.downloads_dir.clone();
    let history: Vec<_> = app
        .controller
        .state
        .history
        .iter()
        .rev()
        .take(4)
        .cloned()
        .collect();
    let url_field = app.url_field(window, cx, &p);
    let analyze = theme::primary_button("home-analyze", s.analyze, &p).on_click(cx.listener(
        |this, _, window, cx| {
            this.sync_url(cx);
            this.controller.analyze_url();
            window.focus(&this.root_focus);
            cx.notify();
        },
    ));
    let paste = theme::secondary_button("home-paste", s.paste, &p)
        .on_click(cx.listener(|this, _, _, cx| this.paste_url(cx)));
    let search = theme::secondary_button("home-search", s.search_youtube, &p)
        .on_click(cx.listener(|this, _, window, cx| this.open_search(window, cx)));
    let mut page = div()
        .id("page-home")
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
                .flex_col()
                .gap_1()
                .child(theme::title(s.home_title, &p))
                .child(theme::muted(s.home_subtitle, &p)),
        );
    if checking {
        page = page.child(theme::loading(s.checking, &p));
    }
    if missing {
        page = page.child(
            div()
                .p_3()
                .rounded_lg()
                .bg(p.danger.opacity(0.1))
                .border_1()
                .border_color(p.danger.opacity(0.35))
                .flex()
                .items_center()
                .gap_3()
                .child(theme::icon("icons/alert.svg", 18., p.danger))
                .child(theme::body(s.deps_missing, &p))
                .child(div().flex_1())
                .child(
                    theme::secondary_button("home-open-deps", s.deps_title, &p).on_click(
                        cx.listener(|this, _, _, cx| {
                            this.controller.set_route(Route::Settings);
                            cx.notify();
                        }),
                    ),
                ),
        );
    }
    page = page.child(
        theme::hero(&p)
            .flex()
            .flex_col()
            .gap_3()
            .child(
                div()
                    .flex()
                    .items_center()
                    .gap_2()
                    .child(div().flex_1().min_w(px(0.)).child(url_field))
                    .child(analyze),
            )
            .child(
                div()
                    .flex()
                    .items_center()
                    .gap_2()
                    .child(paste)
                    .child(search)
                    .child(div().flex_1()),
            )
            .when_some(url_error, |this, error| {
                this.child(theme::notice("icons/alert.svg", error, p.danger))
            }),
    );

    let mut location = theme::card(&p).flex().flex_col().gap_3().child(
        div()
            .flex()
            .items_center()
            .gap_2()
            .child(theme::icon("icons/folder.svg", 18., p.accent))
            .child(theme::section_title(s.download_location, &p)),
    );
    location = location
        .child(
            div()
                .p_2()
                .rounded_md()
                .bg(p.card_alt)
                .border_1()
                .border_color(p.border)
                .truncate()
                .text_size(px(12.5))
                .text_color(p.muted)
                .child(folder.to_string_lossy().into_owned()),
        )
        .child(
            div()
                .flex()
                .gap_2()
                .child(
                    theme::secondary_button("home-choose", s.choose_folder, &p)
                        .on_click(cx.listener(|this, _, _, _cx| this.controller.pick_folder())),
                )
                .child(
                    theme::secondary_button("home-open-folder", s.open_folder, &p).on_click(
                        cx.listener(move |this, _, _, _cx| {
                            this.controller.open_folder(folder.clone());
                        }),
                    ),
                ),
        );

    let mut recent = theme::card(&p).flex().flex_col().gap_3();
    recent = recent.child(
        div()
            .flex()
            .items_center()
            .gap_2()
            .child(theme::icon("icons/history.svg", 18., p.selection))
            .child(theme::section_title(s.recent, &p))
            .child(div().flex_1())
            .when(!history.is_empty(), |this| {
                this.child(
                    theme::ghost_button("home-all-history", s.nav_history, &p).on_click(
                        cx.listener(|this, _, _, cx| {
                            this.controller.set_route(Route::Summary);
                            cx.notify();
                        }),
                    ),
                )
            }),
    );
    if history.is_empty() {
        recent = recent.child(theme::empty_state("icons/info.svg", s.no_history, "", &p));
    } else {
        let mut rows = Vec::new();
        for (index, item) in history.into_iter().enumerate() {
            let color = if item.ok { p.success } else { p.danger };
            let output = item.output_dir.clone();
            let stamp = format::timestamp(item.timestamp, locale);
            rows.push(
                div()
                    .id(SharedString::from(format!(
                        "history-{}-{index}",
                        item.timestamp
                    )))
                    .flex()
                    .items_center()
                    .gap_2()
                    .p_2()
                    .rounded_md()
                    .hover(|this| this.bg(p.card_hover))
                    .child(div().size(px(8.)).rounded_full().bg(color))
                    .child(
                        div()
                            .flex_1()
                            .min_w(px(0.))
                            .truncate()
                            .text_size(px(13.))
                            .text_color(p.text)
                            .child(item.label),
                    )
                    .child(theme::muted(stamp, &p))
                    .child(
                        theme::ghost_button(
                            SharedString::from(format!("open-{}-{index}", item.timestamp)),
                            s.open,
                            &p,
                        )
                        .on_click(cx.listener(move |this, _, _, _cx| {
                            this.controller.open_folder(output.clone());
                        })),
                    ),
            );
        }
        recent = recent.children(rows);
    }

    page.child(
        div()
            .flex()
            .flex_wrap()
            .items_start()
            .gap_4()
            .child(div().flex_1().min_w(px(300.)).child(location))
            .child(div().flex_1().min_w(px(300.)).child(recent)),
    )
}
