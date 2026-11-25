//! Paint: turn a [`Layout`] into an RGBA framebuffer.
//!
//! Rendering is pure software: rounded rectangles, hairline rules, and glyphs
//! from the baked bitmap font in [`crate::font`]. The framebuffer is plain
//! premultiplied-less RGBA8, which is exactly what `PutImage` wants on the wire,
//! so no image library sits between this crate and the window.

#![forbid(unsafe_code)]

use crate::font;
use crate::layout::{Layout, Region, RegionKind};
use crate::view::{Align, Rect, Rgba};

/// A software framebuffer.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Canvas {
    width: u16,
    height: u16,
    pixels: Vec<u8>,
}

impl Canvas {
    /// Allocates a canvas of the given size, filled with opaque black.
    #[must_use]
    pub fn new(width: u16, height: u16) -> Self {
        let len = usize::from(width) * usize::from(height) * 4;
        let mut pixels = vec![0u8; len];
        for chunk in pixels.chunks_exact_mut(4) {
            chunk[3] = 255;
        }
        Self {
            width,
            height,
            pixels,
        }
    }

    /// Canvas width in pixels.
    #[must_use]
    pub const fn width(&self) -> u16 {
        self.width
    }

    /// Canvas height in pixels.
    #[must_use]
    pub const fn height(&self) -> u16 {
        self.height
    }

    /// Raw RGBA bytes, row-major, four bytes per pixel.
    #[must_use]
    pub fn pixels(&self) -> &[u8] {
        &self.pixels
    }

    /// Reads one pixel as RGBA, or transparent black outside the canvas.
    #[must_use]
    pub fn pixel(&self, x: u16, y: u16) -> Rgba {
        if x >= self.width || y >= self.height {
            return Rgba::rgba(0, 0, 0, 0);
        }
        let index = (usize::from(y) * usize::from(self.width) + usize::from(x)) * 4;
        Rgba::rgba(
            self.pixels[index],
            self.pixels[index + 1],
            self.pixels[index + 2],
            self.pixels[index + 3],
        )
    }

    /// Blends `color` over one pixel, ignoring out-of-bounds writes.
    pub fn blend(&mut self, x: i32, y: i32, color: Rgba) {
        let (Ok(x), Ok(y)) = (u16::try_from(x), u16::try_from(y)) else {
            return;
        };
        if x >= self.width || y >= self.height || color.a == 0 {
            return;
        }
        let index = (usize::from(y) * usize::from(self.width) + usize::from(x)) * 4;
        if color.a == 255 {
            self.pixels[index] = color.r;
            self.pixels[index + 1] = color.g;
            self.pixels[index + 2] = color.b;
            self.pixels[index + 3] = 255;
            return;
        }
        let src = u32::from(color.a);
        let dst = 255 - src;
        let mix = |s: u8, d: u8| -> u8 {
            u8::try_from((u32::from(s) * src + u32::from(d) * dst) / 255).unwrap_or(255)
        };
        self.pixels[index] = mix(color.r, self.pixels[index]);
        self.pixels[index + 1] = mix(color.g, self.pixels[index + 1]);
        self.pixels[index + 2] = mix(color.b, self.pixels[index + 2]);
        self.pixels[index + 3] = 255;
    }

    /// Fills a rectangle.
    pub fn fill_rect(&mut self, rect: Rect, color: Rgba) {
        let x0 = i32::from(rect.x).max(0);
        let y0 = i32::from(rect.y).max(0);
        let x1 = i32::from(rect.x).saturating_add(i32::from(rect.w)).min(i32::from(self.width));
        let y1 = i32::from(rect.y).saturating_add(i32::from(rect.h)).min(i32::from(self.height));
        for y in y0..y1 {
            for x in x0..x1 {
                self.blend(x, y, color);
            }
        }
    }

    /// Fills a rectangle with rounded corners of the given radius.
    pub fn fill_round_rect(&mut self, rect: Rect, radius: u16, color: Rgba) {
        let radius = i32::from(radius)
            .min(i32::from(rect.w) / 2)
            .min(i32::from(rect.h) / 2);
        if radius <= 0 {
            self.fill_rect(rect, color);
            return;
        }
        let x0 = i32::from(rect.x);
        let y0 = i32::from(rect.y);
        let x1 = x0.saturating_add(i32::from(rect.w));
        let y1 = y0.saturating_add(i32::from(rect.h));
        let r2 = radius * radius;
        for y in y0.max(0)..y1.min(i32::from(self.height)) {
            for x in x0.max(0)..x1.min(i32::from(self.width)) {
                // Distance from the nearest corner circle centre.
                let cx = if x < x0 + radius {
                    x0 + radius
                } else if x >= x1 - radius {
                    x1 - radius - 1
                } else {
                    x
                };
                let cy = if y < y0 + radius {
                    y0 + radius
                } else if y >= y1 - radius {
                    y1 - radius - 1
                } else {
                    y
                };
                let dx = x - cx;
                let dy = y - cy;
                if dx * dx + dy * dy <= r2 || (x >= x0 + radius && x < x1 - radius) || (y >= y0 + radius && y < y1 - radius) {
                    self.blend(x, y, color);
                }
            }
        }
    }

    /// Outlines a rectangle one pixel wide.
    pub fn stroke_rect(&mut self, rect: Rect, color: Rgba) {
        if rect.is_empty() {
            return;
        }
        let x0 = i32::from(rect.x);
        let y0 = i32::from(rect.y);
        let x1 = x0.saturating_add(i32::from(rect.w)).saturating_sub(1);
        let y1 = y0.saturating_add(i32::from(rect.h)).saturating_sub(1);
        for x in x0..=x1 {
            self.blend(x, y0, color);
            self.blend(x, y1, color);
        }
        for y in y0..=y1 {
            self.blend(x0, y, color);
            self.blend(x1, y, color);
        }
    }

    /// Draws one glyph at `(x, y)` — the top-left of the glyph cell.
    ///
    /// Returns the advance in pixels so callers can lay out runs of text.
    pub fn draw_glyph(&mut self, x: i32, y: i32, ch: char, color: Rgba) -> i32 {
        let advance = i32::from(font::ADVANCE);
        let Some(cell) = font::coverage(ch) else {
            // No glyph: draw a hollow box so missing characters are obvious.
            let box_color = color.with_alpha(120);
            self.stroke_rect(
                Rect::new(
                    i16::try_from(x).unwrap_or(0),
                    i16::try_from(y).unwrap_or(0),
                    font::CELL_WIDTH,
                    font::CELL_HEIGHT,
                ),
                box_color,
            );
            return advance;
        };
        for row in 0..usize::from(font::CELL_HEIGHT) {
            for column in 0..usize::from(font::CELL_WIDTH) {
                let coverage = cell[row * usize::from(font::CELL_WIDTH) + column];
                if coverage == 0 {
                    continue;
                }
                let alpha = u8::try_from(u32::from(coverage) * u32::from(color.a) / 255).unwrap_or(0);
                self.blend(
                    x + i32::try_from(column).unwrap_or(0),
                    y + i32::try_from(row).unwrap_or(0),
                    Rgba::rgba(color.r, color.g, color.b, alpha),
                );
            }
        }
        advance
    }

    /// Draws a string, clipped to `clip` when one is given.
    pub fn draw_text(&mut self, x: i32, y: i32, text: &str, color: Rgba) {
        let mut cursor = x;
        for ch in text.chars() {
            let advance = self.draw_glyph(cursor, y, ch, color);
            cursor = cursor.saturating_add(advance);
        }
    }

    /// Width in pixels of a string in the baked font.
    #[must_use]
    pub fn text_width(text: &str) -> i32 {
        let count = i32::try_from(text.chars().count()).unwrap_or(i32::MAX);
        count.saturating_mul(i32::from(font::ADVANCE))
    }

    /// Encodes the canvas as a binary PPM (P6) for inspection outside the GUI.
    #[must_use]
    pub fn to_ppm(&self) -> Vec<u8> {
        let mut out = format!("P6\n{} {}\n255\n", self.width, self.height).into_bytes();
        out.reserve(self.pixels.len() / 4 * 3);
        for pixel in self.pixels.chunks_exact(4) {
            out.push(pixel[0]);
            out.push(pixel[1]);
            out.push(pixel[2]);
        }
        out
    }

    /// Encodes the canvas as an ASCII-art preview: one character per 8x16 block,
    /// picking the block's average brightness.
    #[must_use]
    pub fn to_ascii(&self) -> String {
        const RAMP: &[u8] = b" .:-=+*#%@";
        let mut out = String::new();
        let mut y = 0u16;
        while y < self.height {
            let mut x = 0u16;
            while x < self.width {
                let mut sum = 0u32;
                let mut count = 0u32;
                for py in y..(y + 16).min(self.height) {
                    for px in x..(x + 8).min(self.width) {
                        let p = self.pixel(px, py);
                        sum += u32::from(p.r) + u32::from(p.g) + u32::from(p.b);
                        count += 1;
                    }
                }
                let brightness = if count == 0 { 0 } else { sum / (count * 3) };
                let index = usize::try_from(u32::from(brightness) * (RAMP.len() as u32 - 1) / 255)
                    .unwrap_or(0)
                    .min(RAMP.len() - 1);
                out.push(char::from(RAMP[index]));
                x = x.saturating_add(8);
            }
            out.push('\n');
            y = y.saturating_add(16);
        }
        out
    }
}

/// Draws a two-pixel-wide check mark centred in `rect`.
///
/// Hand-drawn rather than taken from the font so the tick stays crisp at small
/// sizes and scales with the box instead of the 13-pixel text cell.
fn draw_tick(canvas: &mut Canvas, rect: Rect, color: Rgba) {
    let w = i32::from(rect.w);
    let h = i32::from(rect.h);
    if w < 8 || h < 8 {
        return;
    }
    let x0 = i32::from(rect.x);
    let y0 = i32::from(rect.y);
    // Three points of the polyline, as fractions of the box.
    let a = (x0 + w * 25 / 100, y0 + h * 52 / 100);
    let b = (x0 + w * 43 / 100, y0 + h * 70 / 100);
    let c = (x0 + w * 75 / 100, y0 + h * 32 / 100);
    let mut line = |from: (i32, i32), to: (i32, i32)| {
        let steps = (to.0 - from.0).abs().max((to.1 - from.1).abs()).max(1);
        for step in 0..=steps {
            let x = from.0 + (to.0 - from.0) * step / steps;
            let y = from.1 + (to.1 - from.1) * step / steps;
            canvas.blend(x, y, color);
            canvas.blend(x + 1, y, color);
            canvas.blend(x, y + 1, color);
            canvas.blend(x + 1, y + 1, color);
        }
    };
    line(a, b);
    line(b, c);
}

/// Paints a whole layout into a fresh canvas of the given size.
#[must_use]
pub fn paint(layout: &Layout, width: u16, height: u16) -> Canvas {
    let mut canvas = Canvas::new(width, height);
    for region in &layout.regions {
        paint_region(&mut canvas, region);
    }
    canvas
}

/// Paints one region.
fn paint_region(canvas: &mut Canvas, region: &Region) {
    let rect = region.rect;
    if rect.is_empty() {
        return;
    }
    match region.kind {
        RegionKind::Panel => {
            if region.radius == 0 {
                canvas.fill_rect(rect, region.color);
            } else {
                canvas.fill_round_rect(rect, region.radius, region.color);
            }
        }
        RegionKind::Frame => {
            canvas.fill_round_rect(rect, region.radius, region.color);
            canvas.stroke_rect(rect, region.color);
        }
        RegionKind::Separator => {
            canvas.fill_rect(rect, region.color);
        }
        RegionKind::CheckboxBox => {
            let checked = !region.text.is_empty();
            if checked {
                canvas.fill_round_rect(rect, region.radius, region.color);
            } else {
                canvas.stroke_rect(rect, region.color);
            }
            if checked {
                draw_tick(canvas, rect, Rgba::rgb(12, 14, 28));
            }
        }
        RegionKind::RadioDot => {
            let radius = i32::from(region.radius).min(i32::from(rect.w) / 2);
            let cx = i32::from(rect.x) + i32::from(rect.w) / 2;
            let cy = i32::from(rect.y) + i32::from(rect.h) / 2;
            let filled = !region.text.is_empty();
            for y in 0..=radius {
                for x in -radius..=radius {
                    if x * x + y * y > radius * radius {
                        continue;
                    }
                    let ring = x * x + y * y > (radius - 3) * (radius - 3);
                    if filled || ring {
                        canvas.blend(cx + x, cy + y, region.color);
                        canvas.blend(cx + x, cy - y, region.color);
                    }
                }
            }
        }
        RegionKind::Text => {
            let width = Canvas::text_width(&region.text);
            let start = match region.align {
                Align::Start => i32::from(rect.x),
                Align::Center => {
                    i32::from(rect.x) + (i32::from(rect.w) - width) / 2
                }
                Align::End => i32::from(rect.x) + i32::from(rect.w) - width,
            };
            let y = i32::from(rect.y) + (i32::from(rect.h) - i32::from(font::CELL_HEIGHT)) / 2;
            canvas.draw_text(start.max(i32::from(rect.x)), y, &region.text, region.color);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::{paint, Canvas};
    use crate::layout::layout;
    use crate::view::{Node, Rect, Rgba, View};

    fn canvas_for(view: &View, w: u16, h: u16) -> Canvas {
        paint(&layout(view, Rect::new(0, 0, w, h)), w, h)
    }

    #[test]
    fn canvas_has_requested_size_and_is_opaque() {
        let c = Canvas::new(10, 4);
        assert_eq!((c.width(), c.height()), (10, 4));
        assert_eq!(c.pixels().len(), 10 * 4 * 4);
        assert_eq!(c.pixel(9, 3).a, 255);
    }

    #[test]
    fn background_is_theme_colour() {
        let view = View::new(Node::column(vec![]));
        let c = canvas_for(&view, 120, 80);
        let expected = view.theme.theme().background;
        assert_eq!(c.pixel(0, 0), expected);
        assert_eq!(c.pixel(119, 79), expected);
    }

    #[test]
    fn text_actually_darkens_pixels() {
        let view = View::new(Node::column(vec![Node::label("HHHH")]));
        let c = canvas_for(&view, 240, 120);
        let background = view.theme.theme().background;
        let changed = (0..c.height())
            .flat_map(|y| (0..c.width()).map(move |x| (x, y)))
            .filter(|&(x, y)| c.pixel(x, y) != background)
            .count();
        assert!(changed > 100, "expected glyph pixels, got {changed}");
    }

    #[test]
    fn blend_alpha_mixes_colours() {
        let mut c = Canvas::new(2, 2);
        c.fill_rect(Rect::new(0, 0, 1, 1), Rgba::rgba(0, 0, 0, 255));
        c.blend(0, 0, Rgba::rgba(255, 255, 255, 128));
        let p = c.pixel(0, 0);
        assert!((120..=136).contains(&p.r), "got {p:?}");
        assert_eq!(p.a, 255);
    }

    #[test]
    fn out_of_bounds_painting_is_ignored() {
        let mut c = Canvas::new(4, 4);
        c.fill_rect(Rect::new(-10, -10, 5, 5), Rgba::rgba(255, 0, 0, 255));
        c.blend(100, 100, Rgba::rgba(255, 0, 0, 255));
        assert_eq!(c.pixel(0, 0).r, 0);
        assert_eq!(c.pixel(3, 3).r, 0);
    }

    #[test]
    fn round_rect_clears_corners() {
        let mut c = Canvas::new(20, 20);
        c.fill_round_rect(Rect::new(0, 0, 20, 20), 8, Rgba::rgba(255, 0, 0, 255));
        assert_eq!(c.pixel(0, 0).r, 0, "corner should stay clear");
        assert_eq!(c.pixel(10, 10).r, 255, "centre should be filled");
    }

    #[test]
    fn checked_box_differs_from_unchecked() {
        let on = canvas_for(
            &View::new(Node::column(vec![Node::checkbox("A", true, "a")])),
            200,
            100,
        );
        let off = canvas_for(
            &View::new(Node::column(vec![Node::checkbox("A", false, "a")])),
            200,
            100,
        );
        assert_ne!(on.pixels(), off.pixels());
    }

    #[test]
    fn ppm_header_is_well_formed() {
        let c = Canvas::new(3, 2);
        let ppm = c.to_ppm();
        let header = String::from_utf8_lossy(&ppm[..11]).to_string();
        assert_eq!(header, "P6\n3 2\n255\n");
        assert_eq!(ppm.len(), 11 + 3 * 2 * 3);
    }

    #[test]
    fn ascii_preview_has_expected_shape() {
        let c = canvas_for(&View::new(Node::column(vec![Node::label("hi")])), 64, 64);
        let art = c.to_ascii();
        let lines: Vec<&str> = art.lines().collect();
        assert_eq!(lines.len(), 4);
        assert!(lines.iter().all(|l| l.chars().count() == 8));
    }

    #[test]
    fn every_glyph_in_the_font_is_reachable() {
        // A code point list that is not sorted silently breaks `binary_search`,
        // which shows up as hollow boxes in the UI. Guard it here.
        let mut previous = 0;
        for &code in &crate::font::CODES {
            assert!(code > previous, "font codes must be strictly ascending");
            previous = code;
            let ch = char::from_u32(code).expect("valid scalar");
            assert!(crate::font::coverage(ch).is_some(), "U+{code:04X} unreachable");
        }
    }

    #[test]
    fn unknown_glyph_draws_a_box_instead_of_nothing() {
        let mut c = Canvas::new(20, 20);
        c.draw_glyph(2, 2, '\u{4E00}', Rgba::rgb(255, 255, 255));
        let inked = (0..20)
            .flat_map(|y| (0..20).map(move |x| (x, y)))
            .filter(|&(x, y)| c.pixel(x, y).r > 0)
            .count();
        assert!(inked > 10, "expected a fallback box, got {inked} pixels");
    }

    #[test]
    fn canvas_is_deterministic() {
        let view = View::new(Node::column(vec![Node::button("Go", "go")]));
        let a = canvas_for(&view, 160, 90);
        let b = canvas_for(&view, 160, 90);
        assert_eq!(a, b);
    }
}

#[cfg(test)]
mod shot {
    use super::paint;
    use crate::layout::layout;
    use crate::view::{Node, Rect, View};

    #[test]
    #[ignore = "writes /tmp/conduit_frame.ppm for manual inspection"]
    fn write_frame() {
        let view = View::new(Node::column(vec![
            Node::label("Conduit — 127.0.0.1:31284"),
            Node::Separator,
            Node::section_with("Fragmentation", true, "sec.strategy", vec![
                Node::radio("TLS records, 5 bytes", true, "strat.5"),
                Node::radio("TLS records, 2 bytes", false, "strat.2"),
                Node::radio("Off", false, "strat.none"),
            ]),
            Node::section_with("DNS", true, "sec.dns", vec![
                Node::radio("Cloudflare", true, "dns.cf"),
                Node::radio("Google", false, "dns.google"),
                Node::radio("Quad9", false, "dns.quad9"),
            ]),
            Node::section_with("Options", true, "sec.options", vec![
                Node::checkbox("System proxy", false, "opt.sysproxy"),
                Node::checkbox("Kill switch", false, "opt.killswitch"),
                Node::checkbox("Minimize to tray", true, "opt.tray"),
            ]),
            Node::Separator,
            Node::button("Speed test", "act.speedtest"),
        ]));
        let bounds = Rect::new(0, 0, 420, 560);
        let canvas = paint(&layout(&view, bounds), bounds.w, bounds.h);
        std::fs::write("/tmp/conduit_frame.ppm", canvas.to_ppm()).unwrap();
        println!("{}", canvas.to_ascii());
    }
}
