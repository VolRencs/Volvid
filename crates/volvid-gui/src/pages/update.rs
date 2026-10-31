use eframe::egui;
use egui::RichText;

use crate::app::VolvidApp;
use crate::theme::{self, primary_button_sized, secondary_button_sized};

pub fn show(app: &mut VolvidApp, ctx: &egui::Context) {
    let p = app.palette();
    let s = app.s();
    let Some(update) = app.controller.state.update.clone() else {
        app.controller.state.update_modal = false;
        return;
    };
    let updating = app.controller.state.updating;
    let progress = app.controller.state.update_progress.clone();
    theme::modal_backdrop(ctx);
    let mut close = false;

    egui::Window::new("volvid-update-dialog")
        .title_bar(false)
        .collapsible(false)
        .resizable(false)
        .fixed_size(egui::vec2(560.0, 380.0))
        .frame(theme::modal_frame(p))
        .order(egui::Order::Foreground)
        .anchor(egui::Align2::CENTER_CENTER, [0.0, 0.0])
        .show(ctx, |ui| {
            ui.set_width(512.0);
            theme::modal_header(
                ui,
                p,
                egui::include_image!("../../../../assets/icons/new-download.svg"),
                s.update_available,
                &format!("{} → {}", s.current_version, update.latest),
            );
            ui.add_space(16.0);
            if updating {
                theme::spinner(ui, p, s.checking);
                if let Some(progress) = &progress {
                    ui.add_space(8.0);
                    theme::progress_bar(ui, p, (progress.pct / 100.0) as f32, 10.0);
                    ui.label(
                        RichText::new(format!(
                            "{:.1}%  ·  {}",
                            progress.pct,
                            volvid_i18n::format_bytes(progress.done_b, app.controller.state.locale)
                        ))
                        .color(p.muted)
                        .size(12.0),
                    );
                }
            } else {
                ui.horizontal(|ui| {
                    if primary_button_sized(ui, p, s.install_update, 230.0).clicked() {
                        app.controller.apply_update();
                    }
                    if secondary_button_sized(ui, p, s.later, 230.0).clicked() {
                        close = true;
                    }
                });
            }
            ui.add_space(14.0);
            ui.separator();
            ui.add_space(6.0);
            if theme::modal_close(ui, p, s.close) {
                close = true;
            }
        });
    if close {
        app.controller.state.update_modal = false;
    }
}
