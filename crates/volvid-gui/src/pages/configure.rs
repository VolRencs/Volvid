use gpui::{Context, SharedString, Window, div, linear_gradient, prelude::*, px};
use volvid_app::{FragmentMode, Route};
use volvid_core::{DownloadMode, format_duration};
use volvid_i18n::{format_bytes, fragment_error_text};

use super::components::{option_row, segment, track_chip};
use crate::app::VolvidApp;
use crate::theme::{self, Palette};

pub fn configure(
    app: &mut VolvidApp,
    window: &mut Window,
    cx: &mut Context<VolvidApp>,
    p: &Palette,
) -> impl IntoElement {
    let p = *p;
    let s = app.s();
    let locale = app.controller.state.locale;
    let title = app.controller.state.title_for_configure();
    let duration = app.controller.state.media_duration;
    let is_playlist = app.controller.state.is_playlist_mode();
    let selected_count = app.controller.state.selected_entries.len();
    let quality_error = app.controller.state.quality_error.clone();
    let thumbnail_loading = app.controller.state.thumbnail_loading;
    let thumbnail_error = app.controller.state.thumbnail_error;
    let target = app.controller.state.target.clone();
    let fragment_label = app
        .controller
        .state
        .draft
        .fragment_label(target.as_ref(), duration);

    let header = div()
        .flex()
        .items_center()
        .gap_3()
        .child(
            theme::secondary_button("configure-back", s.back, &p).on_click(cx.listener(
                |this, _, _, cx| {
                    let route = if this.controller.state.is_playlist_mode() {
                        Route::Playlist
                    } else {
                        Route::Home
                    };
                    this.controller.set_route(route);
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
                        s.configure_title.to_owned()
                    } else {
                        title.clone()
                    },
                    &p,
                ))
                .child(theme::muted(
                    if is_playlist {
                        format!(
                            "{}  ·  {}",
                            format_duration(duration),
                            s.videos_count.replace("%d", &selected_count.to_string())
                        )
                    } else {
                        format!("{} {}", s.duration, format_duration(duration))
                    },
                    &p,
                )),
        );

    let mode = app.controller.state.draft.mode;
    let mut modes = div().flex().gap_2();
    for (value, label) in [
        (DownloadMode::Video, s.mode_video),
        (DownloadMode::Audio, s.mode_audio),
        (DownloadMode::Thumbnail, s.mode_thumbnail),
    ] {
        modes = modes.child(
            segment(
                SharedString::from(format!("mode-{value:?}")),
                label,
                mode == value,
                &p,
            )
            .on_click(cx.listener(move |this, _, _, cx| {
                this.controller.set_mode(value);
                cx.notify();
            })),
        );
    }

    let choices = app.controller.state.quality_choices.clone();
    let selected_quality = app.controller.state.draft.quality.clone();
    let profiles = app.controller.state.draft.video_profiles(locale);
    let audio_profiles = app.controller.state.draft.audio_profiles(locale);
    let video_index = app.controller.state.draft.video_profile_index;
    let audio_index = app.controller.state.draft.audio_profile_index;

    let mut left = div().flex().flex_col().gap_4();
    left = left.child(modes);
    match mode {
        DownloadMode::Video => {
            let mut quality_card = theme::card(&p).flex().flex_col().gap_2().child(
                div()
                    .flex()
                    .items_center()
                    .gap_2()
                    .child(theme::section_title(s.quality_title, &p))
                    .when(app.controller.state.quality_loading, |this| {
                        this.child(theme::muted(s.loading_quality, &p))
                    }),
            );
            for choice in choices {
                let selected = selected_quality.as_ref().is_some_and(|current| {
                    current.height == choice.height
                        && current.best == choice.best
                        && current.worst == choice.worst
                });
                let label = volvid_i18n::quality_label(&choice, locale);
                let value = choice.clone();
                quality_card = quality_card.child(
                    option_row(
                        SharedString::from(format!("quality-{}", choice.height)),
                        selected,
                        &label,
                        &p,
                    )
                    .on_click(cx.listener(move |this, _, _, cx| {
                        this.controller.state.draft.quality = Some(value.clone());
                        this.controller.state.draft.video_profile_index = 0;
                        cx.notify();
                    })),
                );
            }
            if let Some(error) = quality_error {
                quality_card =
                    quality_card.child(theme::notice("icons/alert.svg", error, p.warning));
            }
            left = left.child(quality_card);

            let mut formats = theme::card(&p)
                .flex()
                .flex_col()
                .gap_2()
                .child(theme::section_title(s.format_title, &p));
            for (index, profile) in profiles.iter().enumerate() {
                formats = formats.child(
                    option_row(
                        SharedString::from(format!("format-{index}")),
                        video_index == index,
                        &profile.label,
                        &p,
                    )
                    .on_click(cx.listener(move |this, _, _, cx| {
                        this.controller.state.draft.video_profile_index = index;
                        cx.notify();
                    })),
                );
            }
            left = left.child(formats);
            left = left.child(advanced_tracks(app, cx, &p, true));
            left = left.child(fragment_card(app, window, cx, &p));
        }
        DownloadMode::Audio => {
            let mut card = theme::card(&p)
                .flex()
                .flex_col()
                .gap_2()
                .child(theme::section_title(s.audio_title, &p));
            for (index, profile) in audio_profiles.iter().enumerate() {
                card = card.child(
                    option_row(
                        SharedString::from(format!("audio-{index}")),
                        audio_index == index,
                        &profile.label,
                        &p,
                    )
                    .on_click(cx.listener(move |this, _, _, cx| {
                        this.controller.state.draft.audio_profile_index = index;
                        cx.notify();
                    })),
                );
            }
            left = left.child(card);
            left = left.child(advanced_tracks(app, cx, &p, false));
            left = left.child(fragment_card(app, window, cx, &p));
        }
        DownloadMode::Thumbnail => {
            left = left.child(
                theme::card(&p)
                    .flex()
                    .flex_col()
                    .gap_2()
                    .child(theme::section_title(s.thumbnail_title, &p))
                    .child(theme::muted(s.thumbnail_hint, &p)),
            );
        }
    }

    let folder = app.controller.state.downloads_dir.clone();
    left = left.child(
        theme::card(&p)
            .flex()
            .flex_col()
            .gap_2()
            .child(theme::section_title(s.output_folder, &p))
            .child(
                div()
                    .p_2()
                    .rounded_md()
                    .bg(p.card_alt)
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
                        theme::secondary_button("configure-choose", s.choose_folder, &p)
                            .on_click(cx.listener(|this, _, _, _| this.controller.pick_folder())),
                    )
                    .child(
                        theme::secondary_button("configure-open", s.open_folder, &p).on_click(
                            cx.listener(move |this, _, _, _| {
                                this.controller.open_folder(folder.clone());
                            }),
                        ),
                    ),
            ),
    );

    let preview = app.preview().cloned();
    let profile = app.controller.state.draft.profile(locale);
    let quality_label = profile.label.clone();
    let size = estimate_size(app);
    let ready = app.controller.state.deps.ytrust.available
        && !app.controller.state.deps_checking
        && !app.controller.state.probe_loading
        && !app.controller.state.quality_loading;
    let mut download = theme::primary_button("configure-download", s.download_button, &p);
    download = if ready {
        download.on_click(cx.listener(|this, _, _, cx| {
            this.controller.start_download();
            cx.notify();
        }))
    } else {
        download.opacity(0.45)
    };
    let mut summary = theme::card(&p).flex().flex_col().gap_3();
    if let Some(image) = preview {
        summary = summary.child(
            div()
                .h(px(168.))
                .w_full()
                .rounded_lg()
                .overflow_hidden()
                .bg(p.track)
                .child(
                    gpui::img(image)
                        .size_full()
                        .object_fit(gpui::ObjectFit::Cover),
                ),
        );
    } else if thumbnail_loading {
        summary = summary.child(
            div()
                .h(px(120.))
                .w_full()
                .rounded_lg()
                .bg(p.track)
                .flex()
                .items_center()
                .justify_center()
                .child(theme::loading(s.loading, &p)),
        );
    } else if thumbnail_error {
        summary = summary.child(theme::notice(
            "icons/alert.svg",
            s.thumbnail_failed,
            p.warning,
        ));
    } else {
        summary = summary.child(
            div()
                .h(px(120.))
                .w_full()
                .rounded_lg()
                .bg(linear_gradient(
                    135.,
                    gpui::linear_color_stop(p.track, 0.0),
                    gpui::linear_color_stop(p.accent.opacity(0.25), 1.0),
                ))
                .flex()
                .items_center()
                .justify_center()
                .child(theme::icon("icons/play.svg", 28., p.muted)),
        );
    }
    summary = summary
        .child(theme::section_title(s.configure_title, &p))
        .child(theme::body(quality_label, &p));
    if !fragment_label.is_empty() {
        summary = summary.child(theme::muted(
            format!("{}: {fragment_label}", s.trim_title),
            &p,
        ));
    }
    summary = summary
        .child(theme::divider(&p))
        .child(theme::section_title(s.estimated_size, &p))
        .child(
            div()
                .text_size(px(20.))
                .font_weight(gpui::FontWeight::SEMIBOLD)
                .text_color(p.text)
                .child(if size > 0 {
                    format_bytes(size, locale)
                } else {
                    "—".to_owned()
                }),
        );
    if is_playlist {
        summary = summary.child(theme::muted(
            s.videos_count.replace("%d", &selected_count.to_string()),
            &p,
        ));
    }
    summary = summary.child(div().w_full().child(download));
    if !app.controller.state.deps.ytrust.available {
        summary = summary.child(theme::body(s.missing_ytrust, &p).text_color(p.danger));
    }

    div()
        .id("page-configure")
        .flex()
        .flex_col()
        .gap_4()
        .h_full()
        .overflow_y_scroll()
        .scrollbar_width(px(8.))
        .pb_5()
        .child(header)
        .child(
            div()
                .flex()
                .flex_wrap()
                .items_start()
                .gap_4()
                .child(div().flex_1().min_w(px(300.)).child(left))
                .child(div().w(px(310.)).min_w(px(280.)).child(summary)),
        )
}

fn advanced_tracks(
    app: &mut VolvidApp,
    cx: &mut Context<VolvidApp>,
    p: &Palette,
    show_subs: bool,
) -> impl IntoElement {
    let s = app.s();
    let audio = app.controller.state.audio_tracks.clone();
    let subs = app.controller.state.sub_tracks.clone();
    let selected_audio = app.controller.state.draft.audio_langs.clone();
    let selected_subs = app.controller.state.draft.sub_langs.clone();
    let loading_tracks = app.controller.state.probe_loading;
    let tracks_error = app.controller.state.tracks_error.clone();
    let mut card = theme::card(p)
        .flex()
        .flex_col()
        .gap_2()
        .child(theme::section_title(
            if show_subs {
                s.advanced
            } else {
                s.audio_tracks_title
            },
            p,
        ));
    if audio.len() > 1 {
        card = card.child(theme::section_title(s.audio_tracks_title, p));
        let mut chips = div().flex().flex_wrap().gap_2();
        for track in &audio {
            let lang = track.lang.clone();
            let checked = selected_audio.contains(&lang);
            chips = chips.child(
                track_chip(
                    SharedString::from(format!("audio-track-{lang}")),
                    lang.clone(),
                    checked,
                    p,
                )
                .on_click(cx.listener(move |this, _, _, cx| {
                    if !this.controller.state.draft.audio_langs.remove(&lang) {
                        this.controller.state.draft.audio_langs.insert(lang.clone());
                    }
                    cx.notify();
                })),
            );
        }
        card = card.child(chips);
    }
    if show_subs && !subs.is_empty() {
        card = card.child(theme::section_title(s.subtitles_title, p));
        let mut chips = div().flex().flex_wrap().gap_2();
        for track in &subs {
            let lang = track.lang.clone();
            let checked = selected_subs.contains(&lang);
            let label = if track.auto {
                format!("{lang} ({})", s.auto_subtitle)
            } else {
                lang.clone()
            };
            chips = chips.child(
                track_chip(
                    SharedString::from(format!("sub-track-{lang}")),
                    label,
                    checked,
                    p,
                )
                .on_click(cx.listener(move |this, _, _, cx| {
                    if !this.controller.state.draft.sub_langs.remove(&lang) {
                        this.controller.state.draft.sub_langs.insert(lang.clone());
                    }
                    cx.notify();
                })),
            );
        }
        card = card.child(chips);
    }
    if loading_tracks {
        card = card.child(theme::loading(s.loading_tracks, p));
    }
    if let Some(error) = tracks_error {
        card = card.child(theme::notice("icons/alert.svg", error, p.warning));
    }
    if !loading_tracks && audio.len() <= 1 {
        card = card.child(theme::muted(s.no_audio_tracks, p));
    }
    if show_subs && !loading_tracks && subs.is_empty() {
        card = card.child(theme::muted(s.no_subtitles, p));
    }
    card
}

fn fragment_card(
    app: &mut VolvidApp,
    window: &mut Window,
    cx: &mut Context<VolvidApp>,
    p: &Palette,
) -> impl IntoElement {
    let s = app.s();
    let locale = app.controller.state.locale;
    let is_playlist = app.controller.state.is_playlist_mode();
    let duration = app.controller.state.media_duration;
    let target = app.controller.state.target.clone();
    let mode = app.controller.state.draft.fragment_mode;
    let can_url = app
        .controller
        .state
        .draft
        .can_use_url_start(target.as_ref(), duration);
    let fragment = app
        .controller
        .state
        .draft
        .fragment(target.as_ref(), duration);
    let error = fragment
        .as_ref()
        .err()
        .map(|err| fragment_error_text(*err, duration, locale));
    let label = fragment
        .as_ref()
        .ok()
        .map(|fragment| volvid_core::format_fragment_label(fragment.as_ref()))
        .unwrap_or_default();

    let mut card = theme::card(p)
        .flex()
        .flex_col()
        .gap_2()
        .child(theme::section_title(s.trim_title, p));
    if is_playlist {
        return card
            .child(theme::notice(
                "icons/info.svg",
                s.trim_playlist_unavailable,
                p.warning,
            ))
            .into_any_element();
    }

    let full = segment("fragment-none", s.full_media, mode == FragmentMode::None, p).on_click(
        cx.listener(|this, _, _, cx| {
            this.controller.state.draft.fragment_mode = FragmentMode::None;
            cx.notify();
        }),
    );
    let from_url = segment(
        "fragment-url",
        s.from_url_timestamp,
        mode == FragmentMode::FromUrl,
        p,
    );
    let from_url = if can_url {
        from_url.on_click(cx.listener(|this, _, _, cx| {
            this.controller.state.draft.fragment_mode = FragmentMode::FromUrl;
            cx.notify();
        }))
    } else {
        from_url.opacity(0.5)
    };
    let custom = segment(
        "fragment-custom",
        s.custom_range,
        mode == FragmentMode::Custom,
        p,
    )
    .on_click(cx.listener(|this, _, _, cx| {
        this.controller.state.draft.fragment_mode = FragmentMode::Custom;
        cx.notify();
    }));

    card = card.child(
        div()
            .flex()
            .flex_wrap()
            .items_center()
            .gap_2()
            .child(full)
            .child(from_url)
            .child(custom),
    );

    if mode == FragmentMode::Custom {
        let start = app.fragment_start_field(window, cx, p);
        let end = app.fragment_end_field(window, cx, p);
        card = card.child(
            div()
                .flex()
                .items_center()
                .gap_2()
                .child(div().w(px(110.)).child(start))
                .child(theme::muted("—", p))
                .child(div().w(px(110.)).child(end))
                .child(theme::muted(s.trim_hint, p)),
        );
    }
    if let Some(error) = error {
        card = card.child(theme::notice("icons/alert.svg", error, p.warning));
    } else if !label.is_empty() {
        card = card.child(theme::muted(format!("{}: {label}", s.trim_title), p));
    }
    card.into_any_element()
}

fn estimate_size(app: &VolvidApp) -> i64 {
    if app.controller.state.draft.mode != DownloadMode::Video {
        return 0;
    }
    let size = app
        .controller
        .state
        .draft
        .quality
        .as_ref()
        .map(|quality| quality.size_bytes)
        .unwrap_or(0);
    if app.controller.state.is_playlist_mode() {
        size.saturating_mul(app.controller.state.selected_entries.len().max(1) as i64)
    } else {
        size
    }
}
