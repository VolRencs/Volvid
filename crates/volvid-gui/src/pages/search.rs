use eframe::egui;
use egui::RichText;

use crate::app::VolvidApp;
use crate::theme::{self, primary_button_sized};

pub fn show(app: &mut VolvidApp, ctx: &egui::Context) {
    let p = app.palette();
    let s = app.s();
    let loading = app.controller.state.search_loading;
    let error = app.controller.state.search_error.clone();
    let mut close = false;
    let mut selected = None;

    let modal = egui::Modal::new(egui::Id::unique("volvid-search-dialog"))
        .frame(theme::modal_frame(p))
        .backdrop_color(egui::Color32::from_black_alpha(150))
        .show(ctx, |ui| {
            ui.set_width(600.0);
            theme::modal_header(
                ui,
                p,
                egui::include_image!("../../../../assets/icons/search.svg"),
                s.search_title,
                s.search_prompt,
            );
            ui.add_space(16.0);
            ui.horizontal(|ui| {
                let response = ui.add_sized(
                    [400.0, 42.0],
                    egui::TextEdit::singleline(&mut app.controller.state.search_query)
                        .hint_text(s.search_placeholder)
                        .margin(egui::Margin::symmetric(10, 9)),
                );
                if response.has_focus() && ui.input(|input| input.key_pressed(egui::Key::Enter)) {
                    response.surrender_focus();
                    app.controller.search();
                }
                if primary_button_sized(ui, p, s.search_youtube, 154.0).clicked() {
                    app.controller.search();
                }
            });
            if let Some(error) = &error {
                ui.add_space(4.0);
                ui.label(RichText::new(error).color(p.danger).size(12.0));
            }
            if loading {
                ui.add_space(4.0);
                theme::spinner(ui, p, s.searching);
            }
            ui.add_space(8.0);
            egui::ScrollArea::vertical()
                .max_height(230.0)
                .auto_shrink([false, false])
                .show(ui, |ui| {
                    if app.controller.state.search_results.is_empty() && !loading {
                        theme::muted(ui, p, s.no_results);
                    }
                    for (index, result) in app.controller.state.search_results.iter().enumerate() {
                        let response = ui.add(
                            egui::Button::new(
                                theme::strong_text(
                                    format!(
                                        "{}  ·  {}",
                                        result.title,
                                        volvid_core::format_duration(result.duration)
                                    ),
                                    13.0,
                                )
                                .color(p.text),
                            )
                            .fill(p.card_hover)
                            .stroke(egui::Stroke::new(1.0, p.border))
                            .corner_radius(egui::CornerRadius::same(9))
                            .min_size(egui::vec2(600.0, 42.0)),
                        );
                        theme::press_overlay(ui, &response, 9);
                        if response.clicked() {
                            selected = Some(index);
                            close = true;
                        }
                    }
                });
            ui.add_space(10.0);
            ui.separator();
            ui.add_space(6.0);
            if theme::modal_close(ui, p, s.close) {
                close = true;
            }
        });
    if modal.should_close() {
        close = true;
    }
    if let Some(index) = selected {
        app.controller.select_search_result(index);
    }
    if close {
        app.controller.state.search_modal = false;
    }
}
