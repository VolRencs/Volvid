use eframe::egui;
use egui::{Align, Layout, RichText, Stroke};
use volvid_app::{FragmentMode, Route};
use volvid_core::{DownloadMode, format_clock, format_duration};
use volvid_i18n::{fragment_error_text, quality_label};

use crate::app::VolvidApp;
use crate::theme::{self, primary_button, secondary_button};

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
        if secondary_button(ui, p, "←").clicked() {
            let route = if is_playlist {
                Route::Playlist
            } else {
                Route::Home
            };
            app.controller.set_route(route);
        }
        ui.vertical(|ui| {
            ui.label(
                RichText::new(if title.is_empty() {
                    s.configure_title.to_owned()
                } else {
                    title.clone()
                })
                .size(36.0)
                .strong()
                .color(p.text),
            );
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
    let left_width = (available * 0.62).max(420.0);
    let right_width = (available - left_width - 18.0).max(280.0);
    ui.horizontal_top(|ui| {
        ui.allocate_ui_with_layout(
            egui::vec2(left_width, 0.0),
            Layout::top_down(Align::LEFT),
            |ui| options(app, ui),
        );
        ui.add_space(18.0);
        ui.allocate_ui_with_layout(
            egui::vec2(right_width, 0.0),
            Layout::top_down(Align::LEFT),
            |ui| summary(app, ui),
        );
    });
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
            let button = egui::Button::new(
                RichText::new(label)
                    .color(if selected {
                        egui::Color32::WHITE
                    } else {
                        p.text
                    })
                    .strong(),
            )
            .fill(if selected { p.accent } else { p.card })
            .stroke(Stroke::new(1.0, if selected { p.accent } else { p.border }))
            .corner_radius(egui::CornerRadius::same(9))
            .min_size(egui::vec2(110.0, 38.0));
            if ui.add(button).clicked() {
                app.controller.set_mode(mode);
            }
        }
    });
    ui.add_space(12.0);

    match app.controller.state.draft.mode {
        DownloadMode::Video => {
            theme::card_frame(p).show(ui, |ui| {
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
                    if ui
                        .selectable_label(selected, RichText::new(label).size(14.0))
                        .clicked()
                    {
                        app.controller.state.draft.quality = Some(choice);
                        app.controller.state.draft.video_profile_index = 0;
                    }
                }
                if let Some(error) = app.controller.state.quality_error.clone() {
                    ui.label(RichText::new(error).color(p.warning).size(12.0));
                }
            });
            ui.add_space(12.0);
            theme::card_frame(p).show(ui, |ui| {
                theme::section_title(ui, p, s.format_title);
                ui.add_space(4.0);
                let profiles = app.controller.state.draft.video_profiles(locale);
                let selected = app.controller.state.draft.video_profile_index;
                for (index, profile) in profiles.iter().enumerate() {
                    if ui
                        .selectable_label(
                            selected == index,
                            RichText::new(&profile.label).size(14.0),
                        )
                        .clicked()
                    {
                        app.controller.state.draft.video_profile_index = index;
                    }
                }
            });
            ui.add_space(12.0);
            advanced(app, ui);
        }
        DownloadMode::Audio => {
            theme::card_frame(p).show(ui, |ui| {
                theme::section_title(ui, p, s.audio_title);
                ui.add_space(4.0);
                let profiles = app.controller.state.draft.audio_profiles(locale);
                let selected = app.controller.state.draft.audio_profile_index;
                for (index, profile) in profiles.iter().enumerate() {
                    if ui
                        .selectable_label(
                            selected == index,
                            RichText::new(&profile.label).size(14.0),
                        )
                        .clicked()
                    {
                        app.controller.state.draft.audio_profile_index = index;
                    }
                }
            });
            ui.add_space(12.0);
            fragment(app, ui);
        }
        DownloadMode::Thumbnail => {
            theme::card_frame(p).show(ui, |ui| {
                theme::section_title(ui, p, s.thumbnail_title);
                ui.add_space(4.0);
                theme::muted(ui, p, "YouTube thumbnail in the best available resolution.");
            });
        }
    }

    ui.add_space(12.0);
    theme::card_frame(p).show(ui, |ui| {
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
        ui.label(
            RichText::new(app.controller.state.downloads_dir.to_string_lossy())
                .size(12.5)
                .family(egui::FontFamily::Monospace)
                .color(p.muted),
        );
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
                ui.add(
                    egui::Slider::new(&mut app.controller.state.draft.workers, 1..=8)
                        .show_value(true),
                );
            });
        }
    });
}

fn advanced(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    theme::card_frame(p).show(ui, |ui| {
        egui::CollapsingHeader::new(RichText::new(s.advanced).strong().color(p.text))
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
                ui.add_space(8.0);
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
    let mut enabled = app.controller.state.draft.fragment_mode != FragmentMode::None;
    theme::card_frame(p).show(ui, |ui| {
        if ui
            .checkbox(&mut enabled, RichText::new(s.trim_enable).strong())
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
                ui.add_sized(
                    [120.0, 34.0],
                    egui::TextEdit::singleline(&mut app.controller.state.draft.custom_start)
                        .hint_text("1:00"),
                );
                ui.label("—");
                ui.add_sized(
                    [120.0, 34.0],
                    egui::TextEdit::singleline(&mut app.controller.state.draft.custom_end)
                        .hint_text("2:30"),
                );
                theme::muted(ui, p, s.trim_hint);
            });
        }
        if let Err(error) = app
            .controller
            .state
            .draft
            .fragment(target.as_ref(), duration)
        {
            ui.label(
                RichText::new(fragment_error_text(
                    error,
                    duration,
                    app.controller.state.locale,
                ))
                .color(p.danger)
                .size(12.0),
            );
        }
    });
}

fn summary(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    let locale = app.controller.state.locale;
    let preview = app.preview_texture().cloned();
    theme::card_frame(p).show(ui, |ui| {
        let width = ui.available_width();
        if let Some(texture) = preview {
            let aspect = texture.size_vec2().y / texture.size_vec2().x.max(1.0);
            let height = (width * aspect).clamp(120.0, 220.0);
            ui.add(
                egui::Image::new((texture.id(), egui::vec2(width, height)))
                    .corner_radius(egui::CornerRadius::same(12)),
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
                            if app.controller.state.probe_loading {
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
        theme::section_title(ui, p, s.configure_title);
        ui.add_space(6.0);
        let profile = app.controller.state.draft.profile(locale);
        ui.label(
            RichText::new(&profile.label)
                .size(16.0)
                .strong()
                .color(p.text),
        );
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
            RichText::new(if size > 0 {
                volvid_i18n::format_bytes(size, locale)
            } else {
                "—".to_owned()
            })
            .size(20.0)
            .strong()
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
        if primary_button(ui, p, s.download_button).clicked() {
            app.controller.start_download();
        }
        if app.controller.state.quality_loading || app.controller.state.probe_loading {
            ui.add_space(8.0);
            theme::spinner(ui, p, s.loading);
        }
        if !app.controller.state.deps.ytdlp.available {
            ui.add_space(6.0);
            ui.label(RichText::new(s.missing_ytdlp).color(p.danger).size(12.0));
        }
    });
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
                size * app.controller.state.selected_entries.len().max(1) as i64
            } else {
                size
            }
        }
        _ => 0,
    }
}
