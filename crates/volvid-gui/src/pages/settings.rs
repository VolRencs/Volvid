use std::path::PathBuf;

use gpui::{Context, SharedString, Window, div, prelude::*, px};

use super::components::segment;
use crate::app::VolvidApp;
use crate::theme::{self, Palette};

pub fn settings(
    app: &mut VolvidApp,
    _window: &mut Window,
    cx: &mut Context<VolvidApp>,
    p: &Palette,
) -> impl IntoElement {
    let p = *p;
    let s = app.s();
    let locale = app.controller.state.locale;
    let theme = app.controller.state.config.theme;
    let deps = app.controller.state.deps.clone();
    let installing = app.controller.state.installing_dep.clone();
    let progress = app.controller.state.deps_progress.clone();
    let folder = app.controller.state.downloads_dir.clone();
    let logs = app.controller.state.logs_dir.clone();
    let config = app.controller.paths().config_dir.clone();
    let update = app.controller.state.update.clone();
    let checking = app.controller.state.update_checking;

    let mut page = div()
        .id("page-settings")
        .flex()
        .flex_col()
        .gap_4()
        .h_full()
        .overflow_y_scroll()
        .scrollbar_width(px(8.))
        .pb_5()
        .child(theme::title(s.settings_title, &p));

    page = page.child(
        theme::card(&p)
            .flex()
            .flex_col()
            .gap_4()
            .child(theme::section_title(s.appearance, &p))
            .child(setting_row(
                s.select_language,
                div()
                    .flex()
                    .gap_2()
                    .child(
                        segment(
                            "locale-en",
                            "English",
                            locale == volvid_core::Locale::En,
                            &p,
                        )
                        .on_click(cx.listener(|this, _, _, cx| {
                            this.controller.set_locale(volvid_core::Locale::En);
                            cx.notify();
                        })),
                    )
                    .child(
                        segment(
                            "locale-ru",
                            "Русский",
                            locale == volvid_core::Locale::Ru,
                            &p,
                        )
                        .on_click(cx.listener(|this, _, _, cx| {
                            this.controller.set_locale(volvid_core::Locale::Ru);
                            cx.notify();
                        })),
                    ),
                &p,
            ))
            .child(setting_row(
                s.theme,
                div()
                    .flex()
                    .gap_2()
                    .child(
                        segment(
                            "theme-light",
                            s.theme_light,
                            theme == volvid_engine::Theme::Light,
                            &p,
                        )
                        .on_click(cx.listener(|this, _, _, cx| {
                            this.controller.set_theme(volvid_engine::Theme::Light);
                            cx.notify();
                        })),
                    )
                    .child(
                        segment(
                            "theme-dark",
                            s.theme_dark,
                            theme == volvid_engine::Theme::Dark,
                            &p,
                        )
                        .on_click(cx.listener(|this, _, _, cx| {
                            this.controller.set_theme(volvid_engine::Theme::Dark);
                            cx.notify();
                        })),
                    )
                    .child(
                        segment(
                            "theme-system",
                            s.theme_system,
                            theme == volvid_engine::Theme::System,
                            &p,
                        )
                        .on_click(cx.listener(|this, _, _, cx| {
                            this.controller.set_theme(volvid_engine::Theme::System);
                            cx.notify();
                        })),
                    ),
                &p,
            )),
    );

    page = page.child(
        theme::card(&p)
            .flex()
            .flex_col()
            .gap_3()
            .child(theme::section_title(s.paths, &p))
            .child(path_row(
                "settings-downloads",
                s.downloads_dir,
                folder.clone(),
                s.open_folder,
                Some(s.choose_folder),
                &p,
                cx,
            ))
            .child(path_row(
                "settings-logs",
                s.logs,
                logs.clone(),
                s.open_logs,
                None,
                &p,
                cx,
            ))
            .child(path_row(
                "settings-config",
                s.config_dir,
                config.clone(),
                "",
                None,
                &p,
                cx,
            )),
    );

    let mut deps_card = theme::card(&p).flex().flex_col().gap_3().child(
        div()
            .flex()
            .items_center()
            .gap_2()
            .child(theme::section_title(s.deps_title, &p))
            .when(app.controller.state.deps_checking, |this| {
                this.child(theme::loading(s.dep_checking, &p))
            })
            .child(div().flex_1())
            .child(
                theme::ghost_button("deps-refresh", s.dep_refresh, &p).on_click(cx.listener(
                    |this, _, _, cx| {
                        this.controller.refresh_deps();
                        cx.notify();
                    },
                )),
            ),
    );
    let node_inactive = deps.runtime.status != volvid_core::STATUS_ACTIVE;
    for dep in deps.dependencies() {
        deps_card = deps_card.child(dependency_row(
            dep,
            installing.as_deref(),
            progress.as_ref(),
            dep.key == "node" && node_inactive,
            &p,
            locale,
            cx,
        ));
    }
    deps_card = deps_card
        .child(theme::divider(&p))
        .child(theme::section_title(s.environment, &p))
        .child(access_row(
            s.cookies,
            &deps.cookies.status,
            &cookies_detail(&deps.cookies),
            &p,
            locale,
        ))
        .child(access_row(
            s.js_runtime,
            &deps.runtime.status,
            &deps.runtime.name,
            &p,
            locale,
        ));
    page = page.child(deps_card);

    let mut about = theme::card(&p)
        .flex()
        .flex_col()
        .gap_3()
        .child(theme::section_title(s.about, &p))
        .child(
            div()
                .flex()
                .items_center()
                .gap_2()
                .child(theme::body(s.current_version, &p))
                .child(theme::muted(app.controller.app_version(), &p)),
        )
        .child(
            div()
                .flex()
                .items_center()
                .gap_2()
                .child(
                    theme::secondary_button("settings-history", s.clear_history, &p)
                        .when(app.controller.state.history.is_empty(), |this| {
                            this.opacity(0.45)
                        })
                        .on_click(cx.listener(|this, _, _, cx| {
                            if !this.controller.state.history.is_empty() {
                                this.controller.clear_history();
                            }
                            cx.notify();
                        })),
                )
                .child(div().flex_1()),
        );
    if checking {
        about = about.child(theme::muted(s.checking, &p));
    } else if let Some(update) = update {
        about = about.child(
            div()
                .flex()
                .items_center()
                .gap_2()
                .child(theme::badge(
                    format!("{} {}", s.latest_version, update.latest),
                    p.warning,
                    p.card_hover,
                ))
                .child(
                    theme::secondary_button("settings-update", s.install_update, &p).on_click(
                        cx.listener(|this, _, _, cx| {
                            this.controller.state.update_modal = true;
                            cx.notify();
                        }),
                    ),
                ),
        );
    } else {
        about = about.child(
            theme::secondary_button("settings-check", s.check_now, &p).on_click(cx.listener(
                |this, _, _, cx| {
                    this.controller.check_updates();
                    cx.notify();
                },
            )),
        );
    }
    page.child(about)
}

fn setting_row(label: &str, control: impl IntoElement, p: &Palette) -> impl IntoElement {
    div()
        .flex()
        .flex_wrap()
        .items_center()
        .gap_3()
        .child(div().w(px(220.)).child(theme::body(label.to_owned(), p)))
        .child(div().flex_1())
        .child(control)
}

fn path_row(
    id: &'static str,
    label: &str,
    path: PathBuf,
    open_label: &str,
    choose: Option<&str>,
    p: &Palette,
    cx: &mut Context<VolvidApp>,
) -> impl IntoElement {
    let open_path = path.clone();
    let mut row = div()
        .id(id)
        .flex()
        .flex_wrap()
        .items_center()
        .gap_2()
        .child(div().w(px(180.)).child(theme::body(label.to_owned(), p)))
        .child(
            div()
                .flex_1()
                .min_w(px(180.))
                .truncate()
                .p_2()
                .rounded_md()
                .bg(p.card_alt)
                .border_1()
                .border_color(p.border)
                .text_size(px(12.))
                .text_color(p.muted)
                .child(path.to_string_lossy().into_owned()),
        );
    if !open_label.is_empty() {
        row = row.child(
            theme::secondary_button(
                SharedString::from(format!("{id}-open")),
                open_label.to_owned(),
                p,
            )
            .on_click(cx.listener(move |this, _, _, _| {
                this.controller.open_folder(open_path.clone());
            })),
        );
    }
    if let Some(label) = choose {
        row = row.child(
            theme::secondary_button(
                SharedString::from(format!("{id}-choose")),
                label.to_owned(),
                p,
            )
            .on_click(cx.listener(|this, _, _, _| this.controller.pick_folder())),
        );
    }
    row
}

fn dependency_row(
    dep: &volvid_core::DependencyInfo,
    installing: Option<&str>,
    progress: Option<&volvid_core::FileProgress>,
    runtime_needs_install: bool,
    p: &Palette,
    locale: volvid_core::Locale,
    cx: &mut Context<VolvidApp>,
) -> impl IntoElement {
    let s = volvid_i18n::strings(locale);
    let color = if dep.available {
        p.success
    } else if dep.required {
        p.danger
    } else {
        p.muted
    };
    let key = dep.key.clone();
    let busy = installing.is_some();
    let installing_this = installing == Some(dep.key.as_str());
    let needs_install = dep.downloadable
        && (!dep.available || dep.source == volvid_core::DependencySource::Managed);
    let mut row = div()
        .id(SharedString::from(format!("dep-{}", dep.key)))
        .flex()
        .flex_col()
        .gap_2()
        .p_3()
        .rounded_lg()
        .bg(p.card_alt)
        .border_1()
        .border_color(p.border)
        .child(
            div()
                .flex()
                .items_center()
                .gap_2()
                .child(div().size(px(8.)).rounded_full().bg(color))
                .child(theme::body(dep.name.clone(), p))
                .child(theme::badge(
                    if dep.required {
                        s.dep_required
                    } else {
                        s.dep_optional
                    },
                    p.muted,
                    p.card_hover,
                ))
                .child(theme::muted(
                    if dep.version.is_empty() {
                        s.dep_missing.to_owned()
                    } else {
                        dep.version.clone()
                    },
                    p,
                ))
                .child(div().flex_1())
                .child(theme::badge(
                    match dep.source {
                        volvid_core::DependencySource::System => s.dep_system,
                        volvid_core::DependencySource::Managed => s.dep_managed,
                        volvid_core::DependencySource::Missing => s.dep_missing,
                    },
                    p.muted,
                    p.card_hover,
                ))
                .when(installing_this, |this| {
                    this.child(
                        theme::ghost_button(
                            SharedString::from(format!("dep-cancel-{}", dep.key)),
                            s.cancel,
                            p,
                        )
                        .on_click(cx.listener(|this, _, _, cx| {
                            this.controller.cancel_install();
                            cx.notify();
                        })),
                    )
                })
                .when(
                    !installing_this && (needs_install || runtime_needs_install),
                    |this| {
                        this.child(
                            theme::secondary_button(
                                SharedString::from(format!("dep-install-{}", dep.key)),
                                if dep.available {
                                    format!("{} {}", s.dep_update, dep.name)
                                } else {
                                    format!("{} {}", s.dep_install, dep.name)
                                },
                                p,
                            )
                            .when(busy, |this| this.opacity(0.5))
                            .on_click(cx.listener(
                                move |this, _, _, cx| {
                                    if this.controller.state.installing_dep.is_none() {
                                        this.controller.install_dependency(&key);
                                    }
                                    cx.notify();
                                },
                            )),
                        )
                    },
                ),
        );
    if installing == Some(dep.key.as_str())
        && let Some(progress) = progress
    {
        row = row
            .child(theme::progress_bar(p, (progress.pct / 100.0) as f32, 6.))
            .child(theme::muted(
                format!("{:.1}%  {}", progress.pct, progress.speed),
                p,
            ));
    }
    row
}

fn access_row(
    label: &str,
    status: &str,
    detail: &str,
    p: &Palette,
    locale: volvid_core::Locale,
) -> impl IntoElement {
    let s = volvid_i18n::strings(locale);
    let active = status == volvid_core::STATUS_ACTIVE;
    div()
        .flex()
        .items_center()
        .gap_2()
        .child(
            div()
                .size(px(8.))
                .rounded_full()
                .bg(if active { p.success } else { p.muted }),
        )
        .child(theme::body(label.to_owned(), p))
        .child(theme::muted(detail.to_owned(), p))
        .child(div().flex_1())
        .child(theme::badge(
            match status {
                volvid_core::STATUS_ACTIVE => s.status_active.to_owned(),
                volvid_core::STATUS_NOT_FOUND => s.status_not_found.to_owned(),
                volvid_core::STATUS_NO_PROFILE => s.status_no_profile.to_owned(),
                other => other.to_owned(),
            },
            if active { p.success } else { p.muted },
            p.card_hover,
        ))
}

fn cookies_detail(cookies: &volvid_core::BrowserCookiesInfo) -> String {
    if cookies.profile_name.trim().is_empty() {
        cookies.browser.clone()
    } else {
        format!("{}:{}", cookies.browser, cookies.profile_name)
    }
}
