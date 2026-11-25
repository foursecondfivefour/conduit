//! Widget description: a UI is data, top to bottom.
//!
//! There is no widget object graph, no callbacks owned by widgets, and no
//! layout engine that mutates state. A screen is a [`Node`] value; rendering
//! produces paint commands; input is matched against a rect table. Swapping a
//! screen means swapping one value.

#![forbid(unsafe_code)]

use core::fmt;

/// A colour as straight (non-premultiplied) RGBA.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct Rgba {
    /// Red.
    pub r: u8,
    /// Green.
    pub g: u8,
    /// Blue.
    pub b: u8,
    /// Alpha; `255` is opaque.
    pub a: u8,
}

impl Rgba {
    /// Fully opaque colour.
    #[must_use]
    pub const fn rgb(r: u8, g: u8, b: u8) -> Self {
        Self { r, g, b, a: 255 }
    }

    /// Colour with explicit alpha.
    #[must_use]
    pub const fn rgba(r: u8, g: u8, b: u8, a: u8) -> Self {
        Self { r, g, b, a }
    }

    /// Returns the colour with a different alpha.
    #[must_use]
    pub const fn with_alpha(self, a: u8) -> Self {
        Self { a, ..self }
    }
}

/// Palette used by [`ThemeChoice::Dark`] and [`ThemeChoice::Light`].
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Theme {
    /// Window background.
    pub background: Rgba,
    /// Body text.
    pub foreground: Rgba,
    /// Secondary text and disabled labels.
    pub muted: Rgba,
    /// Interactive accent (checked rows, focus ring).
    pub accent: Rgba,
    /// Row background on hover.
    pub hover: Rgba,
    /// Borders and separators.
    pub border: Rgba,
}

impl Default for Theme {
    fn default() -> Self {
        Self::DARK
    }
}

impl Theme {
    /// Dark palette, matching the app icon.
    pub const DARK: Self = Self {
        background: Rgba::rgb(11, 12, 60),
        foreground: Rgba::rgb(236, 238, 248),
        muted: Rgba::rgb(150, 158, 190),
        accent: Rgba::rgb(48, 166, 252),
        hover: Rgba::rgb(32, 40, 96),
        border: Rgba::rgb(52, 60, 120),
    };

    /// Light palette.
    pub const LIGHT: Self = Self {
        background: Rgba::rgb(248, 249, 252),
        foreground: Rgba::rgb(24, 26, 40),
        muted: Rgba::rgb(104, 110, 132),
        accent: Rgba::rgb(37, 99, 235),
        hover: Rgba::rgb(226, 232, 248),
        border: Rgba::rgb(206, 212, 230),
    };
}

/// Which palette a screen asks for.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum ThemeChoice {
    /// Dark palette.
    #[default]
    Dark,
    /// Light palette.
    Light,
}

impl ThemeChoice {
    /// Resolves the palette.
    #[must_use]
    pub const fn theme(self) -> Theme {
        match self {
            Self::Dark => Theme::DARK,
            Self::Light => Theme::LIGHT,
        }
    }
}

/// Rectangle in device pixels, origin at the window's top-left.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct Rect {
    /// Left edge.
    pub x: i16,
    /// Top edge.
    pub y: i16,
    /// Width; never negative.
    pub w: u16,
    /// Height; never negative.
    pub h: u16,
}

impl Rect {
    /// Builds a rectangle.
    #[must_use]
    pub const fn new(x: i16, y: i16, w: u16, h: u16) -> Self {
        Self { x, y, w, h }
    }

    /// Right edge (exclusive).
    #[must_use]
    pub const fn right(self) -> i16 {
        self.x.saturating_add(self.w as i16)
    }

    /// Bottom edge (exclusive).
    #[must_use]
    pub const fn bottom(self) -> i16 {
        self.y.saturating_add(self.h as i16)
    }

    /// Reports whether a point is inside.
    #[must_use]
    pub const fn contains(self, px: i16, py: i16) -> bool {
        px >= self.x && px < self.right() && py >= self.y && py < self.bottom()
    }

    /// Shrinks the rectangle on all sides, saturating at empty.
    #[must_use]
    pub const fn inset(self, by: u16) -> Self {
        let dw = by.saturating_mul(2);
        if self.w <= dw || self.h <= dw {
            return Self::new(self.x.saturating_add(by as i16), self.y.saturating_add(by as i16), 0, 0);
        }
        Self::new(
            self.x.saturating_add(by as i16),
            self.y.saturating_add(by as i16),
            self.w.saturating_sub(dw),
            self.h.saturating_sub(dw),
        )
    }

    /// Cuts `height` pixels off the top, returning the removed band and the rest.
    #[must_use]
    pub const fn split_top(self, height: u16) -> (Self, Self) {
        if height >= self.h {
            return (self, Self::new(self.x, self.y.saturating_add(self.h as i16), self.w, 0));
        }
        let top = Self::new(self.x, self.y, self.w, height);
        let rest = Self::new(
            self.x,
            self.y.saturating_add(height as i16),
            self.w,
            self.h.saturating_sub(height),
        );
        (top, rest)
    }

    /// True when the rectangle has no area.
    #[must_use]
    pub const fn is_empty(self) -> bool {
        self.w == 0 || self.h == 0
    }
}

/// Text alignment inside a rect.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum Align {
    /// Left edge.
    #[default]
    Start,
    /// Centred.
    Center,
    /// Right edge.
    End,
}

/// Horizontal growth policy of a node.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum Width {
    /// Take the natural width of the content.
    #[default]
    Content,
    /// Fill the parent's remaining width.
    Fill,
    /// Exactly this many pixels.
    Fixed(u16),
}

/// A widget, described as data.
///
/// Every variant carries the identifiers the app needs to react to input: an
/// [`Action`] id on interactive widgets, or a [`WidgetId`] for focus targets.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Node {
    /// Stacks children vertically.
    Column {
        /// Children, painted top to bottom.
        children: Vec<Node>,
        /// Gap between children.
        gap: u16,
        /// Outer padding.
        padding: u16,
    },
    /// Stacks children horizontally.
    Row {
        /// Children, painted left to right.
        children: Vec<Node>,
        /// Gap between children.
        gap: u16,
        /// Outer padding.
        padding: u16,
    },
    /// Fills its area with a background colour.
    Panel {
        /// Fill colour.
        fill: Rgba,
        /// Corner radius in pixels.
        radius: u16,
        /// Child, painted inset by `radius`.
        child: Option<Box<Node>>,
    },
    /// A run of text.
    Label {
        /// Text to draw.
        text: String,
        /// Text colour.
        color: Rgba,
        /// Alignment within the node's rect.
        align: Align,
        /// Height in pixels; `0` means the theme's line height.
        height: u16,
        /// Focus identifier, when the label is a heading for a control.
        id: Option<WidgetId>,
    },
    /// A checkable row; the whole row is clickable.
    Checkbox {
        /// Row label.
        label: String,
        /// Whether the box is ticked.
        checked: bool,
        /// Action id emitted with the *new* value on click.
        action: Action,
        /// Whether clicks are ignored.
        enabled: bool,
    },
    /// A radio row; the whole row is clickable.
    Radio {
        /// Row label.
        label: String,
        /// Whether this option is the selected one.
        selected: bool,
        /// Action id emitted on click.
        action: Action,
        /// Whether clicks are ignored.
        enabled: bool,
    },
    /// A full-width button.
    Button {
        /// Button text.
        label: String,
        /// Action id emitted on click.
        action: Action,
        /// Whether clicks are ignored.
        enabled: bool,
    },
    /// A horizontal rule.
    Separator,
    /// A collapsing section holding children.
    Section {
        /// Section title.
        title: String,
        /// Whether the children are shown.
        open: bool,
        /// Action id emitted when the title row is clicked.
        action: Action,
        /// Children, painted when `open`.
        children: Vec<Node>,
    },
    /// A fixed-height spacer.
    Spacer {
        /// Height in pixels.
        height: u16,
    },
    /// Wraps a child and keeps the rest of the column below it.
    Fixed {
        /// Height in pixels.
        height: u16,
        /// Wrapped child.
        child: Box<Node>,
    },
}

impl Node {
    /// A column with no padding and a 4 px gap.
    #[must_use]
    pub fn column(children: Vec<Self>) -> Self {
        Self::Column {
            children,
            gap: 4,
            padding: 0,
        }
    }

    /// A row with no padding and a 8 px gap.
    #[must_use]
    pub fn row(children: Vec<Self>) -> Self {
        Self::Row {
            children,
            gap: 8,
            padding: 0,
        }
    }

    /// A body label.
    #[must_use]
    pub fn label(text: impl Into<String>) -> Self {
        Self::Label {
            text: text.into(),
            color: Rgba::default(),
            align: Align::Start,
            height: 0,
            id: None,
        }
    }

    /// Builds a checkbox row.
    #[must_use]
    pub fn checkbox(label: impl Into<String>, checked: bool, action: impl Into<String>) -> Self {
        Self::Checkbox {
            label: label.into(),
            checked,
            action: Action::new(action),
            enabled: true,
        }
    }

    /// Builds a radio row.
    #[must_use]
    pub fn radio(label: impl Into<String>, selected: bool, action: impl Into<String>) -> Self {
        Self::Radio {
            label: label.into(),
            selected,
            action: Action::new(action),
            enabled: true,
        }
    }

    /// Builds a button.
    #[must_use]
    pub fn button(label: impl Into<String>, action: impl Into<String>) -> Self {
        Self::Button {
            label: label.into(),
            action: Action::new(action),
            enabled: true,
        }
    }

    /// Builds a collapsing section.
    #[must_use]
    pub fn section(title: impl Into<String>, open: bool, children: Vec<Self>) -> Self {
        Self::Section {
            title: title.into(),
            open,
            action: Action::new(""), // replaced by `Node::section_with`
            children,
        }
    }

    /// Builds a collapsing section with an explicit action id.
    #[must_use]
    pub fn section_with(
        title: impl Into<String>,
        open: bool,
        action: impl Into<String>,
        children: Vec<Self>,
    ) -> Self {
        Self::Section {
            title: title.into(),
            open,
            action: Action::new(action),
            children,
        }
    }

    /// Visits every interactive action in paint order.
    pub fn actions(&self, out: &mut Vec<Action>) {
        match self {
            Self::Checkbox { action, .. }
            | Self::Radio { action, .. }
            | Self::Button { action, .. } => {
                if !action.is_empty() {
                    out.push(action.clone());
                }
            }
            Self::Section {
                action, children, ..
            } => {
                if !action.is_empty() {
                    out.push(action.clone());
                }
                for child in children {
                    child.actions(out);
                }
            }
            Self::Column { children, .. } | Self::Row { children, .. } => {
                for child in children {
                    child.actions(out);
                }
            }
            Self::Panel { child, .. } => {
                if let Some(child) = child {
                    child.actions(out);
                }
            }
            Self::Fixed { child, .. } => child.actions(out),
            Self::Label { .. } | Self::Separator | Self::Spacer { .. } => {}
        }
    }
}

/// Identifier of an interactive action, chosen by the app.
#[derive(Debug, Clone, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct Action(String);

impl Action {
    /// Wraps an identifier.
    #[must_use]
    pub fn new(id: impl Into<String>) -> Self {
        Self(id.into())
    }

    /// The identifier.
    #[must_use]
    pub fn as_str(&self) -> &str {
        &self.0
    }

    /// True when the identifier is empty (placeholder action).
    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }
}

impl fmt::Display for Action {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

/// Stable identifier for focusable, non-action widgets.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default)]
pub struct WidgetId(pub u32);

/// A complete screen: one root node plus the palette it wants.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct View {
    /// Palette request.
    pub theme: ThemeChoice,
    /// Root node; normally a [`Node::Column`].
    pub root: Node,
    /// Window title.
    pub title: String,
}

impl View {
    /// Wraps a root node.
    #[must_use]
    pub fn new(root: Node) -> Self {
        Self {
            theme: ThemeChoice::Dark,
            root,
            title: String::from("Conduit"),
        }
    }

    /// Replaces the window title.
    #[must_use]
    pub fn with_title(mut self, title: impl Into<String>) -> Self {
        self.title = title.into();
        self
    }

    /// Replaces the palette.
    #[must_use]
    pub const fn with_theme(mut self, theme: ThemeChoice) -> Self {
        self.theme = theme;
        self
    }

    /// Every action reachable from this view, in paint order.
    #[must_use]
    pub fn actions(&self) -> Vec<Action> {
        let mut out = Vec::new();
        self.root.actions(&mut out);
        out
    }
}

#[cfg(test)]
mod tests {
    use super::{Action, Align, Node, Rect, Rgba, Theme, ThemeChoice, View};

    #[test]
    fn rect_edges_and_containment() {
        let r = Rect::new(10, 20, 100, 50);
        assert_eq!(r.right(), 110);
        assert_eq!(r.bottom(), 70);
        assert!(r.contains(10, 20));
        assert!(r.contains(109, 69));
        assert!(!r.contains(110, 69), "right edge is exclusive");
        assert!(!r.contains(9, 20));
    }

    #[test]
    fn rect_inset_saturates() {
        let r = Rect::new(0, 0, 10, 10).inset(3);
        assert_eq!((r.x, r.y, r.w, r.h), (3, 3, 4, 4));
        let empty = Rect::new(0, 0, 4, 4).inset(5);
        assert!(empty.is_empty());
    }

    #[test]
    fn split_top_hands_out_bands() {
        let r = Rect::new(0, 0, 100, 30);
        let (top, rest) = r.split_top(10);
        assert_eq!((top.y, top.h), (0, 10));
        assert_eq!((rest.y, rest.h), (10, 20));
        let (all, none) = r.split_top(100);
        assert_eq!(all, r);
        assert!(none.is_empty());
    }

    #[test]
    fn collects_actions_in_paint_order() {
        let view = View::new(Node::column(vec![
            Node::checkbox("A", false, "a"),
            Node::radio("B", true, "b"),
            Node::section_with("S", true, "s", vec![Node::button("C", "c")]),
        ]));
        let got: Vec<String> = view
            .actions()
            .iter()
            .map(|a| a.as_str().to_owned())
            .collect();
        assert_eq!(got, ["a", "b", "s", "c"]);
    }

    #[test]
    fn placeholder_actions_are_skipped() {
        let view = View::new(Node::column(vec![Node::section("S", true, vec![])]));
        assert!(view.actions().is_empty());
    }

    #[test]
    fn themes_have_contrasting_text() {
        for theme in [Theme::DARK, Theme::LIGHT] {
            let fg = theme.foreground;
            let bg = theme.background;
            let sum = |c: Rgba| u32::from(c.r) + u32::from(c.g) + u32::from(c.b);
            assert!(
                sum(fg).abs_diff(sum(bg)) > 300,
                "foreground and background must be far apart"
            );
        }
    }

    #[test]
    fn theme_choice_resolves() {
        assert_eq!(ThemeChoice::Dark.theme(), Theme::DARK);
        assert_eq!(ThemeChoice::Light.theme(), Theme::LIGHT);
        assert_eq!(ThemeChoice::default(), ThemeChoice::Dark);
    }

    #[test]
    fn label_builder_defaults() {
        let Node::Label {
            color,
            align,
            height,
            id,
            ..
        } = Node::label("x")
        else {
            panic!("expected label");
        };
        assert_eq!(color, Rgba::default());
        assert_eq!(align, Align::Start);
        assert_eq!(height, 0);
        assert!(id.is_none());
    }

    #[test]
    fn action_identity_and_ordering() {
        assert_eq!(Action::new("a"), Action::new("a"));
        assert_ne!(Action::new("a"), Action::new("b"));
        assert!(Action::new("").is_empty());
        assert_eq!(Action::new("quit").to_string(), "quit");
    }

    #[test]
    fn view_defaults() {
        let v = View::new(Node::column(vec![]));
        assert_eq!(v.theme, ThemeChoice::Dark);
        assert_eq!(v.title, "Conduit");
        assert_eq!(v.with_title("T").title, "T");
    }
}
