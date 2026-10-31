use eframe::egui;
use egui::{Align, Layout, RichText};
use volvid_engine::Theme;

use crate::app::VolvidApp;
use crate::theme::{self, secondary_button, secondary_icon_button};

pub fn show(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    theme::title(ui, p, s.settings_title);
    ui.add_space(22.0);

    settings_card(ui, p, s.appearance, |ui| {
        setting_row(ui, p, s.select_language, |ui| {
            if theme::segmented_button(
                ui,
                p,
                "Русский",
                app.controller.state.locale == volvid_core::Locale::Ru,
                100.0,
            )
            .clicked()
            {
                app.controller.set_locale(volvid_core::Locale::Ru);
            }
            if theme::segmented_button(
                ui,
                p,
                "English",
                app.controller.state.locale == volvid_core::Locale::En,
                100.0,
            )
            .clicked()
            {
                app.controller.set_locale(volvid_core::Locale::En);
            }
        });
        ui.add_space(4.0);
        setting_row(ui, p, s.theme, |ui| {
            let current = app.controller.state.config.theme;
            for (value, label) in [
                (Theme::Light, s.theme_light),
                (Theme::Dark, s.theme_dark),
                (Theme::System, s.theme_system),
            ] {
                if theme::segmented_button(ui, p, label, current == value, 100.0).clicked() {
                    app.controller.set_theme(value);
                }
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
        ui.add_space(8.0);
        path_row(
            app,
            ui,
            s.logs,
            app.controller.state.logs_dir.clone(),
            s.open_logs,
            None,
        );
        ui.add_space(8.0);
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
        setting_row(ui, p, s.playlist_workers, |ui| {
            let mut value = app.controller.state.config.playlist_workers;
            if theme::slider(ui, p, &mut value, 1..=8, 222.0).changed() {
                app.controller.set_playlist_workers(value);
            }
            theme::value_box(ui, p, value.to_string());
        });
    });

    ui.add_space(14.0);
    let deps = app.controller.state.deps.clone();
    let installing = app.controller.state.installing_dep.clone();
    let deps_progress = app.controller.state.deps_progress.clone();
    let mut refresh = false;
    settings_card(ui, p, s.deps_title, |ui| {
        for dep in deps.dependencies() {
            dependency_row(app, ui, dep, installing.as_deref(), deps_progress.as_ref());
            ui.add_space(4.0);
        }
        ui.add_space(4.0);
        ui.separator();
        ui.add_space(6.0);
        theme::section_title(ui, p, s.environment);
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
        ui.add_space(8.0);
        if secondary_icon_button(
            ui,
            p,
            egui::include_image!("../../../../assets/icons/refresh.svg"),
            s.dep_refresh,
        )
        .clicked()
        {
            refresh = true;
        }
    });
    if refresh {
        app.controller.backend.refresh_deps();
    }

    ui.add_space(14.0);
    settings_card(ui, p, s.about, |ui| {
        ui.horizontal(|ui| {
            let label_width = if ui.available_width() >= 720.0 {
                220.0
            } else {
                160.0
            };
            ui.allocate_ui_with_layout(
                egui::vec2(label_width, 32.0),
                Layout::left_to_right(Align::Center),
                |ui| {
                    ui.label(RichText::new(s.current_version).color(p.text).size(14.0));
                },
            );
            ui.label(
                RichText::new(app.controller.app_version())
                    .color(p.text)
                    .size(13.0),
            );
            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                if app.controller.state.update_checking {
                    theme::spinner(ui, p, s.checking);
                } else if let Some(update) = &app.controller.state.update.clone() {
                    if secondary_button(ui, p, s.install_update).clicked() {
                        app.controller.state.update_modal = true;
                    }
                    theme::badge(
                        ui,
                        p,
                        &format!("{} {}", s.latest_version, update.latest),
                        p.warning,
                    );
                } else if secondary_button(ui, p, s.check_now).clicked() {
                    app.controller.check_updates();
                }
            });
        });
        ui.add_space(4.0);
        setting_row(ui, p, s.history_title, |ui| {
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
        ui.set_min_width(ui.available_width());
        theme::section_title(ui, p, title);
        ui.add_space(10.0);
        add_contents(ui);
    });
}

fn setting_row(
    ui: &mut egui::Ui,
    _p: &crate::theme::Palette,
    label: &str,
    add_contents: impl FnOnce(&mut egui::Ui),
) {
    ui.horizontal(|ui| {
        let label_width = if ui.available_width() >= 720.0 {
            220.0
        } else {
            160.0
        };
        ui.allocate_ui_with_layout(
            egui::vec2(label_width, 32.0),
            Layout::left_to_right(Align::Center),
            |ui| {
                ui.label(
                    RichText::new(label)
                        .color(ui.visuals().text_color())
                        .size(14.0),
                );
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
    let wide = ui.available_width() >= 780.0;
    if wide {
        ui.horizontal(|ui| {
            ui.allocate_ui_with_layout(
                egui::vec2(220.0, 32.0),
                Layout::left_to_right(Align::Center),
                |ui| {
                    ui.label(RichText::new(label).color(p.text).size(14.0));
                },
            );
            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                if let Some(second) = second_action
                    && secondary_button(ui, p, second).clicked()
                {
                    app.controller.pick_folder();
                }
                if !action.is_empty() && secondary_button(ui, p, action).clicked() {
                    app.controller.open_folder(path.clone());
                }
                let max_path = (ui.available_width() - 2.0).max(80.0);
                path_box(ui, p, &path, max_path);
            });
        });
    } else {
        ui.vertical(|ui| {
            ui.label(RichText::new(label).color(p.text).size(14.0));
            path_box(ui, p, &path, ui.available_width());
            ui.horizontal(|ui| {
                if !action.is_empty() && secondary_button(ui, p, action).clicked() {
                    app.controller.open_folder(path.clone());
                }
                if let Some(second) = second_action
                    && secondary_button(ui, p, second).clicked()
                {
                    app.controller.pick_folder();
                }
            });
        });
    }
}

fn path_box(ui: &mut egui::Ui, p: &crate::theme::Palette, path: &std::path::Path, width: f32) {
    let text = path.to_string_lossy();
    theme::text_chip(ui, p, &text, 12.0, width, false);
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
        ui.label(theme::strong_text(&dep.name, 14.0).color(p.text))
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
        ui.label(theme::strong_text(label, 13.5).color(p.text));
        ui.label(RichText::new(detail).size(11.5).color(p.muted));
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            theme::badge(ui, p, status, color);
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
