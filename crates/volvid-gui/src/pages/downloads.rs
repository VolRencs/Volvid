use std::path::PathBuf;

use eframe::egui;
use egui::{Align, Layout, RichText, Sense, Stroke, Vec2};
use volvid_app::{JobStatus, Route};
use volvid_core::format_clock;
use volvid_i18n::{format_bytes, format_elapsed};

use crate::app::VolvidApp;
use crate::theme::{self, secondary_button, secondary_button_ex, secondary_button_sized};

enum QueueAction {
    Pause(u64),
    Resume(u64),
    Cancel(u64),
    Retry(u64),
    Dismiss(u64),
    Open(PathBuf),
    PauseAll,
    ResumeAll,
    ClearFinished,
    GoHome,
}

pub fn show(app: &mut VolvidApp, ui: &mut egui::Ui) {
    let p = app.palette();
    let s = app.s();
    let state = &app.controller.state;
    let running = state.active_jobs();
    let queued = state.queued_jobs();
    let paused = state.paused_jobs();
    let failed = state
        .jobs
        .iter()
        .filter(|job| job.status == JobStatus::Failed)
        .count();
    let has_pending = state
        .jobs
        .iter()
        .any(|job| matches!(job.status, JobStatus::Active | JobStatus::Queued));
    let has_finished = state.jobs.iter().any(|job| !job.status.is_pending());
    let mut action = None;

    let mut parts = vec![s.downloading_count.replace("%d", &running.to_string())];
    if queued > 0 {
        parts.push(s.waiting_count.replace("%d", &queued.to_string()));
    }
    if paused > 0 {
        parts.push(s.paused_count.replace("%d", &paused.to_string()));
    }
    if failed > 0 {
        parts.push(s.errors_count.replace("%d", &failed.to_string()));
    }
    let stats = parts.join("  ·  ");

    if ui.available_width() < 760.0 {
        ui.vertical(|ui| {
            theme::title(ui, p, s.queue);
            theme::muted(ui, p, stats.clone());
            ui.add_space(8.0);
            ui.horizontal(|ui| {
                queue_actions(ui, p, s, has_finished, has_pending, paused, &mut action);
            });
        });
    } else {
        ui.horizontal(|ui| {
            ui.vertical(|ui| {
                theme::title(ui, p, s.queue);
                theme::muted(ui, p, stats);
            });
            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                queue_actions(ui, p, s, has_finished, has_pending, paused, &mut action);
            });
        });
    }
    ui.add_space(16.0);

    if state.jobs.is_empty() {
        theme::card_frame(p).show(ui, |ui| {
            ui.set_min_width(ui.available_width());
            ui.vertical_centered(|ui| {
                ui.add_space(28.0);
                theme::status_dot(ui, p.muted);
                ui.add_space(6.0);
                theme::muted(ui, p, s.no_downloads);
                ui.add_space(18.0);
                if secondary_button(ui, p, s.nav_new).clicked() {
                    action = Some(QueueAction::GoHome);
                }
                ui.add_space(28.0);
            });
        });
    } else {
        for job in state.jobs.iter().rev() {
            let status_color = status_color(job.status, p);
            theme::card_frame(p).show(ui, |ui| {
                ui.set_min_width(ui.available_width());
                ui.horizontal_top(|ui| {
                    job_tile(ui, p);
                    let action_width = action_width(job.status);
                    let content_width = (ui.available_width() - action_width - 16.0).max(180.0);
                    ui.allocate_ui_with_layout(
                        egui::vec2(content_width, 0.0),
                        Layout::top_down(Align::LEFT),
                        |ui| {
                            ui.add(
                                egui::Label::new(
                                    theme::strong_text(&job.title, 15.0).color(p.text),
                                )
                                .truncate(),
                            );
                            ui.add(
                                egui::Label::new(
                                    RichText::new(&job.detail).size(12.0).color(p.muted),
                                )
                                .truncate(),
                            );
                            ui.add_space(8.0);
                            theme::progress_bar(ui, p, (job.progress() / 100.0) as f32, 8.0);
                            ui.add_space(6.0);
                            ui.horizontal(|ui| {
                                if job.status == JobStatus::Failed {
                                    let text = job.error.as_deref().unwrap_or(s.download_failed);
                                    ui.add(
                                        egui::Label::new(
                                            RichText::new(text).color(p.danger).size(11.5),
                                        )
                                        .truncate(),
                                    )
                                    .on_hover_text(text);
                                } else {
                                    let label = status_label(job, s, status_color);
                                    ui.add(
                                        egui::Label::new(
                                            RichText::new(&label)
                                                .color(status_color)
                                                .size(theme::FONT_SMALL),
                                        )
                                        .truncate(),
                                    );
                                }
                                ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                                    let meta = job_meta(job, s, state.locale);
                                    if !meta.is_empty() {
                                        ui.add(
                                            egui::Label::new(
                                                RichText::new(meta).size(11.0).color(p.muted),
                                            )
                                            .truncate(),
                                        );
                                    }
                                });
                            });
                        },
                    );
                    ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                        ui.add_space(12.0);
                        match job.status {
                            JobStatus::Active => {
                                if theme::icon_button(
                                    ui,
                                    p,
                                    egui::include_image!("../../../../assets/icons/close.svg"),
                                    42.0,
                                    p.muted,
                                    s.cancel,
                                )
                                .clicked()
                                {
                                    action = Some(QueueAction::Cancel(job.id));
                                }
                                if theme::icon_button(
                                    ui,
                                    p,
                                    egui::include_image!("../../../../assets/icons/pause.svg"),
                                    42.0,
                                    p.text,
                                    s.pause,
                                )
                                .clicked()
                                {
                                    action = Some(QueueAction::Pause(job.id));
                                }
                            }
                            JobStatus::Queued | JobStatus::Cancelled => {
                                if theme::icon_button(
                                    ui,
                                    p,
                                    egui::include_image!("../../../../assets/icons/close.svg"),
                                    42.0,
                                    p.muted,
                                    if job.status == JobStatus::Queued {
                                        s.cancel
                                    } else {
                                        s.remove
                                    },
                                )
                                .clicked()
                                {
                                    action = Some(if job.status == JobStatus::Queued {
                                        QueueAction::Cancel(job.id)
                                    } else {
                                        QueueAction::Dismiss(job.id)
                                    });
                                }
                            }
                            JobStatus::Paused => {
                                if theme::icon_button(
                                    ui,
                                    p,
                                    egui::include_image!("../../../../assets/icons/close.svg"),
                                    42.0,
                                    p.muted,
                                    s.cancel,
                                )
                                .clicked()
                                {
                                    action = Some(QueueAction::Cancel(job.id));
                                }
                                if theme::icon_button(
                                    ui,
                                    p,
                                    egui::include_image!("../../../../assets/icons/play.svg"),
                                    42.0,
                                    p.text,
                                    s.resume,
                                )
                                .clicked()
                                {
                                    action = Some(QueueAction::Resume(job.id));
                                }
                            }
                            JobStatus::Failed => {
                                if theme::icon_button(
                                    ui,
                                    p,
                                    egui::include_image!("../../../../assets/icons/close.svg"),
                                    42.0,
                                    p.muted,
                                    s.remove,
                                )
                                .clicked()
                                {
                                    action = Some(QueueAction::Dismiss(job.id));
                                }
                                if secondary_button_sized(ui, p, s.retry, 110.0).clicked() {
                                    action = Some(QueueAction::Retry(job.id));
                                }
                            }
                            JobStatus::Completed => {
                                if theme::icon_button(
                                    ui,
                                    p,
                                    egui::include_image!("../../../../assets/icons/close.svg"),
                                    42.0,
                                    p.muted,
                                    s.remove,
                                )
                                .clicked()
                                {
                                    action = Some(QueueAction::Dismiss(job.id));
                                }
                                if secondary_button_sized(ui, p, s.open_folder, 136.0).clicked() {
                                    action = Some(QueueAction::Open(job.output_dir.clone()));
                                }
                            }
                        }
                    });
                });
            });
            ui.add_space(12.0);
        }
    }

    match action {
        Some(QueueAction::Pause(id)) => app.controller.pause_job(id),
        Some(QueueAction::Resume(id)) => app.controller.resume_job(id),
        Some(QueueAction::Cancel(id)) => app.controller.cancel_job(id),
        Some(QueueAction::Retry(id)) => app.controller.retry_job(id),
        Some(QueueAction::Dismiss(id)) => app.controller.dismiss_job(id),
        Some(QueueAction::Open(path)) => app.controller.open_folder(path),
        Some(QueueAction::PauseAll) => app.controller.pause_all_jobs(),
        Some(QueueAction::ResumeAll) => app.controller.resume_all_jobs(),
        Some(QueueAction::ClearFinished) => app.controller.clear_finished_jobs(),
        Some(QueueAction::GoHome) => app.controller.set_route(Route::Home),
        None => {}
    }
}

fn queue_actions(
    ui: &mut egui::Ui,
    p: &crate::theme::Palette,
    s: &volvid_i18n::Strings,
    has_finished: bool,
    has_pending: bool,
    paused: usize,
    action: &mut Option<QueueAction>,
) {
    let clear = ui
        .add_enabled_ui(has_finished, |ui| {
            secondary_button_ex(
                ui,
                p,
                s.clear_finished,
                egui::vec2(0.0, theme::BUTTON_HEIGHT),
            )
        })
        .inner
        .clicked();
    if clear {
        *action = Some(QueueAction::ClearFinished);
    }
    let pause_label = if paused > 0 && !has_pending {
        s.resume_all
    } else {
        s.pause_all
    };
    let pause = ui
        .add_enabled_ui(has_pending || paused > 0, |ui| {
            secondary_button_ex(ui, p, pause_label, egui::vec2(0.0, theme::BUTTON_HEIGHT))
        })
        .inner
        .clicked();
    if pause {
        *action = Some(if has_pending {
            QueueAction::PauseAll
        } else {
            QueueAction::ResumeAll
        });
    }
}

fn action_width(status: JobStatus) -> f32 {
    match status {
        JobStatus::Active | JobStatus::Paused => 108.0,
        JobStatus::Failed => 170.0,
        JobStatus::Completed => 198.0,
        JobStatus::Queued | JobStatus::Cancelled => 58.0,
    }
}

fn status_color(status: JobStatus, p: &crate::theme::Palette) -> egui::Color32 {
    match status {
        JobStatus::Queued => p.muted,
        JobStatus::Active => p.accent,
        JobStatus::Paused => p.warning,
        JobStatus::Completed => p.success,
        JobStatus::Failed => p.danger,
        JobStatus::Cancelled => p.muted,
    }
}

fn status_label(
    job: &volvid_app::JobView,
    s: &volvid_i18n::Strings,
    _color: egui::Color32,
) -> String {
    match job.status {
        JobStatus::Queued => s.queued.to_owned(),
        JobStatus::Active => job
            .slots
            .iter()
            .find(|slot| slot.proc && !slot.label.is_empty())
            .map(|slot| slot.label.clone())
            .unwrap_or_else(|| format!("{}  {:.0}%", s.downloading, job.progress())),
        JobStatus::Paused => s.paused.to_owned(),
        JobStatus::Completed => s.completed.to_owned(),
        JobStatus::Failed => s.download_failed.to_owned(),
        JobStatus::Cancelled => s.cancelled.to_owned(),
    }
}

fn job_meta(
    job: &volvid_app::JobView,
    s: &volvid_i18n::Strings,
    locale: volvid_core::Locale,
) -> String {
    let mut parts = Vec::new();
    if job.is_playlist && job.total > 0 {
        parts.push(format!("{} / {}", job.done + job.failed, job.total));
    }
    match job.status {
        JobStatus::Active => {
            if let Some(slot) = job
                .slots
                .iter()
                .find(|slot| slot.total_b > 0 || slot.proc)
                .or_else(|| job.slots.first())
            {
                if job.is_playlist && !slot.title.is_empty() {
                    parts.push(slot.title.clone());
                }
                if !slot.speed.is_empty() {
                    parts.push(slot.speed.clone());
                }
                if slot.total_b > 0 {
                    parts.push(format!(
                        "{} / {}",
                        format_bytes(slot.done_b, locale),
                        format_bytes(slot.total_b, locale)
                    ));
                    if slot.speed_bps > 0 && slot.total_b > slot.done_b {
                        let seconds = (slot.total_b - slot.done_b) / slot.speed_bps;
                        parts.push(format!("{} {}", s.remaining, format_clock(seconds)));
                    }
                }
            }
        }
        JobStatus::Completed | JobStatus::Failed | JobStatus::Cancelled if job.started => {
            parts.push(format_elapsed(job.elapsed.as_secs() as i64, locale));
        }
        _ => {}
    }
    parts.join("  ·  ")
}

fn job_tile(ui: &mut egui::Ui, p: &crate::theme::Palette) {
    let (rect, _) = ui.allocate_exact_size(Vec2::new(124.0, 80.0), Sense::hover());
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
