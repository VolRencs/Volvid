use eframe::egui;
use egui::RichText;

use crate::app::VolvidApp;
use crate::theme::{self, primary_button_ex, secondary_button_ex};

pub fn show(app: &mut VolvidApp, ctx: &egui::Context) {
    let p = app.palette();
    let s = app.s();
    let Some(choice) = app.controller.state.mixed_choice.clone() else {
        return;
    };
    let mut close = false;
    let modal = egui::Modal::new(egui::Id::unique("volvid-mixed-dialog"))
        .frame(theme::modal_frame(p))
        .backdrop_color(egui::Color32::from_black_alpha(150))
        .show(ctx, |ui| {
            ui.set_width(theme::MODAL_WIDTH_SM);
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
                    ui.set_min_width(ui.available_width());
                    ui.add(
                        egui::Label::new(RichText::new(&choice.url).size(11.5).color(p.muted))
                            .wrap(),
                    );
                });
            ui.add_space(18.0);
            ui.horizontal(|ui| {
                ui.spacing_mut().item_spacing.x = 11.0;
                let width = ((ui.available_width() - 11.0) / 2.0).max(1.0);
                if primary_button_ex(ui, p, s.video_only, egui::vec2(width, 54.0)).clicked() {
                    app.controller.choose_mixed(true);
                }
                if secondary_button_ex(ui, p, s.full_playlist, egui::vec2(width, 54.0)).clicked() {
                    app.controller.choose_mixed(false);
                }
            });
            ui.add_space(14.0);
            ui.separator();
            ui.add_space(8.0);
            if theme::modal_close(ui, p, s.close) {
                close = true;
            }
        });
    if modal.should_close() {
        close = true;
    }
    if close {
        app.controller.state.mixed_choice = None;
    }
}
