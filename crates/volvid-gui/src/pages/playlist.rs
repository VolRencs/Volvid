use eframe::egui;
use egui::{Align, Layout, RichText, Sense};
use volvid_app::Route;
use volvid_core::{PlaylistEntry, format_duration};

use crate::app::VolvidApp;
use crate::theme::{self, primary_button, secondary_button, secondary_button_sized};

pub fn show(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    let loading = app.controller.state.playlist_loading;
    let playlist_title = app.controller.state.localize_playlist_title();
    let total = app.controller.state.playlist_entries().len();
    let selected = app.controller.state.selected_entries.len();

    ui.horizontal(|ui| {
        if secondary_button_sized(ui, p, "←", 42.0).clicked() {
            app.controller.set_route(Route::Home);
        }
        ui.add_space(4.0);
        ui.vertical(|ui| {
            ui.label(
                RichText::new(if playlist_title.is_empty() {
                    s.playlist_title.to_owned()
                } else {
                    playlist_title
                })
                .size(36.0)
                .strong()
                .color(p.text),
            );
            theme::muted(
                ui,
                p,
                format!(
                    "{}  ·  {}",
                    s.videos_count.replace("%d", &total.to_string()),
                    s.selected_count
                        .replacen("%d", &selected.to_string(), 1)
                        .replacen("%d", &total.to_string(), 1)
                ),
            );
        });
    });
    ui.add_space(14.0);

    if loading {
        theme::card_frame(p).show(ui, |ui| {
            ui.set_min_height(300.0);
            theme::spinner(ui, p, s.loading_playlist);
        });
        return;
    }

    if app.controller.state.playlist.is_none() {
        theme::card_frame(p).show(ui, |ui| {
            ui.set_min_height(300.0);
            theme::muted(ui, p, s.playlist_empty);
        });
        return;
    }

    theme::card_frame(p).show(ui, |ui| {
        ui.set_min_height(470.0);
        ui.horizontal(|ui| {
            ui.add_sized(
                [260.0, 34.0],
                egui::TextEdit::singleline(&mut app.controller.state.playlist_filter)
                    .hint_text(s.filter_placeholder),
            );
            if secondary_button(ui, p, s.select_all).clicked() {
                app.controller.select_all_playlist();
            }
            if secondary_button(ui, p, s.select_none).clicked() {
                app.controller.clear_playlist();
            }
            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                if primary_button(ui, p, s.apply_range).clicked() {
                    app.controller.apply_playlist_range();
                }
                ui.add_sized(
                    [150.0, 34.0],
                    egui::TextEdit::singleline(&mut app.controller.state.range_input)
                        .hint_text(s.range_placeholder),
                );
            });
        });
        if let Some(error) = app.controller.state.range_error.clone() {
            ui.add_space(4.0);
            ui.label(RichText::new(error).color(p.danger).size(12.0));
        }
        ui.add_space(10.0);
        ui.separator();
        ui.add_space(6.0);

        ui.horizontal(|ui| {
            ui.allocate_ui_with_layout(
                egui::vec2(42.0, 20.0),
                Layout::left_to_right(Align::Center),
                |ui| {
                    ui.label(RichText::new("#").size(11.5).color(p.muted));
                },
            );
            ui.label(RichText::new(s.table_name).size(11.5).color(p.muted));
            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                ui.label(RichText::new(s.table_duration).size(11.5).color(p.muted));
            });
        });
        ui.add_space(4.0);
        ui.separator();

        let filter = app
            .controller
            .state
            .playlist_filter
            .trim()
            .to_ascii_lowercase();
        let filtered: Vec<PlaylistEntry> = app
            .controller
            .state
            .playlist_entries()
            .iter()
            .filter(|entry| filter.is_empty() || entry.title.to_ascii_lowercase().contains(&filter))
            .cloned()
            .collect();
        let selected_entries = app.controller.state.selected_entries.clone();
        egui::ScrollArea::vertical()
            .id_salt("playlist-rows")
            .max_height(340.0)
            .auto_shrink([false, false])
            .show_rows(ui, 38.0, filtered.len(), |ui, range| {
                for index in range {
                    let entry = &filtered[index];
                    let is_selected = selected_entries.contains(&entry.index);
                    let fill = if is_selected {
                        p.selection.gamma_multiply(0.12)
                    } else {
                        egui::Color32::TRANSPARENT
                    };
                    let frame = egui::Frame::new()
                        .fill(fill)
                        .corner_radius(egui::CornerRadius::same(8))
                        .inner_margin(egui::Margin::symmetric(6, 6))
                        .show(ui, |ui| {
                            ui.horizontal(|ui| {
                                ui.add_sized(
                                    [42.0, 24.0],
                                    egui::Label::new(
                                        RichText::new(format!("{:>3}", entry.index))
                                            .color(if is_selected { p.selection } else { p.muted })
                                            .monospace(),
                                    ),
                                );
                                let title_width = (ui.available_width() - 70.0).max(80.0);
                                ui.add_sized(
                                    [title_width, 24.0],
                                    egui::Label::new(
                                        RichText::new(&entry.title).color(p.text).size(14.0),
                                    )
                                    .truncate(),
                                );
                                ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                                    ui.label(
                                        RichText::new(format_duration(entry.duration))
                                            .monospace()
                                            .size(12.0)
                                            .color(p.muted),
                                    );
                                });
                            });
                        });
                    let response = frame.response.interact(Sense::click());
                    if response.clicked() {
                        app.controller.toggle_playlist(entry.index);
                    }
                    if response.hovered() {
                        ui.ctx().set_cursor_icon(egui::CursorIcon::PointingHand);
                    }
                }
            });
    });

    ui.add_space(12.0);
    let selected_now = app.controller.state.selected_entries.len();
    ui.horizontal(|ui| {
        if secondary_button(ui, p, s.back).clicked() {
            app.controller.set_route(Route::Home);
        }
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            ui.add_enabled_ui(selected_now > 0, |ui| {
                if primary_button(ui, p, s.continue_label).clicked() {
                    app.controller.continue_playlist();
                }
            });
            if selected_now == 0 {
                ui.label(RichText::new(s.no_selection).color(p.warning).size(12.0));
            }
        });
    });
}
