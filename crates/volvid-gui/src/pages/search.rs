use eframe::egui;
use egui::RichText;

use crate::app::VolvidApp;
use crate::theme::{self, primary_button_sized};

pub fn show(app: &mut VolvidApp, ctx: &egui::Context) {
    let p = app.palette();
    let s = app.s();
    theme::modal_backdrop(ctx);
    let loading = app.controller.state.search_loading;
    let results = app.controller.state.search_results.clone();
    let error = app.controller.state.search_error.clone();

    let mut close = false;
    egui::Window::new("volvid-search-dialog")
        .title_bar(false)
        .collapsible(false)
        .resizable(false)
        .fixed_size(egui::vec2(660.0, 430.0))
        .frame(theme::modal_frame(p))
        .order(egui::Order::Foreground)
        .anchor(egui::Align2::CENTER_CENTER, [0.0, 0.0])
        .show(ctx, |ui| {
            ui.set_width(612.0);
            theme::modal_header(
                ui,
                p,
                egui::include_image!("../../../../assets/icons/link.svg"),
                s.search_title,
                s.search_prompt,
            );
            ui.add_space(16.0);
            ui.horizontal(|ui| {
                let response = ui.add_sized(
                    [440.0, 38.0],
                    egui::TextEdit::singleline(&mut app.controller.state.search_query)
                        .hint_text(s.search_placeholder)
                        .margin(egui::Margin::symmetric(10, 8)),
                );
                if response.lost_focus() && ui.input(|input| input.key_pressed(egui::Key::Enter)) {
                    app.controller.search();
                }
                if primary_button_sized(ui, p, s.search_youtube, 160.0).clicked() {
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
                .max_height(190.0)
                .auto_shrink([false, false])
                .show(ui, |ui| {
                    if results.is_empty() && !loading {
                        theme::muted(ui, p, s.no_results);
                    }
                    for (index, result) in results.iter().enumerate() {
                        let response = ui.add(
                            egui::Button::new(
                                RichText::new(format!(
                                    "{}  ·  {}",
                                    result.title,
                                    volvid_core::format_duration(result.duration)
                                ))
                                .color(p.text)
                                .size(13.0),
                            )
                            .fill(p.card_hover)
                            .stroke(egui::Stroke::new(1.0, p.border))
                            .corner_radius(egui::CornerRadius::same(9))
                            .min_size(egui::vec2(600.0, 38.0)),
                        );
                        theme::press_overlay(ui, &response, 9);
                        if response.clicked() {
                            app.controller.select_search_result(index);
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
    if close {
        app.controller.state.search_modal = false;
    }
}
