use gpui::{SharedString, div, prelude::*, px};

use crate::theme::{self, Palette};

const SEGMENT_HEIGHT: f32 = 38.0;
const OPTION_HEIGHT: f32 = 40.0;
const CHIP_HEIGHT: f32 = 32.0;

pub(super) fn segment(
    id: impl Into<gpui::ElementId>,
    label: impl Into<SharedString>,
    selected: bool,
    p: &Palette,
) -> gpui::Stateful<gpui::Div> {
    div()
        .id(id)
        .h(px(SEGMENT_HEIGHT))
        .px_4()
        .flex()
        .items_center()
        .justify_center()
        .rounded_lg()
        .bg(if selected {
            p.selection.opacity(0.16)
        } else {
            p.card_hover
        })
        .border_1()
        .border_color(if selected { p.selection } else { p.border })
        .text_color(if selected { p.selection } else { p.text })
        .text_size(px(13.))
        .font_weight(gpui::FontWeight::SEMIBOLD)
        .cursor_pointer()
        .hover(|this| this.bg(p.card_alt))
        .child(label.into())
}

pub(super) fn option_row(
    id: impl Into<gpui::ElementId>,
    selected: bool,
    label: &str,
    p: &Palette,
) -> gpui::Stateful<gpui::Div> {
    div()
        .id(id)
        .h(px(OPTION_HEIGHT))
        .w_full()
        .px_3()
        .flex()
        .items_center()
        .gap_2()
        .rounded_md()
        .bg(if selected {
            p.selection.opacity(0.12)
        } else {
            p.card_alt
        })
        .border_1()
        .border_color(if selected {
            p.selection.opacity(0.7)
        } else {
            p.border
        })
        .cursor_pointer()
        .hover(|this| this.bg(p.card_hover))
        .child(theme::icon(
            if selected {
                "icons/check.svg"
            } else {
                "icons/info.svg"
            },
            15.,
            if selected { p.selection } else { p.muted },
        ))
        .child(
            div()
                .flex_1()
                .min_w(px(0.))
                .truncate()
                .text_size(px(13.5))
                .text_color(if selected { p.selection } else { p.text })
                .child(label.to_owned()),
        )
}

pub(super) fn track_chip(
    id: impl Into<gpui::ElementId>,
    label: impl Into<SharedString>,
    selected: bool,
    p: &Palette,
) -> gpui::Stateful<gpui::Div> {
    div()
        .id(id)
        .h(px(CHIP_HEIGHT))
        .px_3()
        .flex()
        .items_center()
        .rounded_full()
        .bg(if selected {
            p.selection.opacity(0.16)
        } else {
            p.card_hover
        })
        .border_1()
        .border_color(if selected { p.selection } else { p.border })
        .text_color(if selected { p.selection } else { p.text })
        .text_size(px(12.))
        .cursor_pointer()
        .hover(|this| this.bg(p.card_alt))
        .child(label.into())
}
