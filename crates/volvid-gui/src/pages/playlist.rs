use eframe::egui;
use egui::{Align, Layout, RichText, Sense, Stroke};
use volvid_app::Route;
use volvid_core::format_duration;

use crate::app::VolvidApp;
use crate::theme::{self, primary_button, primary_button_sized, secondary_button};

pub fn show(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    let loading = app.controller.state.playlist_loading;
    let playlist_title = app.controller.state.localize_playlist_title();
    let total = app.controller.state.playlist_entries().len();
    let selected = app.controller.state.selected_entries.len();
    let mut back = false;

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
            back = true;
        }
        ui.add_space(4.0);
        ui.vertical(|ui| {
            theme::title(
                ui,
                p,
                if playlist_title.is_empty() {
                    s.playlist_title
                } else {
                    &playlist_title
                },
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

    let placeholder_height = (ui.ctx().content_rect().height() - 320.0).clamp(320.0, 500.0);
    if loading {
        theme::card_frame(p).show(ui, |ui| {
            ui.set_min_width(ui.available_width());
            ui.set_min_height(placeholder_height);
            ui.vertical_centered(|ui| {
                ui.add_space(placeholder_height / 2.0 - 20.0);
                theme::centered_spinner(ui, p, s.loading_playlist);
            });
        });
        if back {
            app.controller.set_route(Route::Home);
        }
        return;
    }

    if app.controller.state.playlist.is_none() {
        theme::card_frame(p).show(ui, |ui| {
            ui.set_min_width(ui.available_width());
            ui.set_min_height(placeholder_height);
            ui.vertical_centered(|ui| {
                ui.add_space(placeholder_height / 2.0 - 10.0);
                theme::muted(ui, p, s.playlist_empty);
            });
        });
        if back {
            app.controller.set_route(Route::Home);
        }
        return;
    }

    let content_height = ui.ctx().content_rect().height();
    let list_height = (content_height - 520.0).clamp(180.0, 340.0);
    let min_height = (content_height - 320.0).clamp(390.0, 500.0);
    let reset_scroll = app.scroll_reset || app.controller.state.playlist_scroll_reset;
    let session_nonce = app.session_nonce();
    let mut clicked_entry = None;
    let mut select_all = false;
    let mut select_none = false;
    let mut apply_range = false;

    theme::card_frame(p).show(ui, |ui| {
        ui.set_min_width(ui.available_width());
        ui.set_min_height(min_height);
        let wide = ui.available_width() >= 860.0;
        if wide {
            ui.horizontal(|ui| {
                ui.add_sized(
                    [300.0, theme::BUTTON_HEIGHT],
                    egui::TextEdit::singleline(&mut app.controller.state.playlist_filter)
                        .hint_text(s.filter_placeholder),
                );
                if secondary_button(ui, p, s.select_all).clicked() {
                    select_all = true;
                }
                if secondary_button(ui, p, s.select_none).clicked() {
                    select_none = true;
                }
                ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                    if primary_button(ui, p, s.apply_range).clicked() {
                        apply_range = true;
                    }
                    ui.add_sized(
                        [150.0, theme::BUTTON_HEIGHT],
                        egui::TextEdit::singleline(&mut app.controller.state.range_input)
                            .hint_text(s.range_placeholder),
                    );
                });
            });
        } else {
            ui.horizontal_wrapped(|ui| {
                ui.add_sized(
                    [260.0, theme::BUTTON_HEIGHT],
                    egui::TextEdit::singleline(&mut app.controller.state.playlist_filter)
                        .hint_text(s.filter_placeholder),
                );
                if secondary_button(ui, p, s.select_all).clicked() {
                    select_all = true;
                }
                if secondary_button(ui, p, s.select_none).clicked() {
                    select_none = true;
                }
            });
            ui.add_space(6.0);
            ui.horizontal_wrapped(|ui| {
                ui.add_sized(
                    [150.0, theme::BUTTON_HEIGHT],
                    egui::TextEdit::singleline(&mut app.controller.state.range_input)
                        .hint_text(s.range_placeholder),
                );
                if primary_button_sized(ui, p, s.apply_range, 174.0).clicked() {
                    apply_range = true;
                }
            });
        }
        if let Some(error) = app.controller.state.range_error.clone() {
            ui.add_space(4.0);
            ui.label(RichText::new(error).color(p.danger).size(12.0));
        }
        ui.add_space(10.0);
        ui.separator();
        ui.add_space(6.0);

        ui.horizontal(|ui| {
            ui.allocate_ui_with_layout(
                egui::vec2(46.0, 20.0),
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

        let filter = app.controller.state.playlist_filter.trim().to_lowercase();
        let entries = app.controller.state.playlist_entries();
        let filtered: Vec<usize> = entries
            .iter()
            .enumerate()
            .filter(|(_, entry)| filter.is_empty() || entry.title.to_lowercase().contains(&filter))
            .map(|(index, _)| index)
            .collect();
        let selected_entries = &app.controller.state.selected_entries;
        let mut rows = egui::ScrollArea::vertical()
            .id_salt(("playlist-rows", session_nonce, playlist_title.as_str()))
            .max_height(list_height)
            .auto_shrink([false, false]);
        if reset_scroll {
            rows = rows.vertical_scroll_offset(0.0);
        }
        rows.show_rows(ui, 38.0, filtered.len(), |ui, range| {
            for index in range {
                let entry = &entries[filtered[index]];
                let is_selected = selected_entries.contains(&entry.index);
                let fill = if is_selected {
                    p.selection.gamma_multiply(0.12)
                } else {
                    egui::Color32::TRANSPARENT
                };
                let frame = egui::Frame::new()
                    .fill(fill)
                    .stroke(if is_selected {
                        Stroke::new(1.0, p.selection.gamma_multiply(0.35))
                    } else {
                        Stroke::NONE
                    })
                    .corner_radius(egui::CornerRadius::same(8))
                    .inner_margin(egui::Margin::symmetric(6, 6))
                    .show(ui, |ui| {
                        ui.horizontal(|ui| {
                            ui.add_sized(
                                [46.0, 24.0],
                                egui::Label::new(
                                    RichText::new(format!("{:>3}", entry.index))
                                        .color(if is_selected { p.selection } else { p.muted })
                                        .size(12.0),
                                ),
                            );
                            let title_width = (ui.available_width() - 70.0).max(80.0);
                            ui.allocate_ui_with_layout(
                                egui::vec2(title_width, 24.0),
                                Layout::left_to_right(Align::Center),
                                |ui| {
                                    ui.add(
                                        egui::Label::new(
                                            RichText::new(&entry.title).color(p.text).size(14.0),
                                        )
                                        .truncate(),
                                    );
                                },
                            );
                            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                                ui.label(
                                    RichText::new(format_duration(entry.duration))
                                        .size(12.0)
                                        .color(p.muted),
                                );
                            });
                        });
                    });
                let response = frame
                    .response
                    .interact(Sense::click())
                    .on_hover_cursor(egui::CursorIcon::PointingHand);
                if response.clicked() {
                    clicked_entry = Some(entry.index);
                }
            }
        });
    });

    ui.add_space(12.0);
    let selected_now = app.controller.state.selected_entries.len();
    ui.horizontal(|ui| {
        if secondary_button(ui, p, s.back).clicked() {
            back = true;
        }
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            let response = ui.add_enabled_ui(selected_now > 0, |ui| {
                primary_button(ui, p, s.continue_label)
            });
            if response.inner.clicked() {
                app.controller.continue_playlist();
            }
            if selected_now == 0 {
                ui.label(RichText::new(s.no_selection).color(p.warning).size(12.0));
            }
        });
    });

    if select_all {
        app.controller.select_all_playlist();
    }
    if select_none {
        app.controller.clear_playlist();
    }
    if apply_range {
        app.controller.apply_playlist_range();
    }
    if let Some(index) = clicked_entry {
        app.controller.toggle_playlist(index);
    }
    app.controller.state.playlist_scroll_reset = false;
    if back {
        app.controller.set_route(Route::Home);
    }
}
