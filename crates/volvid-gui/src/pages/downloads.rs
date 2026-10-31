use eframe::egui;
use egui::{Align, Layout, RichText, Sense, Stroke, Vec2};
use volvid_app::{JobStatus, Route};
use volvid_i18n::{format_bytes, format_elapsed};

use crate::app::VolvidApp;
use crate::theme::{self, secondary_button, secondary_button_sized};

pub fn show(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    let jobs = app.controller.state.jobs.clone();
    let active = jobs
        .iter()
        .filter(|job| job.status == JobStatus::Active)
        .count();
    let waiting = jobs
        .iter()
        .filter(|job| job.status == JobStatus::Active && job.progress() <= 0.0)
        .count();
    let running = active.saturating_sub(waiting);
    let failed = jobs
        .iter()
        .filter(|job| job.status == JobStatus::Failed)
        .count();
    let finished = jobs.len().saturating_sub(active);

    ui.horizontal(|ui| {
        ui.vertical(|ui| {
            ui.label(RichText::new(s.queue).size(36.0).strong().color(p.text));
            let mut parts = vec![s.downloading_count.replace("%d", &running.to_string())];
            if waiting > 0 {
                parts.push(s.waiting_count.replace("%d", &waiting.to_string()));
            }
            if failed > 0 {
                parts.push(s.errors_count.replace("%d", &failed.to_string()));
            }
            theme::muted(ui, p, parts.join("  ·  "));
        });
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            if ui
                .add_enabled(
                    finished > 0,
                    egui::Button::new(RichText::new(s.clear_finished).color(p.text).size(13.0))
                        .fill(p.card_hover)
                        .stroke(Stroke::new(1.0, p.border))
                        .corner_radius(egui::CornerRadius::same(9))
                        .min_size(egui::vec2(0.0, 36.0)),
                )
                .clicked()
            {
                app.controller.clear_finished_jobs();
            }
            if ui
                .add_enabled(
                    active > 0,
                    egui::Button::new(RichText::new(s.stop_all).color(p.text).size(13.0))
                        .fill(p.card_hover)
                        .stroke(Stroke::new(1.0, p.border))
                        .corner_radius(egui::CornerRadius::same(9))
                        .min_size(egui::vec2(0.0, 36.0)),
                )
                .clicked()
            {
                app.controller.stop_all_jobs();
            }
        });
    });
    ui.add_space(16.0);

    if jobs.is_empty() {
        theme::card_frame(p).show(ui, |ui| {
            ui.set_min_width(ui.available_width());
            ui.vertical_centered(|ui| {
                ui.add_space(24.0);
                theme::status_dot(ui, p.muted);
                ui.add_space(6.0);
                theme::muted(ui, p, s.no_downloads);
                ui.add_space(18.0);
                if secondary_button(ui, p, s.nav_new).clicked() {
                    app.controller.set_route(Route::Home);
                }
                ui.add_space(24.0);
            });
        });
        return;
    }

    for job in jobs.iter().rev() {
        let status_color = match job.status {
            JobStatus::Active => p.accent,
            JobStatus::Completed => p.success,
            JobStatus::Failed => p.danger,
            JobStatus::Cancelled => p.warning,
        };
        theme::card_frame(p).show(ui, |ui| {
            ui.set_min_width(ui.available_width());
            ui.horizontal_top(|ui| {
                job_tile(ui, p);
                ui.add_space(4.0);
                let content_width = (ui.available_width() - 140.0).max(220.0);
                ui.allocate_ui_with_layout(
                    egui::vec2(content_width, 0.0),
                    Layout::top_down(Align::LEFT),
                    |ui| {
                        ui.add(
                            egui::Label::new(
                                RichText::new(&job.title).size(15.0).strong().color(p.text),
                            )
                            .truncate(),
                        );
                        ui.add(
                            egui::Label::new(RichText::new(&job.detail).size(12.0).color(p.muted))
                                .truncate(),
                        );
                        ui.add_space(8.0);
                        theme::progress_bar(ui, p, (job.progress() / 100.0) as f32, 8.0);
                        ui.add_space(6.0);
                        ui.horizontal(|ui| {
                            if job.status == JobStatus::Failed {
                                if let Some(error) = &job.error {
                                    ui.label(RichText::new(error).color(p.danger).size(11.5));
                                } else {
                                    ui.label(
                                        RichText::new(s.download_failed).color(p.danger).size(11.5),
                                    );
                                }
                            } else {
                                let label = match job.status {
                                    JobStatus::Active => {
                                        format!("{}  {:.0}%", s.downloading, job.progress())
                                    }
                                    JobStatus::Completed => s.completed.to_owned(),
                                    JobStatus::Cancelled => s.cancel.to_owned(),
                                    JobStatus::Failed => s.download_failed.to_owned(),
                                };
                                ui.label(RichText::new(label).color(status_color).size(11.5));
                            }
                            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                                if job.is_playlist {
                                    theme::muted(
                                        ui,
                                        p,
                                        format!("{} / {}", job.done + job.failed, job.total),
                                    );
                                }
                                if job.status == JobStatus::Active {
                                    let slot = job
                                        .slots
                                        .iter()
                                        .find(|slot| !slot.title.is_empty() || slot.proc);
                                    if let Some(slot) = slot {
                                        if !slot.speed.is_empty() {
                                            theme::muted(ui, p, &slot.speed);
                                        }
                                        if slot.total_b > 0 {
                                            theme::muted(
                                                ui,
                                                p,
                                                format!(
                                                    "{} / {}",
                                                    format_bytes(
                                                        slot.done_b,
                                                        app.controller.state.locale
                                                    ),
                                                    format_bytes(
                                                        slot.total_b,
                                                        app.controller.state.locale
                                                    )
                                                ),
                                            );
                                        }
                                    }
                                } else {
                                    theme::muted(
                                        ui,
                                        p,
                                        format_elapsed(job.elapsed.as_secs() as i64),
                                    );
                                }
                            });
                        });
                    },
                );
                ui.add_space(8.0);
                ui.vertical(|ui| {
                    ui.with_layout(Layout::bottom_up(Align::RIGHT), |ui| {
                        if job.status == JobStatus::Active {
                            if let Some(response) = close_button(ui, p) {
                                let _ = response;
                                app.controller.cancel_job(job.id);
                            }
                        } else if job.status == JobStatus::Failed {
                            if secondary_button_sized(ui, p, s.retry, 118.0).clicked() {
                                app.controller.retry_job(job.id);
                            }
                        } else if secondary_button_sized(ui, p, s.open_folder, 118.0).clicked() {
                            app.controller.open_folder(job.output_dir.clone());
                        }
                    });
                });
            });
        });
        ui.add_space(12.0);
    }
}

fn job_tile(ui: &mut egui::Ui, p: &crate::theme::Palette) {
    let (rect, _) = ui.allocate_exact_size(Vec2::splat(58.0), Sense::hover());
    let painter = ui.painter();
    painter.rect_filled(rect, egui::CornerRadius::same(11), p.card_hover);
    let center = rect.center();
    painter.add(egui::epaint::PathShape::convex_polygon(
        vec![
            center + egui::vec2(-5.0, -8.0),
            center + egui::vec2(-5.0, 8.0),
            center + egui::vec2(8.0, 0.0),
        ],
        p.muted,
        Stroke::NONE,
    ));
}

fn close_button(ui: &mut egui::Ui, p: &crate::theme::Palette) -> Option<egui::Response> {
    let response = ui.add(
        egui::Button::new(RichText::new("×").size(18.0).color(p.muted))
            .fill(p.card_hover)
            .stroke(Stroke::new(1.0, p.border))
            .corner_radius(egui::CornerRadius::same(9))
            .min_size(egui::vec2(38.0, 38.0)),
    );
    theme::press_overlay(ui, &response, 9);
    Some(response)
}
