use eframe::egui;
use egui::{FontId, RichText};

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
            let width = theme::MODAL_WIDTH_LG
                .min(ctx.content_rect().width() - 48.0)
                .max(320.0);
            ui.set_width(width);
            theme::modal_header(
                ui,
                p,
                egui::include_image!("../../../../assets/icons/search.svg"),
                s.search_title,
                s.search_prompt,
            );
            ui.add_space(16.0);
            ui.horizontal_wrapped(|ui| {
                let input_width = (width - 166.0 - 8.0).max(180.0);
                let response = theme::text_input(
                    ui,
                    &mut app.controller.state.search_query,
                    s.search_placeholder,
                    input_width,
                );
                if response.has_focus() && ui.input(|input| input.key_pressed(egui::Key::Enter)) {
                    response.surrender_focus();
                    app.controller.search();
                }
                if primary_button_sized(ui, p, s.search_youtube, 166.0).clicked() {
                    app.controller.search();
                }
            });
            if let Some(error) = &error {
                ui.add_space(4.0);
                ui.add(
                    egui::Label::new(RichText::new(error).color(p.danger).size(theme::FONT_SMALL))
                        .wrap(),
                );
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
                        let duration = volvid_core::format_duration(result.duration);
                        let duration_width = theme::text_width(
                            ui,
                            &duration,
                            FontId::proportional(theme::FONT_MUTED),
                        );
                        let row = egui::Frame::new()
                            .fill(p.card_hover)
                            .stroke(egui::Stroke::new(1.0, p.border))
                            .corner_radius(egui::CornerRadius::same(theme::ROW_RADIUS))
                            .inner_margin(egui::Margin::symmetric(12, 9))
                            .show(ui, |ui| {
                                ui.set_min_width(ui.available_width());
                                ui.horizontal(|ui| {
                                    let title_width =
                                        (ui.available_width() - duration_width - 16.0).max(80.0);
                                    ui.allocate_ui_with_layout(
                                        egui::vec2(title_width, 0.0),
                                        egui::Layout::left_to_right(egui::Align::Center),
                                        |ui| {
                                            ui.add(
                                                egui::Label::new(
                                                    theme::strong_text(&result.title, 14.0)
                                                        .color(p.text),
                                                )
                                                .truncate(),
                                            )
                                            .on_hover_text(&result.title);
                                        },
                                    );
                                    ui.with_layout(
                                        egui::Layout::right_to_left(egui::Align::Center),
                                        |ui| {
                                            ui.add(
                                                egui::Label::new(
                                                    RichText::new(&duration)
                                                        .size(theme::FONT_MUTED)
                                                        .color(p.muted),
                                                )
                                                .truncate(),
                                            );
                                        },
                                    );
                                });
                            });
                        let response = row
                            .response
                            .interact(egui::Sense::click())
                            .on_hover_cursor(egui::CursorIcon::PointingHand)
                            .on_hover_text(format!("{}  ·  {}", result.title, duration));
                        theme::press_overlay(ui, &response, theme::ROW_RADIUS);
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
