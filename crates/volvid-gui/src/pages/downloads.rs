use std::path::PathBuf;

use eframe::egui;
use egui::{Align, CornerRadius, FontId, Layout, Pos2, Rect, RichText, Sense, Stroke, Vec2};
use volvid_app::{JobStatus, Route};
use volvid_core::format_clock;
use volvid_i18n::{format_bytes, format_elapsed};

use crate::app::VolvidApp;
use crate::theme::{self, primary_button, secondary_button, secondary_button_ex};

const TILE_SIZE: Vec2 = Vec2::new(124.0, 80.0);
const TILE_RADIUS: u8 = 11;

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
                ui.add_space(28.0);
            });
        });
    } else {
        for job in state.jobs.iter().rev() {
            job_card(app, ui, p, job, &mut action);
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

fn job_card(
    app: &VolvidApp,
    ui: &mut egui::Ui,
    p: &crate::theme::Palette,
    job: &volvid_app::JobView,
    action: &mut Option<QueueAction>,
) {
    let s = app.s();
    let status_color = status_color(job.status, p);
    let action_width = action_width(ui, job, s);
    theme::card_frame(p).show(ui, |ui| {
        ui.set_min_width(ui.available_width());
        // The row only holds the tile and the text column, so its rect keeps the
        // natural content height. Painting the actions afterwards inside a
        // bounded rect keeps them centered without stretching the card.
        let row = ui.horizontal_top(|ui| {
            job_tile(app, ui, p, job.id);
            let content_width = (ui.available_width() - action_width - 16.0).max(180.0);
            ui.allocate_ui_with_layout(
                egui::vec2(content_width, 0.0),
                Layout::top_down(Align::LEFT),
                |ui| {
                    ui.add(
                        egui::Label::new(theme::strong_text(&job.title, 15.0).color(p.text))
                            .truncate(),
                    )
                    .on_hover_text(&job.title);
                    ui.add(
                        egui::Label::new(RichText::new(&job.detail).size(12.0).color(p.muted))
                            .truncate(),
                    )
                    .on_hover_text(&job.detail);
                    ui.add_space(8.0);
                    theme::progress_bar(ui, p, (job.progress() / 100.0) as f32, 8.0);
                    ui.add_space(6.0);
                    status_line(
                        ui,
                        p,
                        job,
                        s,
                        status_color,
                        app.controller.state.locale,
                        content_width,
                    );
                },
            );
        });
        let row_rect = row.response.rect;
        let actions_rect = Rect::from_min_max(
            Pos2::new(ui.max_rect().right() - action_width, row_rect.top()),
            Pos2::new(ui.max_rect().right(), row_rect.bottom()),
        );
        ui.scope_builder(egui::UiBuilder::new().max_rect(actions_rect), |ui| {
            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                match job.status {
                    JobStatus::Active => {
                        if close_icon(ui, p, s.cancel) {
                            *action = Some(QueueAction::Cancel(job.id));
                        }
                        if icon_action(ui, p, PAUSE_SVG, s.pause) {
                            *action = Some(QueueAction::Pause(job.id));
                        }
                    }
                    JobStatus::Queued | JobStatus::Cancelled => {
                        let queued = job.status == JobStatus::Queued;
                        if close_icon(ui, p, if queued { s.cancel } else { s.remove }) {
                            *action = Some(if queued {
                                QueueAction::Cancel(job.id)
                            } else {
                                QueueAction::Dismiss(job.id)
                            });
                        }
                    }
                    JobStatus::Paused => {
                        if close_icon(ui, p, s.cancel) {
                            *action = Some(QueueAction::Cancel(job.id));
                        }
                        if icon_action(ui, p, PLAY_SVG, s.resume) {
                            *action = Some(QueueAction::Resume(job.id));
                        }
                    }
                    JobStatus::Failed => {
                        if close_icon(ui, p, s.remove) {
                            *action = Some(QueueAction::Dismiss(job.id));
                        }
                        if secondary_button(ui, p, s.retry).clicked() {
                            *action = Some(QueueAction::Retry(job.id));
                        }
                    }
                    JobStatus::Completed => {
                        if close_icon(ui, p, s.remove) {
                            *action = Some(QueueAction::Dismiss(job.id));
                        }
                        if secondary_button(ui, p, s.open_folder).clicked() {
                            *action = Some(QueueAction::Open(job.output_dir.clone()));
                        }
                    }
                }
            });
        });
    });
}

/// Left-side status/error text and right-side metadata on one evenly aligned line.
#[allow(clippy::too_many_arguments)]
fn status_line(
    ui: &mut egui::Ui,
    p: &crate::theme::Palette,
    job: &volvid_app::JobView,
    s: &volvid_i18n::Strings,
    status_color: egui::Color32,
    locale: volvid_core::Locale,
    content_width: f32,
) {
    let status_text = if job.status == JobStatus::Failed {
        job.error
            .clone()
            .unwrap_or_else(|| s.download_failed.to_owned())
    } else {
        status_label(job, s)
    };
    let meta = job_meta(job, s, locale);
    let meta_width = theme::text_width(ui, &meta, FontId::proportional(theme::FONT_SMALL));
    ui.horizontal(|ui| {
        let status_width = if meta.is_empty() {
            content_width
        } else {
            (content_width - meta_width - 16.0).max(60.0)
        };
        ui.allocate_ui_with_layout(
            egui::vec2(status_width, 0.0),
            Layout::left_to_right(Align::Center),
            |ui| {
                ui.add(
                    egui::Label::new(
                        RichText::new(&status_text)
                            .color(status_color)
                            .size(theme::FONT_SMALL),
                    )
                    .truncate(),
                )
                .on_hover_text(&status_text);
            },
        );
        if !meta.is_empty() {
            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                ui.add(
                    egui::Label::new(RichText::new(&meta).color(p.muted).size(theme::FONT_SMALL))
                        .truncate(),
                )
                .on_hover_text(&meta);
            });
        }
    });
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
    if primary_button(ui, p, s.nav_new).clicked() {
        *action = Some(QueueAction::GoHome);
    }
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

fn action_width(ui: &egui::Ui, job: &volvid_app::JobView, s: &volvid_i18n::Strings) -> f32 {
    const LEAD: f32 = 16.0;
    const ICON: f32 = 42.0;
    const GAP: f32 = 8.0;
    let label = |text: &str| theme::text_width(ui, text, theme::semibold(theme::FONT_LABEL)) + 30.0;
    match job.status {
        JobStatus::Active | JobStatus::Paused => LEAD + ICON * 2.0 + GAP,
        JobStatus::Queued | JobStatus::Cancelled => LEAD + ICON,
        JobStatus::Failed => LEAD + ICON + GAP + label(s.retry),
        JobStatus::Completed => LEAD + ICON + GAP + label(s.open_folder),
    }
}

fn close_icon(ui: &mut egui::Ui, p: &crate::theme::Palette, tooltip: &str) -> bool {
    theme::icon_button(
        ui,
        p,
        egui::include_image!("../../../../assets/icons/close.svg"),
        42.0,
        p.muted,
        tooltip,
    )
    .clicked()
}

fn icon_action(
    ui: &mut egui::Ui,
    p: &crate::theme::Palette,
    icon: egui::ImageSource<'static>,
    tooltip: &str,
) -> bool {
    theme::icon_button(ui, p, icon, 42.0, p.text, tooltip).clicked()
}

const PAUSE_SVG: egui::ImageSource = egui::include_image!("../../../../assets/icons/pause.svg");
const PLAY_SVG: egui::ImageSource = egui::include_image!("../../../../assets/icons/play.svg");

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

fn status_label(job: &volvid_app::JobView, s: &volvid_i18n::Strings) -> String {
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

/// Queue tile: fetched thumbnail filling the tile or a subtle play glyph.
fn job_tile(app: &VolvidApp, ui: &mut egui::Ui, p: &crate::theme::Palette, job_id: u64) {
    let (rect, _) = ui.allocate_exact_size(TILE_SIZE, Sense::hover());
    let painter = ui.painter();
    painter.rect_filled(rect, CornerRadius::same(TILE_RADIUS), p.card_hover);
    if let Some(texture) = app.job_preview(job_id) {
        let size = texture.size_vec2();
        let uv = fill_uv(
            size.x.max(1.0) / size.y.max(1.0),
            rect.width() / rect.height(),
        );
        ui.put(
            rect,
            egui::Image::new((texture.id(), rect.size()))
                .uv(uv)
                .corner_radius(CornerRadius::same(TILE_RADIUS)),
        );
    } else {
        let center = rect.center();
        painter.circle_stroke(center, 15.0, Stroke::new(1.5, p.muted.gamma_multiply(0.55)));
        painter.add(egui::epaint::PathShape::convex_polygon(
            vec![
                center + egui::vec2(-3.0, -7.0),
                center + egui::vec2(-3.0, 7.0),
                center + egui::vec2(7.0, 0.0),
            ],
            p.muted,
            Stroke::NONE,
        ));
    }
}

/// Centered UV window so a texture fills the whole box without distortion.
fn fill_uv(image_aspect: f32, box_aspect: f32) -> Rect {
    if image_aspect > box_aspect {
        let width = (box_aspect / image_aspect).clamp(0.0, 1.0);
        let offset = (1.0 - width) / 2.0;
        Rect::from_min_max(Pos2::new(offset, 0.0), Pos2::new(offset + width, 1.0))
    } else {
        let height = (image_aspect / box_aspect).clamp(0.0, 1.0);
        let offset = (1.0 - height) / 2.0;
        Rect::from_min_max(Pos2::new(0.0, offset), Pos2::new(1.0, offset + height))
    }
}
