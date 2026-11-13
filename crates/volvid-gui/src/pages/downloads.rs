use gpui::{Context, SharedString, Window, div, prelude::*, px};
use volvid_app::{JobStatus, Route};
use volvid_i18n::{format_bytes, format_elapsed};

use crate::app::VolvidApp;
use crate::theme::{self, Palette};

pub fn downloads(
    app: &mut VolvidApp,
    _window: &mut Window,
    cx: &mut Context<VolvidApp>,
    p: &Palette,
) -> impl IntoElement {
    let p = *p;
    let s = app.s();
    let locale = app.controller.state.locale;
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
    let has_finished = state.jobs.iter().any(|job| {
        matches!(
            job.status,
            JobStatus::Completed | JobStatus::Failed | JobStatus::Cancelled
        )
    });
    let jobs: Vec<_> = state.jobs.iter().rev().cloned().collect();

    let mut page = div()
        .id("page-downloads")
        .flex()
        .flex_col()
        .gap_4()
        .h_full()
        .overflow_y_scroll()
        .scrollbar_width(px(8.))
        .pb_5()
        .child(
            div()
                .flex()
                .flex_wrap()
                .items_center()
                .gap_3()
                .child(
                    div()
                        .flex()
                        .flex_col()
                        .gap_1()
                        .child(theme::title(s.queue, &p))
                        .child(theme::muted(
                            {
                                let mut parts = vec![
                                    s.downloading_count.replace("%d", &running.to_string()),
                                    s.waiting_count.replace("%d", &queued.to_string()),
                                ];
                                if paused > 0 {
                                    parts.push(s.paused_count.replace("%d", &paused.to_string()));
                                }
                                parts.push(s.errors_count.replace("%d", &failed.to_string()));
                                parts.join("  ·  ")
                            },
                            &p,
                        )),
                )
                .child(div().flex_1())
                .child(
                    theme::secondary_button("queue-clear", s.clear_finished, &p)
                        .when(!has_finished, |this| this.opacity(0.45))
                        .on_click(cx.listener(|this, _, _, cx| {
                            if this.controller.state.has_finished_jobs() {
                                this.controller.clear_finished_jobs();
                            }
                            cx.notify();
                        })),
                )
                .child(
                    theme::secondary_button(
                        "queue-pause",
                        if paused > 0 && !has_pending {
                            s.resume_all
                        } else {
                            s.pause_all
                        },
                        &p,
                    )
                    .when(!has_pending && paused == 0, |this| this.opacity(0.45))
                    .on_click(cx.listener(move |this, _, _, cx| {
                        if has_pending {
                            this.controller.pause_all_jobs();
                        } else if this.controller.state.paused_jobs() > 0 {
                            this.controller.resume_all_jobs();
                        }
                        cx.notify();
                    })),
                )
                .child(
                    theme::primary_button("queue-new", s.nav_new, &p).on_click(cx.listener(
                        |this, _, _, cx| {
                            this.controller.set_route(Route::Home);
                            cx.notify();
                        },
                    )),
                ),
        );

    if jobs.is_empty() {
        page = page.child(theme::card(&p).child(theme::empty_state(
            "icons/download.svg",
            s.no_downloads,
            "",
            &p,
        )));
        return page;
    }

    for job in jobs {
        let color = match job.status {
            JobStatus::Active => p.accent,
            JobStatus::Completed => p.success,
            JobStatus::Failed => p.danger,
            JobStatus::Paused => p.warning,
            _ => p.muted,
        };
        let progress = job.progress() as f32 / 100.0;
        let image = app.job_image(job.id).cloned();
        let has_image = image.is_some();
        let id = job.id;
        let title = job.title.clone();
        let detail = job.detail.clone();
        let output = job.output_dir.clone();
        let status = job.status;
        let mut card = theme::card(&p).flex().gap_4().items_start();
        card = card.child(
            div()
                .w(px(124.))
                .h(px(80.))
                .flex_none()
                .rounded_lg()
                .overflow_hidden()
                .bg(p.track)
                .when_some(image, |this, image| {
                    this.child(
                        gpui::img(image)
                            .size_full()
                            .object_fit(gpui::ObjectFit::Cover),
                    )
                })
                .when(!has_image, |this| {
                    this.flex()
                        .items_center()
                        .justify_center()
                        .child(theme::icon("icons/play.svg", 26., p.muted))
                }),
        );
        card = card.child(
            div()
                .flex_1()
                .min_w(px(0.))
                .flex()
                .flex_col()
                .gap_2()
                .child(
                    div()
                        .flex()
                        .items_center()
                        .gap_2()
                        .child(
                            div()
                                .flex_1()
                                .min_w(px(0.))
                                .truncate()
                                .text_size(px(15.))
                                .font_weight(gpui::FontWeight::SEMIBOLD)
                                .text_color(p.text)
                                .child(title),
                        )
                        .child(theme::badge(
                            match status {
                                JobStatus::Queued => s.queued,
                                JobStatus::Active => s.downloading,
                                JobStatus::Paused => s.paused,
                                JobStatus::Completed => s.completed,
                                JobStatus::Failed => s.failed,
                                JobStatus::Cancelled => s.cancelled,
                            },
                            color,
                            p.card_hover,
                        )),
                )
                .child(
                    div()
                        .truncate()
                        .text_size(px(12.))
                        .text_color(p.muted)
                        .child(detail),
                )
                .child(theme::progress_bar(&p, progress, 7.))
                .child(
                    div()
                        .flex()
                        .items_center()
                        .gap_2()
                        .child(
                            div()
                                .flex_1()
                                .min_w(px(0.))
                                .truncate()
                                .text_size(px(11.5))
                                .text_color(color)
                                .child(status_text(&job, s)),
                        )
                        .child(theme::muted(job_meta(&job, s, locale), &p)),
                ),
        );
        let mut actions = div().flex().flex_col().items_end().gap_2();
        match status {
            JobStatus::Active => {
                actions = actions
                    .child(
                        theme::secondary_button(
                            SharedString::from(format!("job-pause-{id}")),
                            s.pause,
                            &p,
                        )
                        .on_click(cx.listener(move |this, _, _, cx| {
                            this.controller.pause_job(id);
                            cx.notify();
                        })),
                    )
                    .child(
                        theme::danger_button(
                            SharedString::from(format!("job-cancel-{id}")),
                            s.cancel,
                            &p,
                        )
                        .on_click(cx.listener(move |this, _, _, cx| {
                            this.controller.cancel_job(id);
                            cx.notify();
                        })),
                    );
            }
            JobStatus::Paused => {
                actions = actions
                    .child(
                        theme::primary_button(
                            SharedString::from(format!("job-resume-{id}")),
                            s.resume,
                            &p,
                        )
                        .on_click(cx.listener(move |this, _, _, cx| {
                            this.controller.resume_job(id);
                            cx.notify();
                        })),
                    )
                    .child(
                        theme::danger_button(
                            SharedString::from(format!("job-cancel-{id}")),
                            s.cancel,
                            &p,
                        )
                        .on_click(cx.listener(move |this, _, _, cx| {
                            this.controller.cancel_job(id);
                            cx.notify();
                        })),
                    );
            }
            JobStatus::Queued => {
                actions = actions.child(
                    theme::danger_button(
                        SharedString::from(format!("job-cancel-{id}")),
                        s.cancel,
                        &p,
                    )
                    .on_click(cx.listener(move |this, _, _, cx| {
                        this.controller.cancel_job(id);
                        cx.notify();
                    })),
                );
            }
            JobStatus::Failed => {
                actions = actions
                    .child(
                        theme::primary_button(
                            SharedString::from(format!("job-retry-{id}")),
                            s.retry,
                            &p,
                        )
                        .on_click(cx.listener(move |this, _, _, cx| {
                            this.controller.retry_job(id);
                            cx.notify();
                        })),
                    )
                    .child(
                        theme::danger_button(
                            SharedString::from(format!("job-remove-{id}")),
                            s.remove,
                            &p,
                        )
                        .on_click(cx.listener(move |this, _, _, cx| {
                            this.controller.dismiss_job(id);
                            cx.notify();
                        })),
                    );
            }
            JobStatus::Completed => {
                actions = actions
                    .child(
                        theme::secondary_button(
                            SharedString::from(format!("job-open-{id}")),
                            s.open_folder,
                            &p,
                        )
                        .on_click(cx.listener(move |this, _, _, _cx| {
                            this.controller.open_folder(output.clone());
                        })),
                    )
                    .child(
                        theme::ghost_button(
                            SharedString::from(format!("job-dismiss-{id}")),
                            s.remove,
                            &p,
                        )
                        .on_click(cx.listener(move |this, _, _, cx| {
                            this.controller.dismiss_job(id);
                            cx.notify();
                        })),
                    );
            }
            JobStatus::Cancelled => {
                actions = actions.child(
                    theme::ghost_button(
                        SharedString::from(format!("job-dismiss-{id}")),
                        s.remove,
                        &p,
                    )
                    .on_click(cx.listener(move |this, _, _, cx| {
                        this.controller.dismiss_job(id);
                        cx.notify();
                    })),
                );
            }
        }
        card = card.child(actions);
        page = page.child(card);
    }
    page
}

fn status_text(job: &volvid_app::JobView, s: &volvid_i18n::Strings) -> String {
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
        JobStatus::Failed => job
            .error
            .clone()
            .unwrap_or_else(|| s.download_failed.to_owned()),
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
        parts.push(format!("{} / {}", job.finished(), job.total));
    }
    let slot = job
        .slots
        .iter()
        .find(|slot| slot.proc || slot.speed_bps > 0 || slot.total_b > 0)
        .or_else(|| job.slots.first());
    if let Some(slot) = slot {
        if !slot.speed.is_empty() {
            parts.push(slot.speed.clone());
        }
        if slot.total_b > 0 {
            parts.push(format!(
                "{} / {}",
                format_bytes(slot.done_b, locale),
                format_bytes(slot.total_b, locale)
            ));
        }
        if slot.eta_secs > 0 {
            parts.push(format!(
                "{} {}",
                volvid_core::format_clock(slot.eta_secs),
                s.remaining
            ));
        }
    }
    if job.status.is_pending() {
        parts.join("  ·  ")
    } else if job.started {
        parts.push(format_elapsed(job.elapsed.as_secs() as i64, locale));
        parts.join("  ·  ")
    } else {
        parts.join("  ·  ")
    }
}
