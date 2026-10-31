use std::sync::Arc;

use eframe::CreationContext;
use egui::{
    Align, Atom, Color32, CornerRadius, FontData, FontDefinitions, FontFamily, FontId, Layout,
    Margin, RichText, Stroke, TextStyle, Vec2, Visuals,
};
use volvid_engine::Theme as ConfigTheme;

const FONT_REGULAR: &str = "onest";
const FONT_SEMIBOLD: &str = "onest-semibold";
const SEMIBOLD_FAMILY: &str = "onest-semibold-family";

pub const BUTTON_HEIGHT: f32 = 42.0;
pub const INPUT_HEIGHT: f32 = 42.0;
pub const BUTTON_RADIUS: u8 = 10;
pub const CONTROL_HEIGHT: f32 = 42.0;
pub const CARD_RADIUS: u8 = 14;
pub const ROW_RADIUS: u8 = 8;
pub const CARD_PADDING: i8 = 18;
pub const PAGE_MARGIN_X: i8 = 42;
pub const PAGE_MARGIN_Y: i8 = 22;
pub const CARD_GAP: f32 = 14.0;
pub const SECTION_GAP: f32 = 8.0;
pub const PAGE_HEADER_GAP: f32 = 16.0;
pub const FONT_PAGE_TITLE: f32 = 36.0;
pub const FONT_SECTION: f32 = 12.5;
pub const FONT_BODY: f32 = 15.0;
pub const FONT_LABEL: f32 = 14.0;
pub const FONT_MUTED: f32 = 13.0;
pub const FONT_SMALL: f32 = 11.5;
pub const SETTINGS_LABEL_W: f32 = 220.0;
pub const SETTINGS_LABEL_W_NARROW: f32 = 180.0;
pub const SETTINGS_BREAKPOINT: f32 = 720.0;
pub const MODAL_WIDTH_LG: f32 = 600.0;
pub const MODAL_WIDTH_MD: f32 = 512.0;
pub const MODAL_WIDTH_SM: f32 = 480.0;

pub struct Palette {
    pub bg: Color32,
    pub panel: Color32,
    pub card: Color32,
    pub card_hover: Color32,
    pub border: Color32,
    pub text: Color32,
    pub muted: Color32,
    pub accent: Color32,
    pub accent_hover: Color32,
    pub on_accent: Color32,
    pub selection: Color32,
    pub success: Color32,
    pub warning: Color32,
    pub danger: Color32,
    pub track: Color32,
}

pub const DARK: Palette = Palette {
    bg: Color32::from_rgb(15, 17, 21),
    panel: Color32::from_rgb(11, 13, 16),
    card: Color32::from_rgb(23, 26, 33),
    card_hover: Color32::from_rgb(31, 35, 44),
    border: Color32::from_rgb(42, 47, 58),
    text: Color32::from_rgb(236, 238, 242),
    muted: Color32::from_rgb(163, 170, 184),
    accent: Color32::from_rgb(255, 90, 79),
    accent_hover: Color32::from_rgb(255, 116, 105),
    on_accent: Color32::from_rgb(15, 17, 21),
    selection: Color32::from_rgb(92, 200, 232),
    success: Color32::from_rgb(74, 222, 154),
    warning: Color32::from_rgb(245, 177, 66),
    danger: Color32::from_rgb(255, 90, 79),
    track: Color32::from_rgb(31, 35, 44),
};

pub const LIGHT: Palette = Palette {
    bg: Color32::from_rgb(245, 246, 248),
    panel: Color32::from_rgb(252, 252, 253),
    card: Color32::from_rgb(255, 255, 255),
    card_hover: Color32::from_rgb(242, 244, 247),
    border: Color32::from_rgb(218, 222, 229),
    text: Color32::from_rgb(23, 26, 33),
    muted: Color32::from_rgb(104, 112, 126),
    accent: Color32::from_rgb(230, 68, 56),
    accent_hover: Color32::from_rgb(200, 52, 42),
    on_accent: Color32::WHITE,
    selection: Color32::from_rgb(0, 139, 177),
    success: Color32::from_rgb(20, 156, 94),
    warning: Color32::from_rgb(184, 116, 9),
    danger: Color32::from_rgb(209, 55, 65),
    track: Color32::from_rgb(238, 241, 245),
};

pub fn palette(theme: egui::Theme) -> &'static Palette {
    match theme {
        egui::Theme::Light => &LIGHT,
        egui::Theme::Dark => &DARK,
    }
}

pub fn resolve_theme(ctx: &egui::Context, theme: ConfigTheme) -> egui::Theme {
    match theme {
        ConfigTheme::Light => egui::Theme::Light,
        ConfigTheme::Dark => egui::Theme::Dark,
        ConfigTheme::System => ctx.system_theme().unwrap_or(egui::Theme::Dark),
    }
}

pub fn setup(cc: &CreationContext<'_>, theme: ConfigTheme) {
    egui_extras::install_image_loaders(&cc.egui_ctx);
    install_fonts(&cc.egui_ctx);
    apply_theme(&cc.egui_ctx, theme);
}

pub fn semibold(size: f32) -> FontId {
    FontId::new(size, FontFamily::Name(SEMIBOLD_FAMILY.into()))
}

pub fn strong_text(text: impl Into<String>, size: f32) -> RichText {
    RichText::new(text.into()).font(semibold(size))
}

pub fn title(ui: &mut egui::Ui, p: &Palette, text: &str) {
    ui.label(strong_text(text, FONT_PAGE_TITLE).color(p.text));
}

pub fn install_fonts(ctx: &egui::Context) {
    let mut fonts = FontDefinitions::default();
    fonts.font_data.insert(
        FONT_REGULAR.to_owned(),
        Arc::new(FontData::from_static(include_bytes!(
            "../../../assets/fonts/Onest-Regular.ttf"
        ))),
    );
    fonts.font_data.insert(
        FONT_SEMIBOLD.to_owned(),
        Arc::new(FontData::from_static(include_bytes!(
            "../../../assets/fonts/Onest-SemiBold.ttf"
        ))),
    );
    fonts
        .families
        .entry(FontFamily::Proportional)
        .or_default()
        .insert(0, FONT_REGULAR.to_owned());
    fonts
        .families
        .entry(FontFamily::Monospace)
        .or_default()
        .insert(0, FONT_REGULAR.to_owned());
    fonts.families.insert(
        FontFamily::Name(SEMIBOLD_FAMILY.into()),
        vec![FONT_SEMIBOLD.to_owned(), FONT_REGULAR.to_owned()],
    );
    ctx.set_fonts(fonts);

    ctx.all_styles_mut(|style| {
        style.text_styles = [
            (
                TextStyle::Heading,
                FontId::new(24.0, FontFamily::Name(SEMIBOLD_FAMILY.into())),
            ),
            (TextStyle::Body, FontId::new(15.0, FontFamily::Proportional)),
            (
                TextStyle::Button,
                FontId::new(15.0, FontFamily::Name(SEMIBOLD_FAMILY.into())),
            ),
            (
                TextStyle::Small,
                FontId::new(12.0, FontFamily::Proportional),
            ),
            (
                TextStyle::Monospace,
                FontId::new(14.0, FontFamily::Proportional),
            ),
        ]
        .into();
        style.spacing.item_spacing = egui::vec2(8.0, 8.0);
        style.spacing.button_padding = egui::vec2(14.0, 8.0);
        style.spacing.interact_size.y = 34.0;
    });
}

pub fn apply_theme(ctx: &egui::Context, theme: ConfigTheme) {
    ctx.set_theme(match theme {
        ConfigTheme::Light => egui::ThemePreference::Light,
        ConfigTheme::Dark => egui::ThemePreference::Dark,
        ConfigTheme::System => egui::ThemePreference::System,
    });
    ctx.set_visuals_of(egui::Theme::Dark, visuals_for(&DARK, true));
    ctx.set_visuals_of(egui::Theme::Light, visuals_for(&LIGHT, false));
}

fn visuals_for(p: &Palette, dark: bool) -> Visuals {
    let mut visuals = if dark {
        Visuals::dark()
    } else {
        Visuals::light()
    };
    visuals.panel_fill = p.bg;
    visuals.window_fill = p.panel;
    visuals.extreme_bg_color = p.bg;
    visuals.faint_bg_color = p.card;
    visuals.code_bg_color = p.card;
    visuals.selection.bg_fill = p.selection.gamma_multiply(0.22);
    visuals.selection.stroke = Stroke::new(1.0, p.selection);
    visuals.hyperlink_color = p.selection;
    visuals.warn_fg_color = p.warning;
    visuals.error_fg_color = p.danger;
    visuals.widgets.noninteractive.bg_fill = p.card;
    visuals.widgets.noninteractive.bg_stroke = Stroke::new(1.0, p.border);
    visuals.widgets.noninteractive.fg_stroke = Stroke::new(1.0, p.muted);
    visuals.widgets.inactive.bg_fill = p.card;
    visuals.widgets.inactive.weak_bg_fill = p.card;
    visuals.widgets.inactive.bg_stroke = Stroke::new(1.0, p.border);
    visuals.widgets.inactive.fg_stroke = Stroke::new(1.0, p.text);
    visuals.widgets.hovered.bg_fill = p.card_hover;
    visuals.widgets.hovered.weak_bg_fill = p.card_hover;
    visuals.widgets.hovered.bg_stroke = Stroke::new(1.0, p.accent.gamma_multiply(0.7));
    visuals.widgets.hovered.fg_stroke = Stroke::new(1.0, p.text);
    visuals.widgets.active.bg_fill = p.accent;
    visuals.widgets.active.weak_bg_fill = p.accent;
    visuals.widgets.active.bg_stroke = Stroke::new(1.0, p.accent_hover);
    visuals.widgets.active.fg_stroke = Stroke::new(1.0, p.on_accent);
    visuals.window_corner_radius = CornerRadius::same(14);
    visuals.menu_corner_radius = CornerRadius::same(10);
    for widget in [
        &mut visuals.widgets.noninteractive,
        &mut visuals.widgets.inactive,
        &mut visuals.widgets.hovered,
        &mut visuals.widgets.active,
        &mut visuals.widgets.open,
    ] {
        widget.corner_radius = CornerRadius::same(8);
    }
    visuals.window_stroke = Stroke::new(1.0, p.border);
    visuals.window_shadow = egui::epaint::Shadow {
        offset: [0, 12],
        blur: 36,
        spread: 0,
        color: Color32::from_black_alpha(if dark { 90 } else { 45 }),
    };
    visuals
}

pub fn card_frame(p: &Palette) -> egui::Frame {
    egui::Frame::new()
        .fill(p.card)
        .stroke(Stroke::new(1.0, p.border))
        .corner_radius(CornerRadius::same(CARD_RADIUS))
        .inner_margin(Margin::same(CARD_PADDING))
}

pub fn card(ui: &mut egui::Ui, p: &Palette, add_contents: impl FnOnce(&mut egui::Ui)) {
    card_frame(p).show(ui, |ui| {
        ui.set_min_width(ui.available_width());
        add_contents(ui);
    });
}

pub fn section_title(ui: &mut egui::Ui, p: &Palette, text: &str) {
    ui.add(egui::Label::new(strong_text(text, FONT_SECTION).color(p.muted)).truncate());
}

pub fn muted(ui: &mut egui::Ui, p: &Palette, text: impl Into<String>) {
    ui.add(egui::Label::new(RichText::new(text.into()).color(p.muted).size(FONT_MUTED)).wrap());
}

pub fn muted_inline(ui: &mut egui::Ui, p: &Palette, text: impl Into<String>) {
    ui.add(egui::Label::new(RichText::new(text.into()).color(p.muted).size(FONT_MUTED)).truncate());
}

pub fn page_title(ui: &mut egui::Ui, p: &Palette, text: &str) {
    ui.add(egui::Label::new(strong_text(text, FONT_PAGE_TITLE).color(p.text)).truncate());
}

pub fn text_input(ui: &mut egui::Ui, buf: &mut String, hint: &str, width: f32) -> egui::Response {
    ui.add_sized(
        [width, INPUT_HEIGHT],
        egui::TextEdit::singleline(buf)
            .hint_text(hint)
            .margin(Margin::symmetric(12, 11))
            .align(egui::Align2::LEFT_CENTER),
    )
}

/// Lays out a text column and a right-aligned action column without overlap.
pub fn row_with_actions(
    ui: &mut egui::Ui,
    actions_width: f32,
    add_text: impl FnOnce(&mut egui::Ui),
    add_actions: impl FnOnce(&mut egui::Ui),
) {
    ui.horizontal(|ui| {
        let text_width = (ui.available_width() - actions_width - 8.0).max(80.0);
        ui.allocate_ui_with_layout(
            Vec2::new(text_width, 0.0),
            Layout::top_down(Align::LEFT),
            add_text,
        );
        ui.with_layout(Layout::right_to_left(Align::Center), add_actions);
    });
}

/// Like [`row_with_actions`], but keeps the text column on one line.
pub fn row_with_actions_inline(
    ui: &mut egui::Ui,
    actions_width: f32,
    add_text: impl FnOnce(&mut egui::Ui),
    add_actions: impl FnOnce(&mut egui::Ui),
) {
    ui.horizontal(|ui| {
        let text_width = (ui.available_width() - actions_width - 8.0).max(80.0);
        ui.allocate_ui_with_layout(
            Vec2::new(text_width, 0.0),
            Layout::left_to_right(Align::Center),
            add_text,
        );
        ui.with_layout(Layout::right_to_left(Align::Center), add_actions);
    });
}

pub fn status_dot(ui: &mut egui::Ui, color: Color32) {
    let size = egui::vec2(8.0, 8.0);
    let (rect, _) = ui.allocate_exact_size(size, egui::Sense::hover());
    ui.painter().circle_filled(rect.center(), 4.0, color);
}

pub fn badge(ui: &mut egui::Ui, _p: &Palette, text: &str, color: Color32) {
    badge_sized(ui, text, color, 260.0);
}

pub fn badge_sized(ui: &mut egui::Ui, text: &str, color: Color32, max_width: f32) {
    let response = egui::Frame::new()
        .fill(color.gamma_multiply(0.15))
        .stroke(Stroke::new(1.0, color.gamma_multiply(0.45)))
        .corner_radius(CornerRadius::same(255))
        .inner_margin(Margin::symmetric(10, 4))
        .show(ui, |ui| {
            ui.set_max_width((max_width - 22.0).max(24.0));
            ui.add(egui::Label::new(strong_text(text, 12.0).color(color)).truncate());
        })
        .response;
    response.on_hover_text(text);
}

fn add_auto_button(ui: &mut egui::Ui, button: egui::Button<'_>) -> egui::Response {
    ui.horizontal(|ui| ui.add(button.truncate())).inner
}

fn add_fixed_button(
    ui: &mut egui::Ui,
    size: egui::Vec2,
    button: egui::Button<'_>,
) -> egui::Response {
    ui.allocate_ui_with_layout(size, Layout::left_to_right(Align::Center), |ui| {
        ui.add(button.min_size(size).truncate())
    })
    .inner
}

pub fn primary_button(ui: &mut egui::Ui, p: &Palette, text: &str) -> egui::Response {
    primary_button_ex(ui, p, text, egui::vec2(0.0, BUTTON_HEIGHT))
}

pub fn primary_button_sized(
    ui: &mut egui::Ui,
    p: &Palette,
    text: &str,
    width: f32,
) -> egui::Response {
    primary_button_ex(ui, p, text, egui::vec2(width, BUTTON_HEIGHT))
}

pub fn primary_button_ex(
    ui: &mut egui::Ui,
    p: &Palette,
    text: &str,
    size: egui::Vec2,
) -> egui::Response {
    let response = if size.x > 0.0 {
        add_fixed_button(
            ui,
            size,
            egui::Button::new((
                Atom::grow(),
                strong_text(text, FONT_LABEL).color(p.on_accent),
                Atom::grow(),
            ))
            .fill(p.accent)
            .stroke(Stroke::NONE)
            .corner_radius(CornerRadius::same(BUTTON_RADIUS)),
        )
    } else {
        add_auto_button(
            ui,
            egui::Button::new(strong_text(text, FONT_LABEL).color(p.on_accent))
                .fill(p.accent)
                .stroke(Stroke::NONE)
                .corner_radius(CornerRadius::same(BUTTON_RADIUS)),
        )
    };
    press_overlay(ui, &response, BUTTON_RADIUS);
    response
}

pub fn primary_button_fill(
    ui: &mut egui::Ui,
    p: &Palette,
    text: &str,
    width: f32,
) -> egui::Response {
    let (rect, response) =
        ui.allocate_exact_size(egui::vec2(width, BUTTON_HEIGHT), egui::Sense::click());
    let fill = if response.is_pointer_button_down_on() || response.hovered() {
        p.accent_hover
    } else {
        p.accent
    };
    let painter = ui.painter();
    painter.rect_filled(rect, CornerRadius::same(BUTTON_RADIUS), fill);
    let galley = egui::WidgetText::from(strong_text(text, FONT_LABEL).color(p.on_accent))
        .into_galley(
            ui,
            Some(egui::TextWrapMode::Truncate),
            (rect.width() - 24.0).max(1.0),
            egui::TextStyle::Button,
        );
    painter.galley(rect.center() - galley.size() / 2.0, galley, p.on_accent);
    if response.has_focus() {
        painter.rect_stroke(
            rect,
            CornerRadius::same(BUTTON_RADIUS),
            Stroke::new(2.0, p.selection),
            egui::StrokeKind::Inside,
        );
    }
    let enabled = ui.is_enabled();
    let label = text.to_owned();
    response.widget_info(|| egui::WidgetInfo::labeled(egui::Role::Button, enabled, &label));
    response
        .on_hover_cursor(egui::CursorIcon::PointingHand)
        .on_hover_text(text)
}

pub fn secondary_button(ui: &mut egui::Ui, p: &Palette, text: &str) -> egui::Response {
    secondary_button_ex(ui, p, text, egui::vec2(0.0, BUTTON_HEIGHT))
}

pub fn secondary_button_sized(
    ui: &mut egui::Ui,
    p: &Palette,
    text: &str,
    width: f32,
) -> egui::Response {
    secondary_button_ex(ui, p, text, egui::vec2(width, BUTTON_HEIGHT))
}

pub fn secondary_button_ex(
    ui: &mut egui::Ui,
    p: &Palette,
    text: &str,
    size: egui::Vec2,
) -> egui::Response {
    let response = if size.x > 0.0 {
        add_fixed_button(
            ui,
            size,
            egui::Button::new((
                Atom::grow(),
                strong_text(text, FONT_LABEL).color(p.text),
                Atom::grow(),
            ))
            .fill(p.card_hover)
            .stroke(Stroke::new(1.0, p.border))
            .corner_radius(CornerRadius::same(BUTTON_RADIUS)),
        )
    } else {
        add_auto_button(
            ui,
            egui::Button::new(strong_text(text, FONT_LABEL).color(p.text))
                .fill(p.card_hover)
                .stroke(Stroke::new(1.0, p.border))
                .corner_radius(CornerRadius::same(BUTTON_RADIUS)),
        )
    };
    press_overlay(ui, &response, BUTTON_RADIUS);
    response
}

pub fn secondary_icon_button(
    ui: &mut egui::Ui,
    p: &Palette,
    icon: egui::ImageSource<'static>,
    text: &str,
) -> egui::Response {
    let image = egui::Image::new(icon)
        .fit_to_exact_size(egui::vec2(15.0, 15.0))
        .tint(p.muted);
    let response = add_auto_button(
        ui,
        egui::Button::new((image, strong_text(text, 13.5).color(p.text)))
            .image_tint_follows_text_color(true)
            .fill(p.card_hover)
            .stroke(Stroke::new(1.0, p.border))
            .corner_radius(CornerRadius::same(BUTTON_RADIUS)),
    );
    press_overlay(ui, &response, BUTTON_RADIUS);
    response
}

pub fn icon_button(
    ui: &mut egui::Ui,
    p: &Palette,
    icon: egui::ImageSource<'static>,
    size: f32,
    tint: Color32,
    tooltip: &str,
) -> egui::Response {
    let image = egui::Image::new(icon)
        .fit_to_exact_size(egui::vec2(18.0, 18.0))
        .tint(tint);
    let response = ui
        .scope(|ui| {
            ui.spacing_mut().button_padding = egui::vec2(12.0, 12.0);
            add_fixed_button(
                ui,
                egui::vec2(size, size),
                egui::Button::new((Atom::grow(), image, Atom::grow()))
                    .fill(p.card_hover)
                    .stroke(Stroke::new(1.0, p.border))
                    .corner_radius(CornerRadius::same(BUTTON_RADIUS)),
            )
        })
        .inner;
    press_overlay(ui, &response, BUTTON_RADIUS);
    response.on_hover_text(tooltip)
}

pub fn text_width(ui: &egui::Ui, text: &str, font_id: FontId) -> f32 {
    ui.ctx().fonts_mut(|fonts| {
        fonts
            .layout_no_wrap(text.to_owned(), font_id, Color32::WHITE)
            .size()
            .x
    })
}

fn text_chip_ex(
    ui: &mut egui::Ui,
    p: &Palette,
    text: &str,
    font_size: f32,
    max_width: f32,
    semibold_text: bool,
    center: bool,
) -> egui::Response {
    let font_id = if semibold_text {
        semibold(font_size)
    } else {
        FontId::proportional(font_size)
    };
    let text_width = text_width(ui, text, font_id);
    let width = (text_width + 24.0).clamp(36.0, max_width.max(36.0));
    let frame = egui::Frame::new()
        .fill(p.card_hover)
        .stroke(Stroke::new(1.0, p.border))
        .corner_radius(CornerRadius::same(BUTTON_RADIUS))
        .inner_margin(Margin::symmetric(10, 8))
        .show(ui, |ui| {
            ui.set_width((width - 22.0).max(16.0));
            let label = if semibold_text {
                strong_text(text, font_size)
            } else {
                RichText::new(text).size(font_size)
            };
            ui.add(
                egui::Label::new(label.color(p.text))
                    .halign(if center {
                        egui::Align::Center
                    } else {
                        egui::Align::LEFT
                    })
                    .truncate(),
            );
        });
    frame.response.on_hover_text(text)
}

pub fn text_chip(
    ui: &mut egui::Ui,
    p: &Palette,
    text: &str,
    font_size: f32,
    max_width: f32,
    semibold_text: bool,
) -> egui::Response {
    text_chip_ex(ui, p, text, font_size, max_width, semibold_text, false)
}

pub fn value_box(ui: &mut egui::Ui, p: &Palette, value: impl Into<String>) {
    let value = value.into();
    text_chip_ex(ui, p, &value, 13.0, 120.0, true, true);
}

pub fn segmented_button(
    ui: &mut egui::Ui,
    p: &Palette,
    label: &str,
    selected: bool,
    width: f32,
) -> egui::Response {
    let (fill, stroke, color) = if selected {
        (
            p.selection.gamma_multiply(0.16),
            Stroke::new(1.0, p.selection),
            p.selection,
        )
    } else {
        (p.card_hover, Stroke::new(1.0, p.border), p.text)
    };
    let response = add_fixed_button(
        ui,
        egui::vec2(width, CONTROL_HEIGHT),
        egui::Button::new((
            Atom::grow(),
            strong_text(label, 13.0).color(color),
            Atom::grow(),
        ))
        .fill(fill)
        .stroke(stroke)
        .corner_radius(CornerRadius::same(BUTTON_RADIUS)),
    );
    press_overlay(ui, &response, BUTTON_RADIUS);
    response.on_hover_text(label)
}

pub fn slider(
    ui: &mut egui::Ui,
    p: &Palette,
    value: &mut usize,
    range: std::ops::RangeInclusive<usize>,
    width: f32,
) -> egui::Response {
    let width = if width.is_finite() && width > 0.0 {
        width
    } else {
        ui.available_width()
    }
    .max(1.0);
    let (rect, mut response) =
        ui.allocate_exact_size(egui::vec2(width, 34.0), egui::Sense::click_and_drag());
    let start = *range.start();
    let end = *range.end();
    if end > start {
        let mut changed = false;
        if (response.clicked() || response.dragged())
            && let Some(pointer) = ui.input(|input| input.pointer.interact_pos())
        {
            let track = rect.shrink2(egui::vec2(9.0, 0.0));
            let t = if track.width() > 0.0 {
                ((pointer.x - track.left()) / track.width()).clamp(0.0, 1.0)
            } else {
                0.0
            };
            let next = start + ((end - start) as f32 * t).round() as usize;
            if next != *value {
                *value = next;
                changed = true;
            }
        }
        if response.has_focus() {
            let delta = ui.input(|input| {
                i32::from(input.key_pressed(egui::Key::ArrowRight))
                    - i32::from(input.key_pressed(egui::Key::ArrowLeft))
            });
            if delta != 0 {
                let next = value
                    .saturating_add_signed(delta as isize)
                    .clamp(start, end);
                if next != *value {
                    *value = next;
                    changed = true;
                }
            }
        }
        if changed {
            response.mark_changed();
        }
    }

    let t = if end > start {
        (*value).clamp(start, end).saturating_sub(start) as f32 / (end - start) as f32
    } else {
        0.0
    };
    let center_y = rect.center().y;
    let track = egui::Rect::from_min_max(
        egui::pos2(rect.left() + 9.0, center_y - 3.0),
        egui::pos2(rect.right() - 9.0, center_y + 3.0),
    );
    let painter = ui.painter();
    painter.rect_filled(track, CornerRadius::same(3), p.border);
    let knob_x = track.left() + track.width() * t;
    if t > 0.0 {
        painter.rect_filled(
            egui::Rect::from_min_max(track.min, egui::pos2(knob_x, track.bottom())),
            CornerRadius::same(3),
            p.text,
        );
    }
    let ring = if response.hovered() || response.dragged() {
        p.accent
    } else {
        p.text
    };
    painter.circle_filled(egui::pos2(knob_x, center_y), 8.0, p.card);
    painter.circle_stroke(egui::pos2(knob_x, center_y), 8.0, Stroke::new(3.0, ring));
    response.on_hover_cursor(egui::CursorIcon::ResizeHorizontal)
}

pub fn progress_bar(ui: &mut egui::Ui, p: &Palette, fraction: f32, height: f32) -> egui::Response {
    let desired = egui::vec2(ui.available_width(), height);
    let (rect, response) = ui.allocate_exact_size(desired, egui::Sense::hover());
    let painter = ui.painter();
    let radius = CornerRadius::same((height / 2.0).round().max(0.0) as u8);
    painter.rect_filled(rect, radius, p.track);
    let fraction = if fraction.is_finite() {
        fraction.clamp(0.0, 1.0)
    } else {
        0.0
    };
    if fraction > 0.0 {
        let width = rect.width() * fraction;
        let fill = egui::Rect::from_min_size(rect.min, egui::vec2(width, rect.height()));
        painter.rect_filled(fill, radius, p.accent);
    }
    response
}

pub fn modal_frame(p: &Palette) -> egui::Frame {
    egui::Frame::new()
        .fill(p.card)
        .stroke(Stroke::new(1.0, p.border))
        .corner_radius(CornerRadius::same(20))
        .inner_margin(Margin::same(24))
        .shadow(egui::epaint::Shadow {
            offset: [0, 18],
            blur: 44,
            spread: 0,
            color: Color32::from_black_alpha(120),
        })
}

pub fn modal_icon(ui: &mut egui::Ui, p: &Palette, icon: egui::ImageSource<'static>) {
    egui::Frame::new()
        .fill(p.selection.gamma_multiply(0.15))
        .stroke(Stroke::new(1.0, p.selection.gamma_multiply(0.45)))
        .corner_radius(CornerRadius::same(12))
        .inner_margin(Margin::same(12))
        .show(ui, |ui| {
            ui.add(
                egui::Image::new(icon)
                    .fit_to_exact_size(egui::vec2(22.0, 22.0))
                    .tint(p.selection),
            );
        });
}

pub fn modal_header(
    ui: &mut egui::Ui,
    p: &Palette,
    icon: egui::ImageSource<'static>,
    title: &str,
    subtitle: &str,
) {
    ui.horizontal(|ui| {
        modal_icon(ui, p, icon);
        ui.add_space(4.0);
        ui.vertical(|ui| {
            ui.add(egui::Label::new(strong_text(title, 22.0).color(p.text)).truncate());
            if !subtitle.trim().is_empty() {
                ui.add(
                    egui::Label::new(RichText::new(subtitle).size(FONT_MUTED).color(p.muted))
                        .wrap(),
                );
            }
        });
    });
}

pub fn modal_close(ui: &mut egui::Ui, p: &Palette, text: &str) -> bool {
    let mut clicked = false;
    ui.vertical_centered(|ui| {
        let response = add_auto_button(
            ui,
            egui::Button::new(strong_text(text, 13.0).color(p.muted))
                .fill(Color32::TRANSPARENT)
                .stroke(Stroke::NONE),
        );
        clicked = response.clicked();
    });
    clicked
}

pub fn press_overlay(ui: &mut egui::Ui, response: &egui::Response, radius: u8) {
    if response.is_pointer_button_down_on() {
        ui.painter().rect_filled(
            response.rect.shrink(1.0),
            CornerRadius::same(radius),
            Color32::from_black_alpha(42),
        );
    }
}

pub fn spinner(ui: &mut egui::Ui, p: &Palette, text: &str) {
    ui.horizontal(|ui| {
        ui.add(egui::Spinner::new().color(p.accent));
        ui.add(egui::Label::new(RichText::new(text).color(p.muted)).truncate());
    });
}

pub fn centered_spinner(ui: &mut egui::Ui, p: &Palette, text: &str) {
    let text_width = text_width(ui, text, FontId::proportional(FONT_BODY));
    let group_width = (18.0 + 8.0 + text_width).min(ui.available_width().max(1.0));
    ui.allocate_ui_with_layout(
        egui::vec2(group_width, CONTROL_HEIGHT),
        egui::Layout::left_to_right(egui::Align::Center),
        |ui| spinner(ui, p, text),
    );
}

pub fn dashed_placeholder(ui: &mut egui::Ui, p: &Palette, height: f32, text: &str) {
    let width = ui.available_width();
    let (rect, _) = ui.allocate_exact_size(egui::vec2(width, height), egui::Sense::hover());
    let painter = ui.painter();
    let color = p.border;
    let dash = 6.0;
    let gap = 5.0;
    let draw_edge = |from: egui::Pos2, to: egui::Pos2| {
        let delta = to - from;
        let length = delta.length();
        if length <= 0.0 {
            return;
        }
        let dir = delta / length;
        let mut offset = 0.0;
        while offset < length {
            let end = (offset + dash).min(length);
            painter.line_segment(
                [from + dir * offset, from + dir * end],
                Stroke::new(1.0, color),
            );
            offset = end + gap;
        }
    };
    draw_edge(rect.left_top(), rect.right_top());
    draw_edge(rect.right_top(), rect.right_bottom());
    draw_edge(rect.right_bottom(), rect.left_bottom());
    draw_edge(rect.left_bottom(), rect.left_top());
    let galley = painter.layout(
        text.to_owned(),
        egui::FontId::proportional(FONT_MUTED),
        p.muted,
        (rect.width() - 24.0).max(1.0),
    );
    painter.galley(rect.center() - galley.size() / 2.0, galley, p.muted);
}

pub fn split_columns(
    available: f32,
    gap: f32,
    left_ratio: f32,
    left_min: f32,
    right_min: f32,
) -> Option<(f32, f32)> {
    if available < left_min + right_min + gap {
        return None;
    }
    let left = ((available - gap) * left_ratio).max(left_min);
    let right = available - gap - left;
    (right >= right_min).then_some((left, right))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn dark_palette_matches_design_spec() {
        assert_eq!(DARK.bg, Color32::from_rgb(0x0F, 0x11, 0x15));
        assert_eq!(DARK.panel, Color32::from_rgb(0x0B, 0x0D, 0x10));
        assert_eq!(DARK.card, Color32::from_rgb(0x17, 0x1A, 0x21));
        assert_eq!(DARK.card_hover, Color32::from_rgb(0x1F, 0x23, 0x2C));
        assert_eq!(DARK.border, Color32::from_rgb(0x2A, 0x2F, 0x3A));
        assert_eq!(DARK.text, Color32::from_rgb(0xEC, 0xEE, 0xF2));
        assert_eq!(DARK.muted, Color32::from_rgb(0xA3, 0xAA, 0xB8));
        assert_eq!(DARK.accent, Color32::from_rgb(0xFF, 0x5A, 0x4F));
        assert_eq!(DARK.success, Color32::from_rgb(0x4A, 0xDE, 0x9A));
        assert_eq!(DARK.selection, Color32::from_rgb(0x5C, 0xC8, 0xE8));
    }

    #[test]
    fn home_columns_match_screenshot_ratio() {
        let (left, right) = split_columns(967.0, 16.0, 0.42, 340.0, 360.0).unwrap();
        assert!((left - 399.4).abs() < 0.5);
        assert!((right - 551.6).abs() < 0.5);
    }

    #[test]
    fn narrow_layout_stacks() {
        assert!(split_columns(628.0, 16.0, 0.42, 340.0, 360.0).is_none());
    }
}
