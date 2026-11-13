use gpui::{
    Div, ElementId, Hsla, SharedString, Stateful, WindowAppearance, div, linear_color_stop,
    linear_gradient, prelude::*, px, relative, rgb,
};
use volvid_engine::Theme as ConfigTheme;

pub const BUTTON_HEIGHT: f32 = 42.0;
pub const GHOST_HEIGHT: f32 = 36.0;

#[derive(Clone, Copy)]
pub struct Palette {
    pub bg: Hsla,
    pub bg_bottom: Hsla,
    pub panel: Hsla,
    pub card: Hsla,
    pub card_hover: Hsla,
    pub card_alt: Hsla,
    pub border: Hsla,
    pub divider: Hsla,
    pub text: Hsla,
    pub muted: Hsla,
    pub accent: Hsla,
    pub accent_hover: Hsla,
    pub on_accent: Hsla,
    pub selection: Hsla,
    pub success: Hsla,
    pub warning: Hsla,
    pub danger: Hsla,
    pub track: Hsla,
}

fn color(value: u32) -> Hsla {
    rgb(value).into()
}

pub fn palette(theme: ConfigTheme, appearance: WindowAppearance) -> Palette {
    let dark = match theme {
        ConfigTheme::Dark => true,
        ConfigTheme::Light => false,
        ConfigTheme::System => matches!(
            appearance,
            WindowAppearance::Dark | WindowAppearance::VibrantDark
        ),
    };
    if dark {
        dark_palette()
    } else {
        light_palette()
    }
}

fn dark_palette() -> Palette {
    Palette {
        bg: color(0x0C0F17),
        bg_bottom: color(0x07090F),
        panel: color(0x090B11),
        card: color(0x161B26),
        card_hover: color(0x1F2634),
        card_alt: color(0x11151F),
        border: color(0x2D3646),
        divider: color(0x252D3C),
        text: color(0xF2F5FA),
        muted: color(0x9AA6BC),
        accent: color(0xFF6B5E),
        accent_hover: color(0xFF8A78),
        on_accent: color(0x0F0C0C),
        selection: color(0x45D8FF),
        success: color(0x4BE3A4),
        warning: color(0xFFC857),
        danger: color(0xFF6B6B),
        track: color(0x232B3A),
    }
}

fn light_palette() -> Palette {
    Palette {
        bg: color(0xF6F8FC),
        bg_bottom: color(0xECF1F8),
        panel: color(0xFCFDFF),
        card: color(0xFFFFFF),
        card_hover: color(0xF2F5FA),
        card_alt: color(0xF9FBFE),
        border: color(0xDCE2EC),
        divider: color(0xE6EBF3),
        text: color(0x18202D),
        muted: color(0x5E6A7C),
        accent: color(0xC93E2F),
        accent_hover: color(0xB33427),
        on_accent: color(0xFFFFFF),
        selection: color(0x007A9A),
        success: color(0x0E7A50),
        warning: color(0x8A5B00),
        danger: color(0xC22F3E),
        track: color(0xE9EEF5),
    }
}

pub fn input_style(p: &Palette) -> crate::text_input::TextInputStyle {
    crate::text_input::TextInputStyle {
        text: p.text,
        placeholder: p.muted,
        background: p.card_alt,
        border: p.border,
        focus: p.accent,
        cursor: p.accent,
        selection: p.selection.opacity(0.22),
        height: 42.0,
    }
}

pub fn card(p: &Palette) -> Div {
    div()
        .p_4()
        .rounded_xl()
        .bg(p.card)
        .border_1()
        .border_color(p.border)
        .shadow_sm()
}

pub fn hero(p: &Palette) -> Div {
    div()
        .p_5()
        .rounded_xl()
        .border_1()
        .border_color(p.border)
        .bg(linear_gradient(
            135.,
            linear_color_stop(p.card, 0.0),
            linear_color_stop(p.accent.opacity(0.14), 1.0),
        ))
        .shadow_md()
}

pub fn title(text: impl Into<SharedString>, p: &Palette) -> Div {
    div()
        .text_size(px(28.))
        .font_weight(gpui::FontWeight::BOLD)
        .text_color(p.text)
        .child(text.into())
}

pub fn page_title(text: impl Into<SharedString>, p: &Palette) -> Div {
    div()
        .text_size(px(28.))
        .font_weight(gpui::FontWeight::BOLD)
        .text_color(p.text)
        .truncate()
        .child(text.into())
}

pub fn section_title(text: impl Into<SharedString>, p: &Palette) -> Div {
    div()
        .text_size(px(12.5))
        .font_weight(gpui::FontWeight::SEMIBOLD)
        .text_color(p.muted)
        .child(text.into())
}

pub fn muted(text: impl Into<SharedString>, p: &Palette) -> Div {
    div()
        .text_size(px(13.))
        .text_color(p.muted)
        .child(text.into())
}

pub fn body(text: impl Into<SharedString>, p: &Palette) -> Div {
    div()
        .text_size(px(14.))
        .text_color(p.text)
        .child(text.into())
}

/// Inline status/error strip used across pages.
pub fn notice(icon_path: &'static str, text: impl Into<SharedString>, color: Hsla) -> Div {
    div()
        .p_3()
        .rounded_lg()
        .bg(color.opacity(0.1))
        .border_1()
        .border_color(color.opacity(0.35))
        .flex()
        .items_center()
        .gap_3()
        .child(icon(icon_path, 17., color))
        .child(
            div()
                .flex_1()
                .min_w(px(0.))
                .text_size(px(13.))
                .text_color(color)
                .child(text.into()),
        )
}

/// Small loading row for async sections.
pub fn loading(text: impl Into<SharedString>, p: &Palette) -> Div {
    div()
        .flex()
        .items_center()
        .gap_2()
        .child(div().size(px(8.)).rounded_full().bg(p.selection))
        .child(muted(text, p))
}

pub fn primary_button(
    id: impl Into<ElementId>,
    label: impl Into<SharedString>,
    p: &Palette,
) -> Stateful<Div> {
    div()
        .id(id)
        .h(px(BUTTON_HEIGHT))
        .px_4()
        .flex()
        .items_center()
        .justify_center()
        .rounded_lg()
        .bg(linear_gradient(
            180.,
            linear_color_stop(p.accent, 0.0),
            linear_color_stop(p.accent_hover, 1.0),
        ))
        .text_color(p.on_accent)
        .text_size(px(14.))
        .font_weight(gpui::FontWeight::SEMIBOLD)
        .cursor_pointer()
        .hover(|this| this.opacity(0.92))
        .active(|this| this.opacity(0.82))
        .child(label.into())
}

pub fn secondary_button(
    id: impl Into<ElementId>,
    label: impl Into<SharedString>,
    p: &Palette,
) -> Stateful<Div> {
    div()
        .id(id)
        .h(px(BUTTON_HEIGHT))
        .px_4()
        .flex()
        .items_center()
        .justify_center()
        .rounded_lg()
        .bg(p.card_hover)
        .border_1()
        .border_color(p.border)
        .text_color(p.text)
        .text_size(px(14.))
        .font_weight(gpui::FontWeight::SEMIBOLD)
        .cursor_pointer()
        .hover(|this| this.bg(p.card_alt).border_color(p.accent.opacity(0.6)))
        .active(|this| this.opacity(0.8))
        .child(label.into())
}

pub fn danger_button(
    id: impl Into<ElementId>,
    label: impl Into<SharedString>,
    p: &Palette,
) -> Stateful<Div> {
    secondary_button(id, label, p)
        .text_color(p.danger)
        .border_color(p.danger.opacity(0.5))
}

pub fn ghost_button(
    id: impl Into<ElementId>,
    label: impl Into<SharedString>,
    p: &Palette,
) -> Stateful<Div> {
    div()
        .id(id)
        .h(px(GHOST_HEIGHT))
        .px_3()
        .flex()
        .items_center()
        .justify_center()
        .rounded_md()
        .text_color(p.muted)
        .text_size(px(13.))
        .cursor_pointer()
        .hover(|this| this.bg(p.card_hover).text_color(p.text))
        .child(label.into())
}

pub fn badge(text: impl Into<SharedString>, fg: Hsla, bg: Hsla) -> Div {
    div()
        .px_2()
        .py_1()
        .rounded_full()
        .bg(bg)
        .border_1()
        .border_color(fg.opacity(0.4))
        .text_color(fg)
        .text_size(px(11.))
        .font_weight(gpui::FontWeight::SEMIBOLD)
        .child(text.into())
}

pub fn icon(path: &'static str, size: f32, color: Hsla) -> impl IntoElement {
    gpui::svg().path(path).size(px(size)).text_color(color)
}

pub fn progress_bar(p: &Palette, fraction: f32, height: f32) -> Div {
    let fraction = if fraction.is_finite() {
        fraction.clamp(0.0, 1.0)
    } else {
        0.0
    };
    div()
        .h(px(height))
        .w_full()
        .rounded_full()
        .bg(p.track)
        .overflow_hidden()
        .child(
            div()
                .h_full()
                .w(relative(fraction))
                .rounded_full()
                .bg(linear_gradient(
                    90.,
                    linear_color_stop(p.accent, 0.0),
                    linear_color_stop(p.selection, 1.0),
                )),
        )
}

pub fn empty_state(
    icon_path: &'static str,
    title: impl Into<SharedString>,
    subtitle: impl Into<SharedString>,
    p: &Palette,
) -> Div {
    div()
        .flex()
        .flex_col()
        .items_center()
        .justify_center()
        .gap_3()
        .py_8()
        .child(
            div()
                .size(px(52.))
                .rounded_full()
                .bg(p.card_hover)
                .flex()
                .items_center()
                .justify_center()
                .child(icon(icon_path, 24., p.accent)),
        )
        .child(
            div()
                .text_size(px(15.))
                .font_weight(gpui::FontWeight::SEMIBOLD)
                .text_color(p.text)
                .child(title.into()),
        )
        .child(muted(subtitle, p))
}

pub fn divider(p: &Palette) -> Div {
    div().h(px(1.)).w_full().bg(p.divider)
}

pub const FONT_FAMILY: &str = "Onest";
