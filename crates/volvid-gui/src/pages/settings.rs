use eframe::egui;
use egui::{Align, Layout, RichText, Stroke};
use volvid_engine::Theme;

use crate::app::VolvidApp;
use crate::theme::{self, secondary_button};

pub fn show(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    ui.label(
        RichText::new(s.settings_title)
            .size(36.0)
            .strong()
            .color(p.text),
    );
    ui.add_space(22.0);

    settings_card(ui, p, s.appearance, |ui| {
        setting_row(ui, s.select_language, |ui| {
            segmented(
                ui,
                p,
                "Русский",
                app.controller.state.locale == volvid_core::Locale::Ru,
                || app.controller.set_locale(volvid_core::Locale::Ru),
            );
            segmented(
                ui,
                p,
                "English",
                app.controller.state.locale == volvid_core::Locale::En,
                || app.controller.set_locale(volvid_core::Locale::En),
            );
        });
        ui.add_space(4.0);
        setting_row(ui, s.theme, |ui| {
            let current = app.controller.state.config.theme;
            for (value, label) in [
                (Theme::Light, s.theme_light),
                (Theme::Dark, s.theme_dark),
                (Theme::System, s.theme_system),
            ] {
                segmented(ui, p, label, current == value, || {
                    app.controller.set_theme(value)
                });
            }
        });
    });

    ui.add_space(14.0);
    settings_card(ui, p, s.paths, |ui| {
        path_row(
            app,
            ui,
            s.downloads_dir,
            app.controller.state.downloads_dir.clone(),
            s.open_folder,
            Some(s.choose_folder),
        );
        ui.add_space(2.0);
        path_row(
            app,
            ui,
            s.logs,
            app.controller.state.logs_dir.clone(),
            s.open_logs,
            None,
        );
        ui.add_space(2.0);
        path_row(
            app,
            ui,
            s.config_dir,
            app.controller.paths().config_dir.clone(),
            "",
            None,
        );
    });

    ui.add_space(14.0);
    settings_card(ui, p, s.behavior, |ui| {
        setting_row(ui, s.playlist_workers, |ui| {
            let mut value = app.controller.state.config.playlist_workers;
            ui.scope(|ui| {
                let visuals = ui.visuals_mut();
                visuals.widgets.inactive.bg_fill = p.text;
                visuals.widgets.hovered.bg_fill = p.text;
                visuals.widgets.active.bg_fill = p.text;
                visuals.widgets.inactive.fg_stroke = Stroke::new(1.0, p.border);
                visuals.widgets.hovered.fg_stroke = Stroke::new(1.0, p.border);
                visuals.selection.bg_fill = p.text;
                if ui
                    .add_sized(
                        [260.0, 18.0],
                        egui::Slider::new(&mut value, 1..=8)
                            .trailing_fill(false)
                            .show_value(false),
                    )
                    .changed()
                {
                    app.controller.set_playlist_workers(value);
                }
            });
            egui::Frame::new()
                .fill(p.card_hover)
                .stroke(Stroke::new(1.0, p.border))
                .corner_radius(egui::CornerRadius::same(8))
                .inner_margin(egui::Margin::symmetric(10, 5))
                .show(ui, |ui| {
                    ui.label(
                        RichText::new(value.to_string())
                            .color(p.text)
                            .strong()
                            .monospace(),
                    );
                });
        });
    });

    ui.add_space(14.0);
    let deps = app.controller.state.deps.clone();
    let installing = app.controller.state.installing_dep.clone();
    let deps_progress = app.controller.state.deps_progress.clone();
    settings_card(ui, p, s.deps_title, |ui| {
        for dep in deps.dependencies() {
            dependency_row(app, ui, dep, installing.as_deref(), deps_progress.as_ref());
            ui.add_space(2.0);
        }
        ui.add_space(2.0);
        ui.separator();
        ui.add_space(4.0);
        ui.label(
            RichText::new(s.environment)
                .size(11.5)
                .strong()
                .color(p.muted),
        );
        ui.add_space(2.0);
        access_row(
            ui,
            p,
            s.cookies,
            &deps.cookies.status,
            &cookies_detail(&deps.cookies),
        );
        access_row(
            ui,
            p,
            s.js_runtime,
            &deps.runtime.status,
            &deps.runtime.name,
        );
        ui.add_space(6.0);
        if secondary_button(ui, p, s.dep_refresh).clicked() {
            app.controller.backend.refresh_deps();
        }
    });

    ui.add_space(14.0);
    settings_card(ui, p, s.about, |ui| {
        setting_row(ui, s.current_version, |ui| {
            ui.label(
                RichText::new(app.controller.app_version())
                    .color(p.text)
                    .monospace()
                    .size(13.0),
            );
        });
        ui.add_space(2.0);
        setting_row(ui, s.updates, |ui| {
            if app.controller.state.update_checking {
                theme::spinner(ui, p, s.checking);
            } else if let Some(update) = &app.controller.state.update {
                theme::badge(
                    ui,
                    p,
                    &format!("{} {}", s.latest_version, update.latest),
                    p.warning,
                );
                if secondary_button(ui, p, s.install_update).clicked() {
                    app.controller.state.update_modal = true;
                }
            } else {
                theme::muted(ui, p, s.no_updates);
                if secondary_button(ui, p, s.check_now).clicked() {
                    app.controller.check_updates();
                }
            }
        });
        ui.add_space(2.0);
        setting_row(ui, s.history_title, |ui| {
            if secondary_button(ui, p, s.clear_history).clicked() {
                app.controller.clear_history();
            }
        });
    });
}

fn settings_card(
    ui: &mut egui::Ui,
    p: &crate::theme::Palette,
    title: &str,
    add_contents: impl FnOnce(&mut egui::Ui),
) {
    theme::card_frame(p).show(ui, |ui| {
        ui.label(RichText::new(title).size(12.0).strong().color(p.muted));
        ui.add_space(10.0);
        add_contents(ui);
    });
}

fn setting_row(ui: &mut egui::Ui, label: &str, add_contents: impl FnOnce(&mut egui::Ui)) {
    ui.horizontal(|ui| {
        ui.allocate_ui_with_layout(
            egui::vec2(220.0, 30.0),
            Layout::left_to_right(Align::Center),
            |ui| {
                ui.label(RichText::new(label).color(ui.visuals().text_color()));
            },
        );
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            add_contents(ui);
        });
    });
}

fn path_row(
    app: &mut VolvidApp,
    ui: &mut egui::Ui,
    label: &str,
    path: std::path::PathBuf,
    action: &str,
    second_action: Option<&str>,
) {
    let p = app.palette();
    ui.horizontal(|ui| {
        ui.allocate_ui_with_layout(
            egui::vec2(220.0, 30.0),
            Layout::left_to_right(Align::Center),
            |ui| {
                ui.label(RichText::new(label).color(p.text));
            },
        );
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            if let Some(second) = second_action {
                if secondary_button(ui, p, second).clicked() {
                    app.controller.pick_folder();
                }
            }
            if !action.is_empty() && secondary_button(ui, p, action).clicked() {
                app.controller.open_folder(path.clone());
            }
            egui::Frame::new()
                .fill(p.card_hover)
                .stroke(Stroke::new(1.0, p.border))
                .corner_radius(egui::CornerRadius::same(8))
                .inner_margin(egui::Margin::symmetric(10, 7))
                .show(ui, |ui| {
                    ui.set_width(300.0);
                    ui.add(
                        egui::Label::new(
                            RichText::new(path.to_string_lossy())
                                .family(egui::FontFamily::Monospace)
                                .size(11.5)
                                .color(p.text),
                        )
                        .truncate(),
                    );
                });
        });
    });
}

fn dependency_row(
    app: &mut VolvidApp,
    ui: &mut egui::Ui,
    dep: &volvid_core::DependencyInfo,
    installing: Option<&str>,
    progress: Option<&volvid_core::FileProgress>,
) {
    let p = app.palette();
    let s = app.s();
    let color = if dep.available {
        p.success
    } else if dep.required {
        p.danger
    } else {
        p.muted
    };
    ui.horizontal(|ui| {
        theme::status_dot(ui, color);
        ui.label(RichText::new(&dep.name).strong().color(p.text))
            .on_hover_text(&dep.path);
        ui.label(
            RichText::new(if dep.version.is_empty() {
                if app.controller.state.deps_checking {
                    s.dep_checking
                } else if dep.available {
                    s.dep_available
                } else {
                    s.dep_missing
                }
            } else {
                &dep.version
            })
            .size(11.5)
            .color(p.muted),
        );
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            let role_color = if dep.required { p.selection } else { p.muted };
            theme::badge(
                ui,
                p,
                if dep.required {
                    s.dep_required
                } else {
                    s.dep_optional
                },
                role_color,
            );
            let source = match dep.source {
                volvid_core::DependencySource::System => s.dep_system,
                volvid_core::DependencySource::Managed => s.dep_managed,
                volvid_core::DependencySource::Missing => "",
            };
            if !source.is_empty() {
                theme::badge(ui, p, source, p.muted);
            }
            if dep.downloadable
                && (!dep.available || dep.source == volvid_core::DependencySource::Managed)
            {
                let label = if dep.available {
                    format!("{} {}", s.dep_update, dep.name)
                } else {
                    format!("{} {}", s.dep_install, dep.name)
                };
                if secondary_button(ui, p, &label).clicked() {
                    app.controller.install_dependency(&dep.key);
                }
            }
        });
    });
    if installing == Some(dep.key.as_str())
        && let Some(progress) = progress
    {
        ui.add_space(4.0);
        theme::progress_bar(ui, p, (progress.pct / 100.0) as f32, 7.0);
        ui.label(
            RichText::new(format!("{:.1}%  {}", progress.pct, progress.speed))
                .size(11.0)
                .color(p.muted),
        );
    }
}

fn access_row(
    ui: &mut egui::Ui,
    p: &crate::theme::Palette,
    label: &str,
    status: &str,
    detail: &str,
) {
    let color = if status == "active" {
        p.success
    } else {
        p.muted
    };
    ui.horizontal(|ui| {
        theme::status_dot(ui, color);
        ui.label(RichText::new(label).strong().color(p.text));
        ui.label(RichText::new(detail).size(11.5).color(p.muted));
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            theme::badge(ui, p, status, color);
            let _ = color;
        });
    });
}

fn cookies_detail(cookies: &volvid_core::BrowserCookiesInfo) -> String {
    if cookies.profile_name.trim().is_empty() {
        cookies.browser.clone()
    } else {
        format!("{}:{}", cookies.browser, cookies.profile_name)
    }
}

fn segmented(
    ui: &mut egui::Ui,
    p: &crate::theme::Palette,
    label: &str,
    selected: bool,
    mut action: impl FnMut(),
) {
    let (fill, stroke, color) = if selected {
        (
            p.selection.gamma_multiply(0.16),
            Stroke::new(1.0, p.selection),
            p.selection,
        )
    } else {
        (p.card_hover, Stroke::new(1.0, p.border), p.text)
    };
    let response = ui.add(
        egui::Button::new(RichText::new(label).color(color).size(13.0).strong())
            .fill(fill)
            .stroke(stroke)
            .corner_radius(egui::CornerRadius::same(8))
            .min_size(egui::vec2(100.0, 36.0)),
    );
    theme::press_overlay(ui, &response, 8);
    if response.clicked() {
        action();
    }
}
