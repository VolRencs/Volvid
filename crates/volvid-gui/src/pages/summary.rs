use std::path::PathBuf;

use eframe::egui;
use egui::{Align, Layout, RichText};
use volvid_app::Route;
use volvid_core::DownloadMode;

use crate::app::VolvidApp;
use crate::format;
use crate::theme::{self, primary_button, secondary_button};

enum SummaryAction {
    New,
    Clear,
    Open(PathBuf),
}

pub fn show(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    let state = &app.controller.state;
    let ok = state.history.iter().filter(|item| item.ok).count();
    let failed = state.history.len().saturating_sub(ok);
    let mut action = None;

    ui.horizontal(|ui| {
        ui.vertical(|ui| {
            theme::title(ui, p, s.history_title);
            theme::muted(ui, p, s.session_history);
        });
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            if primary_button(ui, p, s.nav_new).clicked() {
                action = Some(SummaryAction::New);
            }
            if !state.history.is_empty() && secondary_button(ui, p, s.clear_history).clicked() {
                action = Some(SummaryAction::Clear);
            }
        });
    });
    ui.add_space(16.0);

    theme::card_frame(p).show(ui, |ui| {
        ui.set_min_width(ui.available_width());
        ui.horizontal(|ui| {
            stat_block(ui, p, s.completed, ok, p.success);
            ui.add_space(20.0);
            stat_block(ui, p, s.failed, failed, p.danger);
            ui.add_space(20.0);
            stat_block(ui, p, s.session_history, state.history.len(), p.accent);
        });
    });
    ui.add_space(14.0);

    if state.history.is_empty() {
        theme::card_frame(p).show(ui, |ui| {
            ui.set_min_width(ui.available_width());
            ui.vertical_centered(|ui| {
                ui.add_space(20.0);
                theme::muted(ui, p, s.no_history);
                ui.add_space(20.0);
            });
        });
    } else {
        for item in state.history.iter().rev().take(50) {
            let color = if item.ok { p.success } else { p.danger };
            let timestamp = format::timestamp(item.timestamp, state.locale);
            let open_width =
                theme::text_width(ui, s.open_folder, theme::semibold(theme::FONT_LABEL)) + 30.0;
            let timestamp_width = theme::text_width(
                ui,
                &timestamp,
                egui::FontId::proportional(theme::FONT_SMALL),
            );
            let actions_width = open_width + 8.0 + timestamp_width + 8.0;
            theme::card(ui, p, |ui| {
                ui.horizontal(|ui| {
                    theme::status_dot(ui, color);
                    theme::row_with_actions(
                        ui,
                        actions_width,
                        |ui| {
                            ui.add(
                                egui::Label::new(
                                    theme::strong_text(&item.label, theme::FONT_BODY).color(p.text),
                                )
                                .truncate(),
                            );
                            ui.add(
                                egui::Label::new(
                                    RichText::new(&item.url)
                                        .size(theme::FONT_SMALL)
                                        .color(p.muted),
                                )
                                .truncate(),
                            );
                        },
                        |ui| {
                            if secondary_button(ui, p, s.open_folder).clicked() {
                                action = Some(SummaryAction::Open(item.output_dir.clone()));
                            }
                            ui.add(
                                egui::Label::new(
                                    RichText::new(&timestamp)
                                        .size(theme::FONT_SMALL)
                                        .color(p.muted),
                                )
                                .truncate(),
                            );
                        },
                    );
                });
                ui.add_space(6.0);
                ui.horizontal(|ui| {
                    let mode = match item.mode {
                        DownloadMode::Video => s.mode_video,
                        DownloadMode::Audio => s.mode_audio,
                        DownloadMode::Thumbnail => s.mode_thumbnail,
                    };
                    theme::badge(ui, p, mode, p.accent);
                    if item.items > 1 {
                        let mut details = vec![s.job_items.replace("%d", &item.items.to_string())];
                        if item.failed > 0 {
                            details.push(s.failed_items.replace("%d", &item.failed.to_string()));
                        }
                        theme::muted(ui, p, details.join("  ·  "));
                    }
                });
            });
            ui.add_space(10.0);
        }
    }

    // The action target is resolved after rendering to keep the history borrow
    // narrow and avoid cloning the whole list every frame.
    match action {
        Some(SummaryAction::New) => app.controller.set_route(Route::Home),
        Some(SummaryAction::Clear) => app.controller.clear_history(),
        Some(SummaryAction::Open(path)) => app.controller.open_folder(path),
        None => {}
    }
}

fn stat_block(
    ui: &mut egui::Ui,
    p: &crate::theme::Palette,
    label: &str,
    value: usize,
    color: egui::Color32,
) {
    ui.vertical(|ui| {
        ui.label(theme::strong_text(value.to_string(), 24.0).color(color));
        theme::muted(ui, p, label);
    });
}
