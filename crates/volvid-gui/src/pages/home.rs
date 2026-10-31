use eframe::egui;
use egui::{Align, Key, Layout, RichText, Stroke};
use volvid_app::Route;

use crate::app::VolvidApp;
use crate::theme::{self, primary_button, secondary_button};

pub fn show(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    ui.add_space(4.0);
    ui.label(
        RichText::new(s.home_title)
            .size(36.0)
            .strong()
            .color(p.text),
    );
    ui.add_space(6.0);
    theme::muted(ui, p, s.home_subtitle);
    ui.add_space(24.0);

    if app.controller.state.deps.missing_required() {
        egui::Frame::new()
            .fill(p.danger.gamma_multiply(0.08))
            .stroke(Stroke::new(1.0, p.danger.gamma_multiply(0.3)))
            .corner_radius(egui::CornerRadius::same(12))
            .inner_margin(egui::Margin::same(14))
            .show(ui, |ui| {
                ui.label(RichText::new(s.deps_missing).color(p.danger).strong());
                theme::muted(ui, p, s.deps_settings_hint);
            });
        ui.add_space(14.0);
    }

    theme::card_frame(p)
        .inner_margin(egui::Margin::same(18))
        .show(ui, |ui| {
            ui.horizontal(|ui| {
                let button_width = 174.0;
                let input_width = (ui.available_width() - button_width - 40.0).max(200.0);
                ui.add(
                    egui::Image::new(egui::include_image!("../../../../assets/icons/link.svg"))
                        .fit_to_exact_size(egui::vec2(18.0, 18.0))
                        .tint(p.muted),
                );
                ui.scope(|ui| {
                    ui.visuals_mut().extreme_bg_color = p.card_hover;
                    ui.visuals_mut().widgets.inactive.bg_stroke = Stroke::new(1.0, p.border);
                    let response = ui.add_sized(
                        [input_width, 46.0],
                        egui::TextEdit::singleline(&mut app.controller.state.url)
                            .hint_text(s.url_placeholder)
                            .font(egui::TextStyle::Body)
                            .margin(egui::Margin::symmetric(12, 11)),
                    );
                    if response.lost_focus() && ui.input(|input| input.key_pressed(Key::Enter)) {
                        app.controller.analyze_url();
                    }
                });
                if primary_button(ui, p, s.analyze).clicked() {
                    app.controller.analyze_url();
                }
            });
            if let Some(error) = app.controller.state.url_error.clone() {
                ui.add_space(4.0);
                ui.label(RichText::new(error).color(p.danger).size(12.5));
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
                    theme::muted(ui, p, s.drag_hint);
                });
            });
        });

    ui.add_space(16.0);
    let available = ui.available_width();
    let card_width = ((available - 16.0) / 2.0).max(320.0);
    ui.horizontal_top(|ui| {
        ui.allocate_ui_with_layout(
            egui::vec2(card_width, 0.0),
            Layout::top_down(Align::LEFT),
            |ui| location_card(app, ui),
        );
        ui.add_space(16.0);
        ui.allocate_ui_with_layout(
            egui::vec2(card_width, 0.0),
            Layout::top_down(Align::LEFT),
            |ui| recent_card(app, ui),
        );
    });
}

fn location_card(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    theme::card_frame(p).show(ui, |ui| {
        ui.label(
            RichText::new(s.download_location)
                .size(12.0)
                .strong()
                .color(p.muted),
        );
        ui.add_space(8.0);
        egui::Frame::new()
            .fill(p.track)
            .corner_radius(egui::CornerRadius::same(9))
            .inner_margin(egui::Margin::symmetric(12, 10))
            .show(ui, |ui| {
                ui.add(
                    egui::Label::new(
                        RichText::new(app.controller.state.downloads_dir.to_string_lossy())
                            .color(p.text)
                            .size(12.5)
                            .family(egui::FontFamily::Monospace),
                    )
                    .truncate(),
                );
            });
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
    theme::card_frame(p).show(ui, |ui| {
        ui.horizontal(|ui| {
            ui.label(RichText::new(s.recent).size(12.0).strong().color(p.muted));
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
                ui.horizontal(|ui| {
                    theme::status_dot(ui, color);
                    let title_width = (ui.available_width() - 150.0).max(80.0);
                    ui.add_sized(
                        [title_width, 20.0],
                        egui::Label::new(RichText::new(&item.label).color(p.text).size(12.5))
                            .truncate(),
                    );
                    ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                        if secondary_button(ui, p, s.open).clicked() {
                            app.controller.open_folder(item.output_dir.clone());
                        }
                        ui.label(
                            RichText::new(format_timestamp(item.timestamp))
                                .size(10.5)
                                .color(p.muted),
                        );
                    });
                });
            }
        }
    });
}

fn format_timestamp(timestamp: i64) -> String {
    jiff::Timestamp::from_second(timestamp)
        .map(|time| time.strftime("%Y-%m-%d %H:%M").to_string())
        .unwrap_or_default()
}
