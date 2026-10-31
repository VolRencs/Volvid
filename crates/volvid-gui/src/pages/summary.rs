use eframe::egui;
use egui::{Align, Layout, RichText};
use volvid_app::Route;
use volvid_core::DownloadMode;

use crate::app::VolvidApp;
use crate::theme::{self, primary_button, secondary_button};

pub fn show(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    let history = app.controller.state.history.clone();
    let ok = history.iter().filter(|item| item.ok).count();
    let failed = history.len().saturating_sub(ok);

    ui.horizontal(|ui| {
        ui.vertical(|ui| {
            ui.label(
                RichText::new(s.history_title)
                    .size(36.0)
                    .strong()
                    .color(p.text),
            );
            theme::muted(ui, p, s.session_history);
        });
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            if primary_button(ui, p, s.nav_new).clicked() {
                app.controller.set_route(Route::Home);
            }
            if !history.is_empty() && secondary_button(ui, p, s.clear_history).clicked() {
                app.controller.clear_history();
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
            stat_block(ui, p, s.session_history, history.len(), p.accent);
        });
    });
    ui.add_space(14.0);

    if history.is_empty() {
        theme::card_frame(p).show(ui, |ui| {
            ui.vertical_centered(|ui| {
                ui.add_space(20.0);
                theme::muted(ui, p, s.no_history);
                ui.add_space(20.0);
            });
        });
        return;
    }

    for item in history.iter().rev().take(50) {
        let color = if item.ok { p.success } else { p.danger };
        theme::card_frame(p).show(ui, |ui| {
            ui.horizontal(|ui| {
                theme::status_dot(ui, color);
                ui.vertical(|ui| {
                    ui.add(
                        egui::Label::new(
                            RichText::new(&item.label).size(15.0).strong().color(p.text),
                        )
                        .truncate(),
                    );
                    ui.add(
                        egui::Label::new(
                            RichText::new(&item.url)
                                .size(11.5)
                                .family(egui::FontFamily::Monospace)
                                .color(p.muted),
                        )
                        .truncate(),
                    );
                });
                ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                    if secondary_button(ui, p, s.open_folder).clicked() {
                        app.controller.open_folder(item.output_dir.clone());
                    }
                    ui.label(
                        RichText::new(format_timestamp(item.timestamp))
                            .size(11.5)
                            .color(p.muted),
                    );
                });
            });
            ui.add_space(5.0);
            ui.horizontal(|ui| {
                let mode = match item.mode {
                    DownloadMode::Video => s.mode_video,
                    DownloadMode::Audio => s.mode_audio,
                    DownloadMode::Thumbnail => s.mode_thumbnail,
                };
                theme::badge(ui, p, mode, p.accent);
                if item.items > 1 {
                    theme::muted(
                        ui,
                        p,
                        format!(
                            "{}  ·  {} {}",
                            s.job_items.replace("%d", &item.items.to_string()),
                            item.failed,
                            s.failed
                        ),
                    );
                }
            });
        });
        ui.add_space(10.0);
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
        ui.label(
            RichText::new(value.to_string())
                .size(24.0)
                .strong()
                .color(color),
        );
        theme::muted(ui, p, label);
    });
}

fn format_timestamp(timestamp: i64) -> String {
    jiff::Timestamp::from_second(timestamp)
        .map(|time| time.strftime("%Y-%m-%d %H:%M").to_string())
        .unwrap_or_default()
}
