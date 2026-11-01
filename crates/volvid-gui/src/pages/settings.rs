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
                "English",
                app.controller.state.locale == volvid_core::Locale::En,
                104.0,
            )
            .clicked()
            {
                app.controller.set_locale(volvid_core::Locale::En);
            }
            if theme::segmented_button(
                ui,
                p,
                "Русский",
                app.controller.state.locale == volvid_core::Locale::Ru,
                104.0,
            )
            .clicked()
            {
                app.controller.set_locale(volvid_core::Locale::Ru);
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
                if theme::segmented_button(ui, p, label, current == value, 104.0).clicked() {
                    app.controller.set_theme(value);
                }
            }
        });
    });

    ui.add_space(theme::CARD_GAP);
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
            s,
            s.cookies,
            &deps.cookies.status,
            &cookies_detail(&deps.cookies),
        );
        access_row(
            ui,
            p,
            s,
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
        app.controller.refresh_deps();
    }

    ui.add_space(14.0);
    settings_card(ui, p, s.about, |ui| {
        let update = app.controller.state.update.clone();
        let checking = app.controller.state.update_checking;
        if ui.available_width() < 700.0 {
            ui.vertical(|ui| {
                ui.add(
                    egui::Label::new(
                        RichText::new(s.current_version)
                            .color(p.text)
                            .size(theme::FONT_LABEL),
                    )
                    .truncate(),
                );
                ui.add(
                    egui::Label::new(
                        RichText::new(app.controller.app_version())
                            .color(p.text)
                            .size(theme::FONT_MUTED),
                    )
                    .truncate(),
                );
                ui.add_space(6.0);
                ui.horizontal_wrapped(|ui| {
                    about_actions(ui, p, s, app, checking, update.as_ref());
                });
            });
        } else {
            ui.horizontal(|ui| {
                ui.allocate_ui_with_layout(
                    egui::vec2(theme::SETTINGS_LABEL_W, 32.0),
                    Layout::left_to_right(Align::Center),
                    |ui| {
                        ui.add(
                            egui::Label::new(
                                RichText::new(s.current_version)
                                    .color(p.text)
                                    .size(theme::FONT_LABEL),
                            )
                            .truncate(),
                        );
                    },
                );
                ui.add(
                    egui::Label::new(
                        RichText::new(app.controller.app_version())
                            .color(p.text)
                            .size(theme::FONT_MUTED),
                    )
                    .truncate(),
                );
                ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                    ui.with_layout(Layout::left_to_right(Align::Center), |ui| {
                        about_actions(ui, p, s, app, checking, update.as_ref());
                    });
                });
            });
        }
        ui.add_space(4.0);
        setting_row(ui, p, s.history_title, |ui| {
            if secondary_button(ui, p, s.clear_history).clicked() {
                app.controller.clear_history();
            }
        });
    });
}

fn about_actions(
    ui: &mut egui::Ui,
    p: &crate::theme::Palette,
    s: &volvid_i18n::Strings,
    app: &mut VolvidApp,
    checking: bool,
    update: Option<&volvid_core::UpdateInfo>,
) {
    if checking {
        theme::spinner(ui, p, s.checking);
    } else if let Some(update) = update {
        if secondary_button(ui, p, s.install_update).clicked() {
            app.controller.state.update_modal = true;
        }
        theme::badge(
            ui,
            &format!("{} {}", s.latest_version, update.latest),
            p.warning,
        );
    } else if secondary_button(ui, p, s.check_now).clicked() {
        app.controller.check_updates();
    }
}

fn settings_card(
    ui: &mut egui::Ui,
    p: &crate::theme::Palette,
    title: &str,
    add_contents: impl FnOnce(&mut egui::Ui),
) {
    theme::card(ui, p, |ui| {
        theme::section_title(ui, p, title);
        ui.add_space(10.0);
        add_contents(ui);
    });
}

fn setting_row(
    ui: &mut egui::Ui,
    p: &crate::theme::Palette,
    label: &str,
    add_contents: impl FnOnce(&mut egui::Ui),
) {
    let narrow = ui.available_width() < theme::SETTINGS_BREAKPOINT;
    if narrow {
        ui.vertical(|ui| {
            ui.add(
                egui::Label::new(RichText::new(label).color(p.text).size(theme::FONT_LABEL))
                    .truncate(),
            );
            ui.horizontal_wrapped(|ui| add_contents(ui));
        });
    } else {
        ui.horizontal(|ui| {
            let label_width = if ui.available_width() >= theme::SETTINGS_BREAKPOINT {
                theme::SETTINGS_LABEL_W
            } else {
                theme::SETTINGS_LABEL_W_NARROW
            };
            label_cell(ui, p, label, label_width);
            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                ui.with_layout(Layout::left_to_right(Align::Center), |ui| {
                    add_contents(ui);
                });
            });
        });
    }
}

fn label_cell(ui: &mut egui::Ui, p: &crate::theme::Palette, label: &str, width: f32) {
    ui.allocate_ui_with_layout(
        egui::vec2(width, 32.0),
        Layout::left_to_right(Align::Center),
        |ui| {
            ui.add(
                egui::Label::new(RichText::new(label).color(p.text).size(theme::FONT_LABEL))
                    .truncate(),
            );
        },
    );
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
            label_cell(ui, p, label, theme::SETTINGS_LABEL_W);
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

fn badge_width(ui: &egui::Ui, text: &str) -> f32 {
    theme::text_width(ui, text, theme::semibold(12.0)) + 22.0
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
    let role_text = if dep.required {
        s.dep_required
    } else {
        s.dep_optional
    };
    let source = match dep.source {
        volvid_core::DependencySource::System => s.dep_system,
        volvid_core::DependencySource::Managed => s.dep_managed,
        volvid_core::DependencySource::Missing => "",
    };
    let install_label = (dep.downloadable
        && (!dep.available || dep.source == volvid_core::DependencySource::Managed))
        .then(|| {
            if dep.available {
                format!("{} {}", s.dep_update, dep.name)
            } else {
                format!("{} {}", s.dep_install, dep.name)
            }
        });
    let mut actions_width = badge_width(ui, role_text) + 8.0;
    if !source.is_empty() {
        actions_width += badge_width(ui, source) + 8.0;
    }
    if let Some(label) = &install_label {
        actions_width += theme::text_width(ui, label, theme::semibold(theme::FONT_LABEL)) + 30.0;
    }
    let version = if dep.version.is_empty() {
        if app.controller.state.deps_checking {
            s.dep_checking
        } else if dep.available {
            s.dep_available
        } else {
            s.dep_missing
        }
    } else {
        dep.version.as_str()
    };
    let busy = installing.is_some();
    theme::row_with_actions_inline(
        ui,
        actions_width,
        |ui| {
            theme::status_dot(ui, color);
            ui.add(
                egui::Label::new(theme::strong_text(&dep.name, theme::FONT_LABEL).color(p.text))
                    .truncate(),
            )
            .on_hover_text(&dep.path);
            ui.add(
                egui::Label::new(
                    RichText::new(version)
                        .size(theme::FONT_SMALL)
                        .color(p.muted),
                )
                .truncate(),
            );
        },
        |ui| {
            if let Some(label) = &install_label {
                ui.add_enabled_ui(!busy, |ui| {
                    if secondary_button(ui, p, label).clicked() {
                        app.controller.install_dependency(&dep.key);
                    }
                });
            }
            if !source.is_empty() {
                theme::badge(ui, source, p.muted);
            }
            theme::badge(
                ui,
                role_text,
                if dep.required { p.selection } else { p.muted },
            );
        },
    );
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
    s: &volvid_i18n::Strings,
    label: &str,
    status: &str,
    detail: &str,
) {
    let color = if status == volvid_core::STATUS_ACTIVE {
        p.success
    } else {
        p.muted
    };
    let status = match status {
        volvid_core::STATUS_ACTIVE => s.status_active,
        volvid_core::STATUS_NOT_FOUND => s.status_not_found,
        volvid_core::STATUS_NO_PROFILE => s.status_no_profile,
        other => other,
    };
    let actions_width = badge_width(ui, status) + 8.0;
    theme::row_with_actions_inline(
        ui,
        actions_width,
        |ui| {
            theme::status_dot(ui, color);
            ui.add(
                egui::Label::new(theme::strong_text(label, theme::FONT_LABEL).color(p.text))
                    .truncate(),
            );
            ui.add(
                egui::Label::new(RichText::new(detail).size(theme::FONT_SMALL).color(p.muted))
                    .truncate(),
            );
        },
        |ui| {
            theme::badge(ui, status, color);
        },
    );
}

fn cookies_detail(cookies: &volvid_core::BrowserCookiesInfo) -> String {
    if cookies.profile_name.trim().is_empty() {
        cookies.browser.clone()
    } else {
        format!("{}:{}", cookies.browser, cookies.profile_name)
    }
}
