use std::sync::Arc;

use eframe::CreationContext;
use egui::{
    Color32, CornerRadius, FontData, FontDefinitions, FontFamily, FontId, Margin, RichText, Stroke,
    TextStyle, Visuals,
};
use volvid_engine::Theme;

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
    selection: Color32::from_rgb(0, 139, 177),
    success: Color32::from_rgb(20, 156, 94),
    warning: Color32::from_rgb(184, 116, 9),
    danger: Color32::from_rgb(209, 55, 65),
    track: Color32::from_rgb(238, 241, 245),
};

pub fn palette(theme: Theme) -> &'static Palette {
    match theme {
        Theme::Light => &LIGHT,
        _ => &DARK,
    }
}

pub fn setup(cc: &CreationContext<'_>, theme: Theme) {
    egui_extras::install_image_loaders(&cc.egui_ctx);
    install_fonts(&cc.egui_ctx);
    apply_theme(&cc.egui_ctx, theme);
}

pub fn install_fonts(ctx: &egui::Context) {
    let mut fonts = FontDefinitions::default();
    fonts.font_data.insert(
        "noto-sans".to_owned(),
        Arc::new(FontData::from_static(include_bytes!(
            "../../../assets/fonts/NotoSans-Regular.ttf"
        ))),
    );
    fonts.font_data.insert(
        "noto-sans-semibold".to_owned(),
        Arc::new(FontData::from_static(include_bytes!(
            "../../../assets/fonts/NotoSans-SemiBold.ttf"
        ))),
    );
    fonts
        .families
        .entry(FontFamily::Proportional)
        .or_default()
        .insert(0, "noto-sans".to_owned());
    fonts
        .families
        .entry(FontFamily::Monospace)
        .or_default()
        .insert(0, "noto-sans".to_owned());
    ctx.set_fonts(fonts);

    ctx.all_styles_mut(|style| {
        style.text_styles = [
            (
                TextStyle::Heading,
                FontId::new(24.0, FontFamily::Proportional),
            ),
            (TextStyle::Body, FontId::new(15.0, FontFamily::Proportional)),
            (
                TextStyle::Button,
                FontId::new(15.0, FontFamily::Proportional),
            ),
            (
                TextStyle::Small,
                FontId::new(12.0, FontFamily::Proportional),
            ),
            (
                TextStyle::Monospace,
                FontId::new(14.0, FontFamily::Monospace),
            ),
        ]
        .into();
        style.spacing.item_spacing = egui::vec2(8.0, 8.0);
        style.spacing.button_padding = egui::vec2(14.0, 8.0);
        style.spacing.interact_size.y = 34.0;
    });
}

pub fn apply_theme(ctx: &egui::Context, theme: Theme) {
    let p = palette(theme);
    let mut visuals = if matches!(theme, Theme::Light) {
        Visuals::light()
    } else {
        Visuals::dark()
    };
    visuals.panel_fill = p.bg;
    visuals.window_fill = p.panel;
    visuals.extreme_bg_color = p.bg;
    visuals.faint_bg_color = p.card;
    visuals.code_bg_color = p.card;
    visuals.selection.bg_fill = p.selection.gamma_multiply(0.22);
    visuals.selection.stroke = Stroke::new(1.0, p.selection);
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
    visuals.widgets.active.fg_stroke = Stroke::new(1.0, Color32::WHITE);
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
        color: Color32::from_black_alpha(90),
    };
    ctx.set_visuals(visuals);
}

pub fn card_frame(p: &Palette) -> egui::Frame {
    egui::Frame::new()
        .fill(p.card)
        .stroke(Stroke::new(1.0, p.border))
        .corner_radius(CornerRadius::same(14))
        .inner_margin(Margin::same(18))
}

pub fn section_title(ui: &mut egui::Ui, p: &Palette, text: &str) {
    ui.label(egui::RichText::new(text).size(13.0).strong().color(p.muted));
}

pub fn muted(ui: &mut egui::Ui, p: &Palette, text: impl Into<String>) {
    ui.label(egui::RichText::new(text.into()).color(p.muted).size(13.0));
}

pub fn status_dot(ui: &mut egui::Ui, color: Color32) {
    let size = egui::vec2(8.0, 8.0);
    let (rect, _) = ui.allocate_exact_size(size, egui::Sense::hover());
    ui.painter().circle_filled(rect.center(), 4.0, color);
}

pub fn badge(ui: &mut egui::Ui, _p: &Palette, text: &str, color: Color32) {
    egui::Frame::new()
        .fill(color.gamma_multiply(0.15))
        .stroke(Stroke::new(1.0, color.gamma_multiply(0.45)))
        .corner_radius(CornerRadius::same(255))
        .inner_margin(Margin::symmetric(10, 4))
        .show(ui, |ui| {
            ui.label(egui::RichText::new(text).color(color).size(12.0).strong());
        });
}

pub fn primary_button(ui: &mut egui::Ui, p: &Palette, text: &str) -> egui::Response {
    let button = egui::Button::new(
        egui::RichText::new(text)
            .color(Color32::from_rgb(15, 17, 21))
            .strong()
            .size(14.5),
    )
    .fill(p.accent)
    .stroke(Stroke::NONE)
    .corner_radius(CornerRadius::same(10))
    .min_size(egui::vec2(128.0, 40.0));
    let response = ui.add(button);
    press_overlay(ui, &response, 10);
    response
}

pub fn secondary_button(ui: &mut egui::Ui, p: &Palette, text: &str) -> egui::Response {
    let response = ui.add(
        egui::Button::new(egui::RichText::new(text).color(p.text).size(14.0))
            .fill(p.card_hover)
            .stroke(Stroke::new(1.0, p.border))
            .corner_radius(CornerRadius::same(10))
            .min_size(egui::vec2(90.0, 38.0)),
    );
    press_overlay(ui, &response, 10);
    response
}

pub fn progress_bar(ui: &mut egui::Ui, p: &Palette, fraction: f32, height: f32) -> egui::Response {
    let desired = egui::vec2(ui.available_width(), height);
    let (rect, response) = ui.allocate_exact_size(desired, egui::Sense::hover());
    let painter = ui.painter();
    painter.rect_filled(rect, CornerRadius::same((height / 2.0) as u8), p.track);
    let fraction = fraction.clamp(0.0, 1.0);
    if fraction > 0.0 {
        let width = rect.width() * fraction;
        let fill = egui::Rect::from_min_size(rect.min, egui::vec2(width, rect.height()));
        painter.rect_filled(fill, CornerRadius::same((height / 2.0) as u8), p.accent);
    }
    response
}

pub fn primary_button_sized(
    ui: &mut egui::Ui,
    p: &Palette,
    text: &str,
    width: f32,
) -> egui::Response {
    let response = ui.add(
        egui::Button::new(
            egui::RichText::new(text)
                .color(Color32::from_rgb(15, 17, 21))
                .strong()
                .size(14.5),
        )
        .fill(p.accent)
        .stroke(Stroke::NONE)
        .corner_radius(CornerRadius::same(10))
        .min_size(egui::vec2(width, 42.0)),
    );
    press_overlay(ui, &response, 10);
    response
}

pub fn secondary_button_sized(
    ui: &mut egui::Ui,
    p: &Palette,
    text: &str,
    width: f32,
) -> egui::Response {
    let response = ui.add(
        egui::Button::new(egui::RichText::new(text).color(p.text).size(14.0))
            .fill(p.card_hover)
            .stroke(Stroke::new(1.0, p.border))
            .corner_radius(CornerRadius::same(10))
            .min_size(egui::vec2(width, 42.0)),
    );
    press_overlay(ui, &response, 10);
    response
}

pub fn modal_backdrop(ctx: &egui::Context) {
    egui::Area::new(egui::Id::unique("volvid-modal-backdrop"))
        .fixed_pos(egui::Pos2::ZERO)
        .order(egui::Order::Middle)
        .show(ctx, |ui| {
            let screen = ui.ctx().content_rect();
            ui.painter()
                .rect_filled(screen, 0, Color32::from_black_alpha(150));
            ui.allocate_rect(screen, egui::Sense::click());
        });
}

pub fn modal_frame(p: &Palette) -> egui::Frame {
    egui::Frame::new()
        .fill(p.card)
        .stroke(Stroke::new(1.0, p.border))
        .corner_radius(CornerRadius::same(18))
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
            ui.label(RichText::new(title).size(21.0).strong().color(p.text));
            if !subtitle.trim().is_empty() {
                ui.label(RichText::new(subtitle).size(13.0).color(p.muted));
            }
        });
    });
}

pub fn modal_close(ui: &mut egui::Ui, p: &Palette, text: &str) -> bool {
    let mut clicked = false;
    ui.vertical_centered(|ui| {
        let response = ui.add(
            egui::Button::new(RichText::new(text).size(13.0).color(p.muted))
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
        ui.label(egui::RichText::new(text).color(p.muted));
    });
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
    painter.text(
        rect.center(),
        egui::Align2::CENTER_CENTER,
        text,
        egui::FontId::proportional(13.0),
        p.muted,
    );
}
