use eframe::egui;
use egui::{Align, Key, Layout, RichText, Stroke};
use volvid_app::Route;

use crate::app::VolvidApp;
use crate::format;
use crate::theme::{self, primary_button_sized, secondary_button};

pub fn show(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    ui.add_space(4.0);
    theme::title(ui, p, s.home_title);
    ui.add_space(4.0);
    theme::muted(ui, p, s.home_subtitle);
    ui.add_space(theme::PAGE_HEADER_GAP);

    if app.controller.state.deps.missing_required() {
        egui::Frame::new()
            .fill(p.danger.gamma_multiply(0.08))
            .stroke(Stroke::new(1.0, p.danger.gamma_multiply(0.3)))
            .corner_radius(egui::CornerRadius::same(12))
            .inner_margin(egui::Margin::same(14))
            .show(ui, |ui| {
                ui.label(theme::strong_text(s.deps_missing, 14.0).color(p.danger));
                theme::muted(ui, p, s.deps_settings_hint);
            });
        ui.add_space(14.0);
    }

    theme::card_frame(p).show(ui, |ui| {
        ui.horizontal(|ui| {
            ui.add(
                egui::Image::new(egui::include_image!("../../../../assets/icons/link.svg"))
                    .fit_to_exact_size(egui::vec2(18.0, 18.0))
                    .tint(p.muted),
            );
            let button_width = theme::text_width(ui, s.analyze, theme::semibold(14.0)) + 40.0;
            let input_width = (ui.available_width() - button_width - 8.0).max(180.0);
            ui.scope(|ui| {
                ui.visuals_mut().extreme_bg_color = p.card_hover;
                ui.visuals_mut().widgets.inactive.bg_stroke = Stroke::new(1.0, p.border);
                let response = theme::text_input(
                    ui,
                    &mut app.controller.state.url,
                    s.url_placeholder,
                    input_width,
                );
                if response.has_focus() && ui.input(|input| input.key_pressed(Key::Enter)) {
                    response.surrender_focus();
                    app.controller.analyze_url();
                }
            });
            if primary_button_sized(ui, p, s.analyze, button_width).clicked() {
                app.controller.analyze_url();
            }
        });
        if let Some(error) = app.controller.state.url_error.clone() {
            ui.add_space(4.0);
            ui.add(
                egui::Label::new(RichText::new(error).color(p.danger).size(theme::FONT_MUTED))
                    .wrap(),
            );
        }
        ui.add_space(12.0);
        ui.horizontal(|ui| {
            if secondary_button(ui, p, s.paste).clicked() {
                app.paste_clipboard();
            }
            if secondary_button(ui, p, s.search_youtube).clicked() {
                app.controller.state.search_modal = true;
                app.controller.state.search_error = None;
            }
            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                theme::muted_inline(ui, p, s.drag_hint);
            });
        });
    });

    ui.add_space(theme::CARD_GAP);
    let available = ui.available_width();
    if let Some((left, right)) = theme::split_columns(available, 16.0, 0.42, 340.0, 360.0) {
        ui.horizontal_top(|ui| {
            ui.spacing_mut().item_spacing.x = 16.0;
            ui.allocate_ui_with_layout(
                egui::vec2(left, 0.0),
                Layout::top_down(Align::LEFT),
                |ui| location_card(app, ui),
            );
            ui.allocate_ui_with_layout(
                egui::vec2(right, 0.0),
                Layout::top_down(Align::LEFT),
                |ui| recent_card(app, ui),
            );
        });
    } else {
        location_card(app, ui);
        ui.add_space(theme::CARD_GAP);
        recent_card(app, ui);
    }
}

fn location_card(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    theme::card(ui, p, |ui| {
        theme::section_title(ui, p, s.download_location);
        ui.add_space(8.0);
        let path = app.controller.state.downloads_dir.to_string_lossy();
        theme::text_chip(ui, p, &path, 12.5, ui.available_width(), false);
        ui.add_space(10.0);
        ui.horizontal(|ui| {
            if secondary_button(ui, p, s.choose_folder).clicked() {
                app.controller.pick_folder();
            }
            if secondary_button(ui, p, s.open_folder).clicked() {
                app.controller
                    .open_folder(app.controller.state.downloads_dir.clone());
            }
        });
    });
}

fn recent_card(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    theme::card(ui, p, |ui| {
        ui.horizontal(|ui| {
            theme::section_title(ui, p, s.recent);
            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                if !app.controller.state.history.is_empty()
                    && secondary_button(ui, p, s.nav_history).clicked()
                {
                    app.controller.set_route(Route::Summary);
                }
            });
        });
        ui.add_space(8.0);
        if app.controller.state.history.is_empty() {
            theme::dashed_placeholder(ui, p, 82.0, s.no_history);
        } else {
            let history: Vec<_> = app
                .controller
                .state
                .history
                .iter()
                .rev()
                .take(4)
                .cloned()
                .collect();
            for item in history {
                let color = if item.ok { p.success } else { p.danger };
                let timestamp = format::timestamp(item.timestamp, app.controller.state.locale);
                let open_width =
                    theme::text_width(ui, s.open, theme::semibold(theme::FONT_LABEL)) + 30.0;
                let timestamp_width = theme::text_width(
                    ui,
                    &timestamp,
                    egui::FontId::proportional(theme::FONT_SMALL),
                );
                let actions_width = open_width + 8.0 + timestamp_width + 8.0;
                ui.horizontal(|ui| {
                    theme::status_dot(ui, color);
                    theme::row_with_actions(
                        ui,
                        actions_width,
                        |ui| {
                            ui.add(
                                egui::Label::new(
                                    RichText::new(&item.label)
                                        .color(p.text)
                                        .size(theme::FONT_MUTED),
                                )
                                .truncate(),
                            );
                        },
                        |ui| {
                            if secondary_button(ui, p, s.open).clicked() {
                                app.controller.open_folder(item.output_dir.clone());
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
            }
        }
    });
}
