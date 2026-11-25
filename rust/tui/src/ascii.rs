//! Text backend: render a [`Layout`] as an ASCII frame.
//!
//! This backend exists so the whole pipeline — view model, layout, paint — can be
//! asserted in CI without a display server. It is also what a future SSH or
//! `-no-gui` console mode would print.

#![forbid(unsafe_code)]

use crate::layout::{Layout, RegionKind};
use crate::view::Align;

/// A rendered frame of glyphs.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Frame {
    width: u16,
    height: u16,
    cells: Vec<char>,
}

impl Frame {
    /// Allocates a frame filled with spaces.
    #[must_use]
    pub fn new(width: u16, height: u16) -> Self {
        let len = usize::from(width) * usize::from(height);
        Self {
            width,
            height,
            cells: vec![' '; len],
        }
    }

    /// Frame width.
    #[must_use]
    pub const fn width(&self) -> u16 {
        self.width
    }

    /// Frame height.
    #[must_use]
    pub const fn height(&self) -> u16 {
        self.height
    }

    /// Writes one glyph, ignoring out-of-bounds coordinates.
    pub fn put(&mut self, x: i16, y: i16, ch: char) {
        if x < 0 || y < 0 {
            return;
        }
        let (Ok(x), Ok(y)) = (u16::try_from(x), u16::try_from(y)) else {
            return;
        };
        if x >= self.width || y >= self.height {
            return;
        }
        let index = usize::from(y) * usize::from(self.width) + usize::from(x);
        if let Some(slot) = self.cells.get_mut(index) {
            *slot = ch;
        }
    }

    /// Writes a string starting at `(x, y)`.
    pub fn put_str(&mut self, x: i16, y: i16, text: &str) {
        for (offset, ch) in text.chars().enumerate() {
            let Ok(dx) = i16::try_from(offset) else {
                break;
            };
            self.put(x.saturating_add(dx), y, ch);
        }
    }

    /// Reads one glyph, or a space outside the frame.
    #[must_use]
    pub fn get(&self, x: u16, y: u16) -> char {
        if x >= self.width || y >= self.height {
            return ' ';
        }
        let index = usize::from(y) * usize::from(self.width) + usize::from(x);
        self.cells.get(index).copied().unwrap_or(' ')
    }

    /// Rows as strings.
    #[must_use]
    pub fn lines(&self) -> Vec<String> {
        let w = usize::from(self.width);
        self.cells
            .chunks(w.max(1))
            .map(|row| row.iter().collect::<String>())
            .collect()
    }

    /// Joins rows with newlines.
    #[must_use]
    pub fn to_text(&self) -> String {
        self.lines().join("\n")
    }
}

/// Renders the layout into a frame, one character per pixel.
#[must_use]
pub fn render(layout: &Layout, width: u16, height: u16) -> Frame {
    render_cells(layout, width, height, 1, 1)
}

/// Renders the layout into a character grid where one cell is
/// `cell_w` by `cell_h` layout pixels.
///
/// A real window is measured in pixels (`420 x 560`), while a terminal cell is
/// roughly `8 x 16`. Dividing the pixel space by the cell size lets the same
/// [`Layout`] be inspected in a terminal without any special-casing.
#[must_use]
pub fn render_cells(layout: &Layout, cols: u16, rows: u16, cell_w: u16, cell_h: u16) -> Frame {
    let mut frame = Frame::new(cols, rows);
    let cell_w = cell_w.max(1);
    let cell_h = cell_h.max(1);
    for region in &layout.regions {
        paint(&mut frame, region, cell_w, cell_h);
    }
    stroke_rect(&mut frame, 0, 0, cols, rows);
    frame
}

/// Converts a layout pixel coordinate to a cell coordinate, rounding to nearest
/// so two regions a few pixels apart do not collapse onto one cell.
fn to_cell(pixels: i16, cell: u16) -> i16 {
    let cell_i16 = i16::try_from(cell).unwrap_or(1).max(1);
    (pixels.max(0).saturating_add(cell_i16 / 2)) / cell_i16
}

/// Paints one region.
fn paint(frame: &mut Frame, region: &crate::layout::Region, cell_w: u16, cell_h: u16) {
    let r = region.rect;
    if r.is_empty() {
        return;
    }
    let x = to_cell(r.x, cell_w);
    let y = to_cell(r.y, cell_h);
    let w = (r.w / cell_w).max(1);
    let h = (r.h / cell_h).max(1);
    match region.kind {
        RegionKind::Panel | RegionKind::Frame => {
            if region.kind == RegionKind::Panel && x == 0 && y == 0 && w >= frame.width() && h >= frame.height() {
                // Full-window background: the border itself is drawn in `render_cells`.
                return;
            }
            stroke_rect(frame, x.max(0) as u16, y.max(0) as u16, w, h);
        }
        RegionKind::Separator => {
            for dx in 0..w {
                let Ok(cx) = u16::try_from(i32::from(x) + i32::from(dx)) else {
                    break;
                };
                frame.put(i16::try_from(cx).unwrap_or(i16::MAX), y, '-');
            }
        }
        RegionKind::CheckboxBox => {
            frame.put(x, y, '[');
            frame.put(x.saturating_add(2), y, ']');
            if !region.text.is_empty() {
                frame.put(x.saturating_add(1), y, 'x');
            }
        }
        RegionKind::RadioDot => {
            frame.put(x, y, if region.text.is_empty() { '(' } else { '*' });
            frame.put(x.saturating_add(2), y, ')');
        }
        RegionKind::Text => {
            let chars = region.text.chars().count();
            let avail = usize::from(w);
            let start = match region.align {
                Align::Start => 0,
                Align::Center => avail.saturating_sub(chars) / 2,
                Align::End => avail.saturating_sub(chars),
            };
            let Ok(dx) = i16::try_from(start) else {
                return;
            };
            frame.put_str(x.saturating_add(dx), y, &region.text);
        }
    }
}

/// Draws the corners of a rectangle.
fn stroke_rect(frame: &mut Frame, x: u16, y: u16, w: u16, h: u16) {
    if w == 0 || h == 0 {
        return;
    }
    let (Ok(x), Ok(y)) = (i16::try_from(x), i16::try_from(y)) else {
        return;
    };
    let right = x.saturating_add(i16::try_from(w).unwrap_or(i16::MAX)).saturating_sub(1);
    let bottom = y.saturating_add(i16::try_from(h).unwrap_or(i16::MAX)).saturating_sub(1);
    frame.put(x, y, '+');
    frame.put(right, y, '+');
    frame.put(x, bottom, '+');
    frame.put(right, bottom, '+');
    let mut cx = x.saturating_add(1);
    while cx < right {
        frame.put(cx, y, '-');
        frame.put(cx, bottom, '-');
        cx = cx.saturating_add(1);
    }
    let mut cy = y.saturating_add(1);
    while cy < bottom {
        frame.put(x, cy, '|');
        frame.put(right, cy, '|');
        cy = cy.saturating_add(1);
    }
}

#[cfg(test)]
mod tests {
    use super::render;
    use crate::layout::layout;
    use crate::view::{Node, Rect, View};

    fn frame_for(view: &View) -> super::Frame {
        let bounds = Rect::new(0, 0, 60, 24);
        render(&layout(view, bounds), bounds.w, bounds.h)
    }

    #[test]
    fn frame_has_requested_size() {
        let f = frame_for(&View::new(Node::column(vec![])));
        assert_eq!((f.width(), f.height()), (60, 24));
        assert_eq!(f.lines().len(), 24);
        assert!(f.lines().iter().all(|l| l.chars().count() == 60));
    }

    #[test]
    fn labels_are_drawn() {
        let f = frame_for(&View::new(Node::column(vec![Node::label("Conduit")])));
        assert!(f.to_text().contains("Conduit"), "{}", f.to_text());
    }

    #[test]
    fn checked_box_shows_marker() {
        let on = frame_for(&View::new(Node::column(vec![Node::checkbox(
            "Auto", true, "auto",
        )])));
        let off = frame_for(&View::new(Node::column(vec![Node::checkbox(
            "Auto", false, "auto",
        )])));
        assert!(on.to_text().contains('x'));
        assert!(!off.to_text().contains('x'));
        assert!(off.to_text().contains("Auto"));
    }

    #[test]
    fn closed_section_hides_title_marker_change() {
        let open = frame_for(&View::new(Node::column(vec![Node::section_with(
            "DNS",
            true,
            "dns",
            vec![Node::label("Cloudflare")],
        )])));
        let closed = frame_for(&View::new(Node::column(vec![Node::section_with(
            "DNS",
            false,
            "dns",
            vec![Node::label("Cloudflare")],
        )])));
        assert!(open.to_text().contains('\u{25BE}'));
        assert!(closed.to_text().contains('\u{25B8}'));
        assert!(!closed.to_text().contains("Cloudflare"));
    }

    #[test]
    fn out_of_bounds_writes_are_ignored() {
        let mut f = super::Frame::new(4, 2);
        f.put(-1, 0, 'x');
        f.put(0, 99, 'x');
        f.put(99, 0, 'x');
        assert_eq!(f.get(0, 0), ' ');
    }

    #[test]
    fn centered_text_is_centered() {
        let mut node = Node::label("mid");
        if let Node::Label { align, .. } = &mut node {
            *align = crate::view::Align::Center;
        }
        let f = frame_for(&View::new(Node::column(vec![node])));
        let line = f
            .lines()
            .into_iter()
            .find(|l| l.contains("mid"))
            .expect("centered line");
        let before = line.find("mid").unwrap_or(0);
        let after = line.len() - before - 3;
        assert!(before.abs_diff(after) <= 3, "line: {line:?}");
    }
}




#[cfg(test)]
mod preview {
    use crate::layout::layout;
    use crate::ascii::render_cells;
    use crate::view::{Node, Rect, View};

    #[test]
    fn print_tray_preview() {
        let view = View::new(Node::column(vec![
            Node::label("Conduit — running on 127.0.0.1:31284"),
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
        let bounds = Rect::new(0, 0, 432, 624);
        let frame = render_cells(&layout(&view, bounds), 54, 39, 8, 16);
        println!("{}", frame.to_text());
    }
}






