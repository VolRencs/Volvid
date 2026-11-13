use gpui::{
    AnyElement, Context, MouseDownEvent, MouseMoveEvent, MouseUpEvent, SharedString, Window,
    canvas, div, point, prelude::*, px,
};
use volvid_app::Route;
use volvid_core::format_duration;

use super::components::segment;
use crate::app::VolvidApp;
use crate::theme::{self, Palette};

pub fn playlist(
    app: &mut VolvidApp,
    window: &mut Window,
    cx: &mut Context<VolvidApp>,
    p: &Palette,
) -> impl IntoElement {
    let p = *p;
    let s = app.s();
    let range_error = app.controller.state.range_error.clone();
    if app.controller.state.playlist_loading {
        return div()
            .id("page-playlist")
            .size_full()
            .flex()
            .flex_col()
            .gap_3()
            .child(
                theme::secondary_button("playlist-back", s.back, &p).on_click(cx.listener(
                    |this, _, _, cx| {
                        this.controller.set_route(Route::Home);
                        cx.notify();
                    },
                )),
            )
            .child(theme::empty_state(
                "icons/queue.svg",
                s.loading_playlist,
                "",
                &p,
            ));
    }
    if app.controller.state.playlist.is_none() {
        return div()
            .id("page-playlist")
            .size_full()
            .flex()
            .flex_col()
            .gap_3()
            .child(
                theme::secondary_button("playlist-back", s.back, &p).on_click(cx.listener(
                    |this, _, _, cx| {
                        this.controller.set_route(Route::Home);
                        cx.notify();
                    },
                )),
            )
            .child(theme::empty_state(
                "icons/queue.svg",
                s.playlist_empty,
                "",
                &p,
            ));
    }

    let title = app.controller.state.localize_playlist_title();
    if app.controller.state.playlist_scroll_reset {
        app.playlist_scroll
            .0
            .borrow()
            .base_handle
            .set_offset(point(px(0.), px(0.)));
        app.controller.state.playlist_scroll_reset = false;
    }
    app.controller.state.refresh_playlist_filter();
    let filtered = app.controller.state.playlist_filtered.clone();
    let total_all = app.controller.state.playlist_entries().len();
    let total = filtered.len();
    let selected = app.controller.state.selected_entries.len();
    let workers = app.controller.state.draft.workers;
    let filter = app.filter_field(window, cx, &p);
    let range = app.range_field(window, cx, &p);
    let mut workers_row = div()
        .flex()
        .flex_wrap()
        .items_center()
        .gap_1()
        .child(theme::muted(s.workers, &p));
    for count in 1..=8 {
        workers_row = workers_row.child(
            segment(
                SharedString::from(format!("workers-{count}")),
                count.to_string(),
                workers == count,
                &p,
            )
            .on_click(cx.listener(move |this, _, _, cx| {
                this.controller.set_playlist_workers(count);
                cx.notify();
            })),
        );
    }

    let list = gpui::uniform_list(
        "playlist-rows",
        total,
        cx.processor(|this, range, _window, cx| {
            let entries: Vec<volvid_core::PlaylistEntry> =
                this.controller.state.playlist_entries().to_vec();
            let filtered = this.controller.state.playlist_filtered.clone();
            let selected = this.controller.state.selected_entries.clone();
            let p = this.palette(_window.appearance());
            let mut rows = Vec::new();
            for index in range {
                let Some(entry_index) = filtered.get(index).copied() else {
                    continue;
                };
                let entry: volvid_core::PlaylistEntry = match entries.get(entry_index).cloned() {
                    Some(entry) => entry,
                    None => continue,
                };
                let checked = selected.contains(&entry.index);
                rows.push(
                    div()
                        .id(("playlist-row", entry.index))
                        .h(px(40.))
                        .w_full()
                        .px_2()
                        .flex()
                        .items_center()
                        .gap_2()
                        .rounded_md()
                        .bg(if checked {
                            p.selection.opacity(0.12)
                        } else {
                            p.card
                        })
                        .hover(|this| this.bg(p.card_hover))
                        .cursor_pointer()
                        .child(theme::icon(
                            if checked {
                                "icons/check.svg"
                            } else {
                                "icons/info.svg"
                            },
                            15.,
                            if checked { p.selection } else { p.muted },
                        ))
                        .child(
                            div()
                                .w(px(34.))
                                .text_size(px(11.5))
                                .text_color(if checked { p.selection } else { p.muted })
                                .child(format!("{:>3}", entry.index)),
                        )
                        .child(
                            div()
                                .flex_1()
                                .min_w(px(0.))
                                .truncate()
                                .text_size(px(14.))
                                .text_color(p.text)
                                .child(entry.title),
                        )
                        .child(theme::muted(format_duration(entry.duration), &p))
                        .on_click(cx.listener(move |this, _, _, cx| {
                            this.controller.toggle_playlist(entry.index);
                            cx.notify();
                        })),
                );
            }
            rows
        }),
    )
    .track_scroll(app.playlist_scroll.clone())
    .flex_1()
    .h_full();

    let scrollbar = playlist_scrollbar(app, &p, cx);
    let list_area = div()
        .relative()
        .flex_1()
        .min_h(px(0.))
        .child(list)
        .child(scrollbar);

    div()
        .id("page-playlist")
        .size_full()
        .flex()
        .flex_col()
        .gap_3()
        .child(
            div()
                .flex()
                .items_center()
                .gap_3()
                .child(
                    theme::secondary_button("playlist-back", s.back, &p).on_click(cx.listener(
                        |this, _, _, cx| {
                            this.controller.set_route(Route::Home);
                            cx.notify();
                        },
                    )),
                )
                .child(
                    div()
                        .flex()
                        .flex_col()
                        .gap_1()
                        .child(theme::page_title(
                            if title.is_empty() {
                                s.playlist_title.to_owned()
                            } else {
                                title.clone()
                            },
                            &p,
                        ))
                        .child(theme::muted(
                            if total == total_all {
                                format!(
                                    "{}  ·  {}",
                                    s.videos_count.replace("%d", &total_all.to_string()),
                                    s.selected_count
                                        .replacen("%d", &selected.to_string(), 1)
                                        .replacen("%d", &total_all.to_string(), 1)
                                )
                            } else {
                                format!(
                                    "{}  ·  {} / {}  ·  {}",
                                    s.videos_count.replace("%d", &total_all.to_string()),
                                    total,
                                    total_all,
                                    s.selected_count
                                        .replacen("%d", &selected.to_string(), 1)
                                        .replacen("%d", &total_all.to_string(), 1)
                                )
                            },
                            &p,
                        )),
                ),
        )
        .child(
            theme::card(&p)
                .flex()
                .flex_col()
                .gap_3()
                .flex_1()
                .min_h(px(0.))
                .child(
                    div()
                        .flex()
                        .flex_wrap()
                        .items_center()
                        .gap_2()
                        .child(div().w(px(240.)).child(filter))
                        .child(
                            theme::secondary_button("playlist-all", s.select_all, &p).on_click(
                                cx.listener(|this, _, _, cx| {
                                    this.controller.select_all_playlist();
                                    cx.notify();
                                }),
                            ),
                        )
                        .child(
                            theme::secondary_button("playlist-none", s.select_none, &p).on_click(
                                cx.listener(|this, _, _, cx| {
                                    this.controller.clear_playlist();
                                    cx.notify();
                                }),
                            ),
                        )
                        .child(div().w(px(120.)).child(range))
                        .child(
                            theme::primary_button("playlist-range", s.apply_range, &p).on_click(
                                cx.listener(|this, _, _, cx| {
                                    this.sync_range(cx);
                                    this.controller.apply_playlist_range();
                                    cx.notify();
                                }),
                            ),
                        ),
                )
                .when_some(range_error, |this, error| {
                    this.child(theme::notice("icons/alert.svg", error, p.warning))
                })
                .child(workers_row)
                .child(theme::divider(&p))
                .child(
                    div()
                        .flex()
                        .gap_2()
                        .px_2()
                        .text_size(px(11.5))
                        .text_color(p.muted)
                        .child(div().w(px(15.)))
                        .child(div().w(px(34.)).child("#"))
                        .child(s.table_name)
                        .child(div().flex_1())
                        .child(s.table_duration),
                )
                .child(list_area),
        )
        .child(
            div()
                .flex()
                .items_center()
                .gap_2()
                .child(div().flex_1())
                .when(selected == 0, |this| {
                    this.child(theme::muted(s.no_selection, &p).text_color(p.warning))
                })
                .child(
                    theme::primary_button("playlist-continue", s.continue_label, &p)
                        .when(selected == 0, |this| this.opacity(0.45))
                        .on_click(cx.listener(|this, _, _, cx| {
                            if !this.controller.state.selected_entries.is_empty() {
                                this.controller.continue_playlist();
                            }
                            cx.notify();
                        })),
                ),
        )
}

fn playlist_scrollbar(app: &mut VolvidApp, p: &Palette, cx: &mut Context<VolvidApp>) -> AnyElement {
    let (viewport, offset, content_height) = {
        let state = app.playlist_scroll.0.borrow();
        (
            state.base_handle.bounds().size.height,
            state.base_handle.offset().y,
            state
                .last_item_size
                .map(|size| size.contents.height)
                .unwrap_or(px(0.)),
        )
    };
    if viewport <= px(0.) || content_height <= viewport + px(1.) {
        return div().into_any_element();
    }

    let thumb_height = (viewport * (viewport / content_height)).clamp(px(28.), viewport);
    let max_scroll = content_height - viewport;
    let top = (-offset / max_scroll).clamp(0., 1.) * (viewport - thumb_height);
    let scroll_handle = app.playlist_scroll.0.borrow().base_handle.clone();
    let entity = cx.entity();

    div()
        .absolute()
        .top(px(4.))
        .bottom(px(4.))
        .right(px(2.))
        .w(px(8.))
        .rounded_full()
        .bg(p.track.opacity(0.7))
        .child(
            div()
                .id("playlist-scrollbar")
                .absolute()
                .top(top)
                .right(px(0.))
                .w(px(8.))
                .h(thumb_height)
                .rounded_full()
                .bg(p.selection.opacity(0.65))
                .hover(|this| this.bg(p.selection))
                .child(
                    canvas(
                        |_, _, _| (),
                        move |thumb_bounds, _, window, _| {
                            window.on_mouse_event({
                                let entity = entity.clone();
                                move |ev: &MouseDownEvent, _, _, cx| {
                                    if !thumb_bounds.contains(&ev.position) {
                                        return;
                                    }
                                    entity.update(cx, |this, _| {
                                        this.playlist_drag = Some(point(
                                            ev.position.x - thumb_bounds.origin.x,
                                            ev.position.y - thumb_bounds.origin.y,
                                        ));
                                    });
                                }
                            });
                            window.on_mouse_event({
                                let entity = entity.clone();
                                move |_: &MouseUpEvent, _, _, cx| {
                                    entity.update(cx, |this, _| this.playlist_drag = None);
                                    cx.notify(entity.entity_id());
                                }
                            });
                            window.on_mouse_event(move |ev: &MouseMoveEvent, _, _, cx| {
                                if !ev.dragging() {
                                    return;
                                }
                                let Some(drag) = entity.read(cx).playlist_drag else {
                                    return;
                                };
                                let list_top = thumb_bounds.origin.y - top;
                                let percentage = ((ev.position.y - list_top - drag.y
                                    + thumb_height / 2.0)
                                    / viewport)
                                    .clamp(0., 1.);
                                scroll_handle.set_offset(point(px(0.), -(max_scroll * percentage)));
                                cx.notify(entity.entity_id());
                            });
                        },
                    )
                    .size_full(),
                ),
        )
        .into_any_element()
}
