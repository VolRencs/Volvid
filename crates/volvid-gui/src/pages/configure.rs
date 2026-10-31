use eframe::egui;
use egui::{Align, Layout, RichText, Sense, Stroke};
use volvid_app::{FragmentMode, Route};
use volvid_core::{DownloadMode, format_clock, format_duration};
use volvid_i18n::{fragment_error_text, quality_label};

use crate::app::VolvidApp;
use crate::theme::{self, primary_button_fill, secondary_button};

pub fn show(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    let title = app.controller.state.title_for_configure();
    let duration = app.controller.state.media_duration;
    let is_playlist = app
        .controller
        .state
        .target
        .as_ref()
        .is_some_and(|target| target.is_playlist() && !app.controller.state.force_single);

    ui.horizontal(|ui| {
        if theme::icon_button(
            ui,
            p,
            egui::include_image!("../../../../assets/icons/arrow-left.svg"),
            42.0,
            p.text,
            s.back,
        )
        .clicked()
        {
            let route = if is_playlist {
                Route::Playlist
            } else {
                Route::Home
            };
            app.controller.set_route(route);
        }
        ui.add_space(4.0);
        ui.vertical(|ui| {
            let title = if title.is_empty() {
                s.configure_title
            } else {
                title.as_str()
            };
            theme::page_title(ui, p, title);
            let mut meta = Vec::new();
            if duration > 0 {
                meta.push(format!("{} {}", s.duration, format_duration(duration)));
            }
            if is_playlist {
                let count = app.controller.state.selected_entries.len();
                meta.push(s.videos_count.replace("%d", &count.to_string()));
            }
            if !meta.is_empty() {
                theme::muted(ui, p, meta.join("  ·  "));
            }
        });
    });
    ui.add_space(16.0);

    let available = ui.available_width();
    if let Some((left_width, right_width)) =
        theme::split_columns(available, 18.0, 0.62, 420.0, 320.0)
    {
        ui.horizontal_top(|ui| {
            ui.spacing_mut().item_spacing.x = 18.0;
            ui.allocate_ui_with_layout(
                egui::vec2(left_width, 0.0),
                Layout::top_down(Align::LEFT),
                |ui| options(app, ui),
            );
            ui.allocate_ui_with_layout(
                egui::vec2(right_width, 0.0),
                Layout::top_down(Align::LEFT),
                |ui| summary(app, ui),
            );
        });
    } else {
        options(app, ui);
        ui.add_space(14.0);
        summary(app, ui);
    }
}

fn options(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    let locale = app.controller.state.locale;

    ui.horizontal(|ui| {
        for (mode, label) in [
            (DownloadMode::Video, s.mode_video),
            (DownloadMode::Audio, s.mode_audio),
            (DownloadMode::Thumbnail, s.mode_thumbnail),
        ] {
            let selected = app.controller.state.draft.mode == mode;
            if theme::segmented_button(ui, p, label, selected, 110.0).clicked() {
                app.controller.set_mode(mode);
            }
        }
    });
    ui.add_space(12.0);

    match app.controller.state.draft.mode {
        DownloadMode::Video => {
            theme::card(ui, p, |ui| {
                ui.horizontal(|ui| {
                    theme::section_title(ui, p, s.quality_title);
                    if app.controller.state.quality_loading {
                        ui.add(egui::Spinner::new().size(14.0).color(p.accent));
                        theme::muted(ui, p, s.loading_quality);
                    }
                });
                ui.add_space(6.0);
                let choices = app.controller.state.quality_choices.clone();
                for choice in choices {
                    let selected =
                        app.controller
                            .state
                            .draft
                            .quality
                            .as_ref()
                            .is_some_and(|current| {
                                current.height == choice.height
                                    && current.best == choice.best
                                    && current.worst == choice.worst
                            });
                    let label = quality_label(&choice, locale);
                    if option_row(ui, p, selected, &label).clicked() {
                        app.controller.state.draft.quality = Some(choice);
                        app.controller.state.draft.video_profile_index = 0;
                    }
                }
                if let Some(error) = app.controller.state.quality_error.clone() {
                    ui.add(
                        egui::Label::new(
                            RichText::new(error)
                                .color(p.warning)
                                .size(theme::FONT_SMALL),
                        )
                        .wrap(),
                    );
                }
            });
            ui.add_space(12.0);
            theme::card(ui, p, |ui| {
                theme::section_title(ui, p, s.format_title);
                ui.add_space(4.0);
                let profiles = app.controller.state.draft.video_profiles(locale);
                let selected = app.controller.state.draft.video_profile_index;
                for (index, profile) in profiles.iter().enumerate() {
                    if option_row(ui, p, selected == index, &profile.label).clicked() {
                        app.controller.state.draft.video_profile_index = index;
                    }
                }
            });
            ui.add_space(12.0);
            advanced(app, ui);
        }
        DownloadMode::Audio => {
            theme::card(ui, p, |ui| {
                theme::section_title(ui, p, s.audio_title);
                ui.add_space(4.0);
                let profiles = app.controller.state.draft.audio_profiles(locale);
                let selected = app.controller.state.draft.audio_profile_index;
                for (index, profile) in profiles.iter().enumerate() {
                    if option_row(ui, p, selected == index, &profile.label).clicked() {
                        app.controller.state.draft.audio_profile_index = index;
                    }
                }
            });
            ui.add_space(12.0);
            fragment(app, ui);
        }
        DownloadMode::Thumbnail => {
            theme::card(ui, p, |ui| {
                theme::section_title(ui, p, s.thumbnail_title);
                ui.add_space(4.0);
                theme::muted(ui, p, s.thumbnail_hint);
            });
        }
    }

    ui.add_space(theme::CARD_GAP);
    theme::card(ui, p, |ui| {
        ui.horizontal(|ui| {
            theme::section_title(ui, p, s.output_folder);
            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                if secondary_button(ui, p, s.open_folder).clicked() {
                    app.controller
                        .open_folder(app.controller.state.downloads_dir.clone());
                }
                if secondary_button(ui, p, s.choose_folder).clicked() {
                    app.controller.pick_folder();
                }
            });
        });
        ui.add_space(4.0);
        let path = app.controller.state.downloads_dir.to_string_lossy();
        theme::text_chip(ui, p, &path, 12.5, ui.available_width(), false);
        let is_playlist = app
            .controller
            .state
            .target
            .as_ref()
            .is_some_and(|target| target.is_playlist() && !app.controller.state.force_single);
        if is_playlist {
            ui.add_space(8.0);
            ui.horizontal(|ui| {
                theme::section_title(ui, p, s.workers);
                ui.with_layout(egui::Layout::right_to_left(Align::Center), |ui| {
                    theme::value_box(ui, p, app.controller.state.draft.workers.to_string());
                    theme::slider(ui, p, &mut app.controller.state.draft.workers, 1..=8, 200.0);
                });
            });
        }
    });
}

fn advanced(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    theme::card(ui, p, |ui| {
        egui::CollapsingHeader::new(theme::strong_text(s.advanced, 14.0).color(p.text))
            .default_open(true)
            .show(ui, |ui| {
                if app.controller.state.probe_loading {
                    theme::spinner(ui, p, s.loading_tracks);
                }
                let audio = app.controller.state.audio_tracks.clone();
                theme::section_title(ui, p, s.audio_tracks_title);
                if audio.len() < 2 {
                    theme::muted(ui, p, s.no_audio_tracks);
                } else {
                    for track in audio {
                        let mut checked =
                            app.controller.state.draft.audio_langs.contains(&track.lang);
                        if ui.checkbox(&mut checked, &track.lang).changed() {
                            if checked {
                                app.controller.state.draft.audio_langs.insert(track.lang);
                            } else {
                                app.controller.state.draft.audio_langs.remove(&track.lang);
                            }
                        }
                    }
                    theme::muted(ui, p, s.audio_original_hint);
                }
                ui.add_space(theme::SECTION_GAP);
                theme::section_title(ui, p, s.subtitles_title);
                let subs = app.controller.state.sub_tracks.clone();
                if subs.is_empty() {
                    theme::muted(ui, p, s.no_subtitles);
                } else {
                    for track in subs {
                        let label = if track.auto {
                            format!("{} ({})", track.lang, s.auto_subtitle)
                        } else {
                            track.lang.clone()
                        };
                        let mut checked =
                            app.controller.state.draft.sub_langs.contains(&track.lang);
                        if ui.checkbox(&mut checked, label).changed() {
                            if checked {
                                app.controller.state.draft.sub_langs.insert(track.lang);
                            } else {
                                app.controller.state.draft.sub_langs.remove(&track.lang);
                            }
                        }
                    }
                }
                if let Some(error) = app.controller.state.tracks_error.clone() {
                    ui.label(RichText::new(error).color(p.warning).size(12.0));
                }
            });
    });
    ui.add_space(8.0);
    fragment(app, ui);
}

fn fragment(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    let target = app.controller.state.target.clone();
    let duration = app.controller.state.media_duration;
    if target
        .as_ref()
        .is_some_and(|target| target.is_playlist() && !app.controller.state.force_single)
    {
        theme::muted(ui, p, s.trim_playlist_unavailable);
        return;
    }
    let mut enabled = app.controller.state.draft.fragment_mode != FragmentMode::None;
    theme::card(ui, p, |ui| {
        if ui
            .checkbox(&mut enabled, theme::strong_text(s.trim_enable, 14.0))
            .changed()
        {
            app.controller.state.draft.fragment_mode = if enabled {
                FragmentMode::Custom
            } else {
                FragmentMode::None
            };
        }
        if !enabled {
            return;
        }
        if app
            .controller
            .state
            .draft
            .can_use_url_start(target.as_ref(), duration)
        {
            ui.radio_value(
                &mut app.controller.state.draft.fragment_mode,
                FragmentMode::None,
                s.full_media,
            );
            ui.radio_value(
                &mut app.controller.state.draft.fragment_mode,
                FragmentMode::FromUrl,
                format!(
                    "{} ({})",
                    s.from_url_timestamp,
                    format_clock(
                        target
                            .as_ref()
                            .map(|target| target.url_start_at)
                            .unwrap_or(0)
                    )
                ),
            );
        }
        ui.radio_value(
            &mut app.controller.state.draft.fragment_mode,
            FragmentMode::Custom,
            s.custom_range,
        );
        if app.controller.state.draft.fragment_mode == FragmentMode::Custom {
            ui.horizontal(|ui| {
                theme::text_input(
                    ui,
                    &mut app.controller.state.draft.custom_start,
                    "1:00",
                    120.0,
                );
                ui.label("—");
                theme::text_input(
                    ui,
                    &mut app.controller.state.draft.custom_end,
                    "2:30",
                    120.0,
                );
            });
            ui.add_space(4.0);
            theme::muted(ui, p, s.trim_hint);
        }
        if let Err(error) = app
            .controller
            .state
            .draft
            .fragment(target.as_ref(), duration)
        {
            ui.add(
                egui::Label::new(
                    RichText::new(fragment_error_text(
                        error,
                        duration,
                        app.controller.state.locale,
                    ))
                    .color(p.danger)
                    .size(theme::FONT_SMALL),
                )
                .wrap(),
            );
        }
    });
}

fn summary(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    let locale = app.controller.state.locale;
    let preview = app.preview_texture().cloned();
    let thumbnail_loading = app.controller.state.thumbnail_loading;
    let thumbnail_error = app.controller.state.thumbnail_error;
    theme::card(ui, p, |ui| {
        let width = ui.available_width();
        if let Some(texture) = preview {
            let size = texture.size_vec2();
            let aspect = size.x.max(1.0) / size.y.max(1.0);
            let mut height = width / aspect.max(0.01);
            if height > 220.0 {
                height = 220.0;
            }
            let image_width = (height * aspect).min(width);
            ui.allocate_ui_with_layout(
                egui::vec2(width, height),
                Layout::left_to_right(Align::Center),
                |ui| {
                    ui.add(
                        egui::Image::new((texture.id(), egui::vec2(image_width, height)))
                            .corner_radius(egui::CornerRadius::same(12)),
                    );
                },
            );
            ui.add_space(12.0);
        } else {
            egui::Frame::new()
                .fill(p.track)
                .stroke(Stroke::new(1.0, p.border))
                .corner_radius(egui::CornerRadius::same(12))
                .inner_margin(egui::Margin::same(0))
                .show(ui, |ui| {
                    ui.allocate_ui_with_layout(
                        egui::vec2(width, 132.0),
                        Layout::centered_and_justified(egui::Direction::TopDown),
                        |ui| {
                            if app.controller.state.probe_loading || thumbnail_loading {
                                ui.add(egui::Spinner::new().size(24.0).color(p.accent));
                            } else {
                                ui.label(
                                    RichText::new("▶")
                                        .size(34.0)
                                        .color(p.muted.gamma_multiply(0.7)),
                                );
                            }
                        },
                    );
                });
            ui.add_space(12.0);
        }
        if thumbnail_error {
            ui.add(
                egui::Label::new(
                    RichText::new(s.thumbnail_failed)
                        .color(p.warning)
                        .size(theme::FONT_SMALL),
                )
                .wrap(),
            );
            ui.add_space(6.0);
        }
        theme::section_title(ui, p, s.configure_title);
        ui.add_space(6.0);
        let profile = app.controller.state.draft.profile(locale);
        ui.add(egui::Label::new(theme::strong_text(&profile.label, 16.0).color(p.text)).wrap());
        ui.add_space(4.0);
        let target = app.controller.state.target.clone();
        let fragment = app
            .controller
            .state
            .draft
            .fragment_label(target.as_ref(), app.controller.state.media_duration);
        if !fragment.is_empty() {
            theme::muted(ui, p, format!("{}: {fragment}", s.trim_title));
        }
        ui.add_space(10.0);
        ui.separator();
        ui.add_space(10.0);
        theme::section_title(ui, p, s.estimated_size);
        let size = estimate_size(app);
        ui.label(
            theme::strong_text(
                if size > 0 {
                    volvid_i18n::format_bytes(size, locale)
                } else {
                    "—".to_owned()
                },
                20.0,
            )
            .color(p.text),
        );
        ui.add_space(4.0);
        let is_playlist = target
            .as_ref()
            .is_some_and(|target| target.is_playlist() && !app.controller.state.force_single);
        if is_playlist {
            theme::muted(
                ui,
                p,
                s.videos_count.replace(
                    "%d",
                    &app.controller.state.selected_entries.len().to_string(),
                ),
            );
        }
        ui.add_space(14.0);
        let width = ui.available_width();
        if primary_button_fill(ui, p, s.download_button, width).clicked() {
            app.controller.start_download();
        }
        if app.controller.state.quality_loading || app.controller.state.probe_loading {
            ui.add_space(8.0);
            theme::spinner(ui, p, s.loading);
        }
        if !app.controller.state.deps.ytdlp.available {
            ui.add_space(6.0);
            ui.add(
                egui::Label::new(
                    RichText::new(s.missing_ytdlp)
                        .color(p.danger)
                        .size(theme::FONT_SMALL),
                )
                .wrap(),
            );
        }
    });
}

fn option_row(
    ui: &mut egui::Ui,
    p: &crate::theme::Palette,
    selected: bool,
    text: &str,
) -> egui::Response {
    let frame = egui::Frame::new()
        .fill(if selected {
            p.selection.gamma_multiply(0.12)
        } else {
            p.card_hover
        })
        .stroke(Stroke::new(
            1.0,
            if selected {
                p.selection.gamma_multiply(0.7)
            } else {
                p.border
            },
        ))
        .corner_radius(egui::CornerRadius::same(9))
        .inner_margin(egui::Margin::symmetric(12, 10))
        .show(ui, |ui| {
            ui.set_min_width(ui.available_width());
            ui.horizontal(|ui| {
                theme::status_dot(ui, if selected { p.selection } else { p.border });
                ui.add(
                    egui::Label::new(
                        RichText::new(text)
                            .color(if selected { p.selection } else { p.text })
                            .size(13.5),
                    )
                    .truncate(),
                );
            });
        });
    frame
        .response
        .interact(Sense::click())
        .on_hover_cursor(egui::CursorIcon::PointingHand)
}

fn estimate_size(app: &VolvidApp) -> i64 {
    match app.controller.state.draft.mode {
        DownloadMode::Video => {
            let size = app
                .controller
                .state
                .draft
                .quality
                .as_ref()
                .map(|quality| quality.size_bytes)
                .unwrap_or(0);
            let is_playlist =
                app.controller.state.target.as_ref().is_some_and(|target| {
                    target.is_playlist() && !app.controller.state.force_single
                });
            if is_playlist {
                size.saturating_mul(app.controller.state.selected_entries.len().max(1) as i64)
            } else {
                size
            }
        }
        _ => 0,
    }
}
