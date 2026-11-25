//! Layout: turn a [`View`] into flat rects.
//!
//! The result is a [`Layout`]: every painted region, every hit target, and every
//! focusable row in paint order. There is no retained tree — the driver rebuilds
//! this by calling [`layout`] after each state change, which keeps rendering and
//! input handling looking at exactly the same numbers.

#![forbid(unsafe_code)]

use crate::view::{Action, Align, Node, Rect, Rgba, Theme, View};

/// Height of a plain text line.
pub const LINE_HEIGHT: u16 = 20;

/// Height of an interactive row (checkbox, radio, button).
pub const ROW_HEIGHT: u16 = 32;

/// Height of a section title row.
pub const SECTION_HEIGHT: u16 = 30;

/// Height of a separator.
pub const SEPARATOR_HEIGHT: u16 = 1;

/// Vertical space above and below a hairline separator.
pub const SEPARATOR_MARGIN: u16 = 8;

/// Total height a separator occupies, hairline plus margins.
pub const SEPARATOR_BLOCK: u16 = SEPARATOR_HEIGHT + SEPARATOR_MARGIN * 2;

/// Default outer padding for a screen.
pub const SCREEN_PADDING: u16 = 16;

/// Kind of painted region, so the renderer knows what to draw.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RegionKind {
    /// Filled rectangle, optionally rounded.
    Panel,
    /// Border-only rectangle.
    Frame,
    /// Text run.
    Text,
    /// Checkbox indicator square.
    CheckboxBox,
    /// Radio indicator circle.
    RadioDot,
    /// Solid separator line.
    Separator,
}

/// One painted region.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Region {
    /// Where to paint.
    pub rect: Rect,
    /// What to paint.
    pub kind: RegionKind,
    /// Primary colour: fill, border, or text.
    pub color: Rgba,
    /// Corner radius, for panels and boxes.
    pub radius: u16,
    /// Text to draw, for [`RegionKind::Text`].
    pub text: String,
    /// Text alignment, for [`RegionKind::Text`].
    pub align: Align,
}

impl Region {
    fn panel(rect: Rect, color: Rgba, radius: u16) -> Self {
        Self {
            rect,
            kind: RegionKind::Panel,
            color,
            radius,
            text: String::new(),
            align: Align::Start,
        }
    }

    fn text(rect: Rect, text: &str, color: Rgba, align: Align) -> Self {
        Self {
            rect,
            kind: RegionKind::Text,
            color,
            radius: 0,
            text: text.to_owned(),
            align,
        }
    }
}

/// An interactive region and the action it emits.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Hit {
    /// Clickable area.
    pub rect: Rect,
    /// Action emitted on click.
    pub action: Action,
    /// Whether clicks are ignored.
    pub enabled: bool,
    /// Index of the matching entry in [`Layout::rows`], when the hit is a row.
    pub row: Option<usize>,
}

/// A focusable row, in paint order.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Row {
    /// Row area.
    pub rect: Rect,
    /// Action emitted when the row is activated with the keyboard.
    pub action: Action,
    /// Whether keyboard activation is ignored.
    pub enabled: bool,
}

/// The full layout of one view.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct Layout {
    /// Regions in paint order (later regions paint over earlier ones).
    pub regions: Vec<Region>,
    /// Clickable targets.
    pub hits: Vec<Hit>,
    /// Focusable rows in paint order.
    pub rows: Vec<Row>,
    /// Height actually consumed, which may exceed the window.
    pub content_height: u16,
    /// Palette resolved from the view.
    pub theme: Theme,
}

impl Layout {
    /// Topmost hit containing the point, searched back to front.
    #[must_use]
    pub fn hit_test(&self, x: i16, y: i16) -> Option<&Hit> {
        self.hits
            .iter()
            .rev()
            .find(|h| h.enabled && h.rect.contains(x, y))
    }

    /// Row rects, for focus bookkeeping.
    #[must_use]
    pub fn row_count(&self) -> usize {
        self.rows.len()
    }
}

/// Lays a view out inside `bounds`.
#[must_use]
pub fn layout(view: &View, bounds: Rect) -> Layout {
    let mut out = Layout {
        theme: view.theme.theme(),
        ..Layout::default()
    };
    if bounds.is_empty() {
        return out;
    }
    // Padding shrinks on small windows instead of collapsing the body to nothing.
    let pad = SCREEN_PADDING
        .min(bounds.w / 4)
        .min(bounds.h / 4);
    let body = bounds.inset(pad);
    out.regions
        .push(Region::panel(bounds, out.theme.background, 0));
    let consumed = walk(view, &view.root, body, &mut out);
    let total = consumed.saturating_add(pad.saturating_mul(2));
    out.content_height = total.max(bounds.h);
    out
}

/// Height a node occupies when it is given free rein.
fn natural_height(node: &Node) -> u16 {
    match node {
        Node::Column {
            children,
            gap,
            padding,
        } => {
            let inner: u16 = children
                .iter()
                .map(natural_height)
                .fold(0u16, |acc, h| acc.saturating_add(h));
            let gaps = gap.saturating_mul(children.len().saturating_sub(1) as u16);
            inner
                .saturating_add(gaps)
                .saturating_add(padding.saturating_mul(2))
        }
        Node::Row { children, padding, .. } => children
            .iter()
            .map(natural_height)
            .max()
            .unwrap_or(0)
            .saturating_add(padding.saturating_mul(2)),
        Node::Panel { radius, child, .. } => child
            .as_ref()
            .map(|c| natural_height(c).saturating_add(radius.saturating_mul(2)))
            .unwrap_or(0),
        Node::Label { height, .. } => {
            if *height == 0 {
                LINE_HEIGHT
            } else {
                *height
            }
        }
        Node::Checkbox { .. } | Node::Radio { .. } | Node::Button { .. } => ROW_HEIGHT,
        Node::Separator => SEPARATOR_BLOCK,
        Node::Section {
            open, children, ..
        } => {
            let inner: u16 = children
                .iter()
                .map(natural_height)
                .fold(0u16, |acc, h| acc.saturating_add(h));
            if *open {
                SECTION_HEIGHT.saturating_add(inner)
            } else {
                SECTION_HEIGHT
            }
        }
        Node::Spacer { height } => *height,
        Node::Fixed { height, .. } => *height,
    }
}

/// Paints one node and returns the height it consumed.
fn walk(view: &View, node: &Node, area: Rect, out: &mut Layout) -> u16 {
    let theme = view.theme.theme();
    match node {
        Node::Column {
            children,
            gap,
            padding,
        } => {
            let inner = area.inset(*padding);
            let mut cursor = inner.y;
            let mut consumed: u16 = 0;
            for (index, child) in children.iter().enumerate() {
                if index > 0 {
                    let gap_i16 = i16::try_from(*gap).unwrap_or(i16::MAX);
                    cursor = cursor.saturating_add(gap_i16);
                    consumed = consumed.saturating_add(*gap);
                }
                // Give the child the full remaining height and let it report what
                // it actually used, so later siblings still get their space.
                let remaining = inner.h.saturating_sub(consumed);
                let natural = natural_height(child).min(remaining.max(LINE_HEIGHT));
                let slot = Rect::new(inner.x, cursor, inner.w, natural);
                let used = walk(view, child, slot, out);
                // A node that consumed nothing (a separator) still advances by its
                // own height so following siblings do not overlap it.
                let advance = used.max(natural_height(child).min(natural));
                cursor = cursor.saturating_add(advance as i16);
                consumed = consumed.saturating_add(advance);
            }
            consumed.saturating_add(padding.saturating_mul(2))
        }
        Node::Row {
            children,
            gap,
            padding,
        } => {
            let inner = area.inset(*padding);
            if children.is_empty() {
                return padding.saturating_mul(2);
            }
            let gaps = gap.saturating_mul(children.len().saturating_sub(1) as u16);
            let avail = inner.w.saturating_sub(gaps);
            let share = match u16::try_from(children.len()) {
                Ok(n) if n > 0 => avail / n,
                _ => 0,
            };
            let mut cursor = inner.x;
            let mut tallest = 0;
            for child in children {
                let slot = Rect::new(cursor, inner.y, share, inner.h);
                let used = walk(view, child, slot, out);
                tallest = tallest.max(used);
                cursor = cursor.saturating_add(share as i16 + *gap as i16);
            }
            tallest.saturating_add(padding.saturating_mul(2))
        }
        Node::Panel {
            fill,
            radius,
            child,
        } => {
            out.regions
                .push(Region::panel(area, *fill, *radius));
            match child {
                Some(child) => {
                    let inner = area.inset(*radius);
                    walk(view, child, inner, out).saturating_add(radius.saturating_mul(2))
                }
                None => area.h,
            }
        }
        Node::Label {
            text,
            color,
            align,
            height,
            ..
        } => {
            let h = if *height == 0 { LINE_HEIGHT } else { *height };
            let band = Rect::new(area.x, area.y, area.w, h.min(area.h.max(h)));
            let fg = if *color == Rgba::default() {
                theme.foreground
            } else {
                *color
            };
            out.regions.push(Region::text(band, text, fg, *align));
            h
        }
        Node::Checkbox {
            label,
            checked,
            action,
            enabled,
        }
        | Node::Radio {
            label,
            selected: checked,
            action,
            enabled,
        } => {
            let row = Rect::new(area.x, area.y, area.w, ROW_HEIGHT.min(area.h.max(ROW_HEIGHT)));
            let is_radio = matches!(node, Node::Radio { .. });
            let fg = if *enabled {
                theme.foreground
            } else {
                theme.muted
            };
            let box_rect = Rect::new(row.x, row.y + 8, 16, 16);
            if is_radio {
                out.regions.push(Region {
                    rect: box_rect,
                    kind: RegionKind::RadioDot,
                    color: if *checked { theme.accent } else { theme.border },
                    radius: 8,
                    text: String::new(),
                    align: Align::Center,
                });
            } else {
                out.regions.push(Region {
                    rect: box_rect,
                    kind: RegionKind::CheckboxBox,
                    color: if *checked { theme.accent } else { theme.muted },
                    radius: 3,
                    text: if *checked {
                        String::from("x")
                    } else {
                        String::new()
                    },
                    align: Align::Center,
                });
            }
            let text_rect = Rect::new(
                row.x + 24,
                row.y.saturating_add(((ROW_HEIGHT.saturating_sub(LINE_HEIGHT)) / 2) as i16),
                row.w.saturating_sub(24),
                LINE_HEIGHT,
            );
            out.regions
                .push(Region::text(text_rect, label, fg, Align::Start));
            let row_index = out.rows.len();
            out.rows.push(Row {
                rect: row,
                action: action.clone(),
                enabled: *enabled,
            });
            out.hits.push(Hit {
                rect: row,
                action: action.clone(),
                enabled: *enabled,
                row: Some(row_index),
            });
            ROW_HEIGHT
        }
        Node::Button {
            label,
            action,
            enabled,
        } => {
            let row = Rect::new(area.x, area.y, area.w, ROW_HEIGHT.min(area.h.max(ROW_HEIGHT)));
            let fill = if *enabled { theme.hover } else { theme.background };
            out.regions.push(Region::panel(row, fill, 6));
            let fg = if *enabled {
                theme.accent
            } else {
                theme.muted
            };
            out.regions
                .push(Region::text(row, label, fg, Align::Center));
            let row_index = out.rows.len();
            out.rows.push(Row {
                rect: row,
                action: action.clone(),
                enabled: *enabled,
            });
            out.hits.push(Hit {
                rect: row,
                action: action.clone(),
                enabled: *enabled,
                row: Some(row_index),
            });
            ROW_HEIGHT
        }
        Node::Separator => {
            // A hairline reads as a divider only with space above and below it.
            let band = Rect::new(
                area.x,
                area.y.saturating_add(SEPARATOR_MARGIN as i16),
                area.w,
                SEPARATOR_HEIGHT,
            );
            out.regions.push(Region {
                rect: band,
                kind: RegionKind::Separator,
                color: theme.border,
                radius: 0,
                text: String::new(),
                align: Align::Start,
            });
            SEPARATOR_BLOCK
        }
        Node::Section {
            title,
            open,
            action,
            children,
        } => {
            let row = Rect::new(
                area.x,
                area.y,
                area.w,
                SECTION_HEIGHT.min(area.h.max(SECTION_HEIGHT)),
            );
            let marker = if *open { '\u{25BE}' } else { '\u{25B8}' };
            let header = format!("{marker}  {title}");
            let title_rect = Rect::new(
                row.x,
                row.y.saturating_add(((SECTION_HEIGHT.saturating_sub(LINE_HEIGHT)) / 2) as i16),
                row.w,
                LINE_HEIGHT,
            );
            out.regions
                .push(Region::text(title_rect, &header, theme.foreground, Align::Start));
            let row_index = out.rows.len();
            out.rows.push(Row {
                rect: row,
                action: action.clone(),
                enabled: !action.is_empty(),
            });
            out.hits.push(Hit {
                rect: row,
                action: action.clone(),
                enabled: !action.is_empty(),
                row: Some(row_index),
            });
            let mut consumed = SECTION_HEIGHT;
            if *open {
                let inner = Rect::new(
                    area.x.saturating_add(8),
                    area.y.saturating_add(SECTION_HEIGHT as i16),
                    area.w.saturating_sub(8),
                    area.h.saturating_sub(SECTION_HEIGHT),
                );
                let mut cursor = inner.y;
                let mut used_total: u16 = 0;
                for child in children {
                    let natural = natural_height(child);
                    let slot = Rect::new(inner.x, cursor, inner.w, natural);
                    let used = walk(view, child, slot, out);
                    cursor = cursor.saturating_add(used as i16);
                    used_total = used_total.saturating_add(used);
                }
                consumed = consumed.saturating_add(used_total);
            }
            consumed
        }
        Node::Spacer { height } => (*height).min(area.h.max(*height)),
        Node::Fixed { height, child } => {
            let slot = Rect::new(area.x, area.y, area.w, (*height).min(area.h.max(*height)));
            walk(view, child, slot, out);
            *height
        }
    }
}

#[cfg(test)]
mod tests {
    use super::{layout, RegionKind, LINE_HEIGHT, ROW_HEIGHT, SCREEN_PADDING};
    use crate::view::{Action, Node, Rect, View};

    fn bounds() -> Rect {
        Rect::new(0, 0, 400, 600)
    }

    #[test]
    fn column_stacks_children_with_gap() {
        let view = View::new(Node::Column {
            children: vec![Node::checkbox("A", false, "a"), Node::checkbox("B", false, "b")],
            gap: 4,
            padding: 0,
        });
        let l = layout(&view, bounds());
        assert_eq!(l.rows.len(), 2);
        assert_eq!(l.rows[0].rect.y, SCREEN_PADDING as i16);
        assert_eq!(l.rows[1].rect.y, SCREEN_PADDING as i16 + ROW_HEIGHT as i16 + 4);
    }

    #[test]
    fn hit_test_finds_row() {
        let view = View::new(Node::column(vec![Node::checkbox("A", true, "a")]));
        let l = layout(&view, bounds());
        let hit = l.hit_test(30, SCREEN_PADDING as i16 + 10).expect("hit");
        assert_eq!(hit.action, Action::new("a"));
        assert!(l.hit_test(30, 599).is_none());
    }

    #[test]
    fn disabled_rows_are_not_hittable() {
        let mut node = Node::checkbox("A", false, "a");
        if let Node::Checkbox { enabled, .. } = &mut node {
            *enabled = false;
        }
        let view = View::new(Node::column(vec![node]));
        let l = layout(&view, bounds());
        assert!(l.hit_test(30, SCREEN_PADDING as i16 + 10).is_none());
    }

    #[test]
    fn label_uses_default_colour_and_line_height() {
        let view = View::new(Node::column(vec![Node::label("hello")]));
        let l = layout(&view, bounds());
        let text = l
            .regions
            .iter()
            .find(|r| r.kind == RegionKind::Text)
            .expect("text region");
        assert_eq!(text.text, "hello");
        assert_eq!(text.rect.h, LINE_HEIGHT);
        assert_eq!(text.color, l.theme.foreground);
    }

    #[test]
    fn section_hides_children_when_closed() {
        let open = View::new(Node::column(vec![Node::section_with(
            "S",
            true,
            "s",
            vec![Node::checkbox("A", false, "a")],
        )]));
        let closed = View::new(Node::column(vec![Node::section_with(
            "S",
            false,
            "s",
            vec![Node::checkbox("A", false, "a")],
        )]));
        assert_eq!(layout(&open, bounds()).rows.len(), 2);
        assert_eq!(layout(&closed, bounds()).rows.len(), 1);
    }

    #[test]
    fn section_paints_each_child_once() {
        let view = View::new(Node::column(vec![Node::section_with(
            "S",
            true,
            "s",
            vec![Node::checkbox("A", false, "a"), Node::checkbox("B", false, "b")],
        )]));
        let l = layout(&view, bounds());
        let actions: Vec<&str> = l.rows.iter().map(|r| r.action.as_str()).collect();
        assert_eq!(actions, ["s", "a", "b"]);
    }

    #[test]
    fn content_height_grows_past_window() {
        let many: Vec<Node> = (0..40)
            .map(|i| Node::checkbox(format!("row {i}"), false, format!("r{i}")))
            .collect();
        let l = layout(&View::new(Node::column(many)), bounds());
        assert!(l.content_height > bounds().h);
    }

    #[test]
    fn empty_bounds_produce_empty_layout() {
        let l = layout(&View::new(Node::column(vec![])), Rect::new(0, 0, 0, 0));
        assert!(l.regions.is_empty());
        assert!(l.hits.is_empty());
    }

    #[test]
    fn separator_is_one_pixel() {
        let view = View::new(Node::column(vec![Node::Separator]));
        let l = layout(&view, bounds());
        assert!(l.regions.iter().any(|r| r.kind == RegionKind::Separator && r.rect.h == 1));
    }

    #[test]
    fn row_targets_cover_full_width() {
        let view = View::new(Node::column(vec![Node::button("Go", "go")]));
        let l = layout(&view, bounds());
        let hit = &l.hits[0];
        assert_eq!(hit.rect.x, SCREEN_PADDING as i16);
        assert_eq!(hit.rect.w, 400 - SCREEN_PADDING * 2);
    }
    #[test]
    fn row_splits_width_evenly() {
        let view = View::new(Node::row(vec![Node::button("A", "a"), Node::button("B", "b")]));
        let l = layout(&view, bounds());
        assert_eq!(l.rows.len(), 2);
        assert_eq!(l.rows[0].rect.w, l.rows[1].rect.w);
        assert!(l.rows[1].rect.x > l.rows[0].rect.x);
    }

    #[test]
    fn fixed_wraps_child_height() {
        let view = View::new(Node::column(vec![Node::Fixed {
            height: 80,
            child: Box::new(Node::label("inside")),
        }]));
        let l = layout(&view, bounds());
        assert_eq!(l.content_height, 600);
        assert!(l.regions.iter().any(|r| r.kind == RegionKind::Text));
    }
}
