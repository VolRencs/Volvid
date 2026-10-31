use eframe::egui;
use egui::RichText;

use crate::app::VolvidApp;
use crate::theme::{self, primary_button_sized, secondary_button_sized};

pub fn show(app: &mut VolvidApp, ctx: &egui::Context) {
    let p = app.palette();
    let s = app.s();
    let Some(choice) = app.controller.state.mixed_choice.clone() else {
        return;
    };
    theme::modal_backdrop(ctx);
    egui::Window::new("volvid-mixed-dialog")
        .title_bar(false)
        .collapsible(false)
        .resizable(false)
        .fixed_size(egui::vec2(640.0, 430.0))
        .frame(theme::modal_frame(p))
        .order(egui::Order::Foreground)
        .anchor(egui::Align2::CENTER_CENTER, [0.0, 0.0])
        .show(ctx, |ui| {
            ui.set_width(592.0);
            theme::modal_header(
                ui,
                p,
                egui::include_image!("../../../../assets/icons/queue.svg"),
                s.mixed_title,
                s.mixed_hint,
            );
            ui.add_space(18.0);
            egui::Frame::new()
                .fill(p.card_hover)
                .stroke(egui::Stroke::new(1.0, p.border))
                .corner_radius(egui::CornerRadius::same(10))
                .inner_margin(egui::Margin::symmetric(14, 12))
                .show(ui, |ui| {
                    ui.label(
                        RichText::new(&choice.url)
                            .family(egui::FontFamily::Monospace)
                            .size(11.5)
                            .color(p.muted),
                    );
                });
            ui.add_space(18.0);
            ui.horizontal(|ui| {
                if primary_button_sized(ui, p, s.video_only, 286.0).clicked() {
                    app.controller.choose_mixed(true);
                }
                if secondary_button_sized(ui, p, s.full_playlist, 286.0).clicked() {
                    app.controller.choose_mixed(false);
                }
            });
            ui.add_space(14.0);
            ui.separator();
            ui.add_space(8.0);
            if theme::modal_close(ui, p, s.close) {
                app.controller.state.mixed_choice = None;
            }
        });
}
