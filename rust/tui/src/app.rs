//! The application shell: state, input, and the event loop.
//!
//! The whole UI is a function of [`State`]. A frame is produced by building a
//! [`View`] from the state, laying it out, and painting the result. Input arrives
//! as pixel coordinates, is matched against the layout's hit table, and mutates
//! the state — which rebuilds the frame. Nothing is retained between frames
//! except the state itself.

#![forbid(unsafe_code)]

use crate::layout::{layout, Layout};
use crate::paint::{paint, Canvas};
use crate::view::{Action, Node, Rect, ThemeChoice, View};
use crate::x11::{DisplayName, Error as X11Error, Event, Window};

/// Which fragmentation strategy is selected.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Strategy {
    /// Split every TLS record into five-byte chunks.
    Records5,
    /// Split every TLS record into two-byte chunks.
    Records2,
    /// Do not fragment.
    Off,
}

impl Strategy {
    /// Label shown in the UI.
    #[must_use]
    pub const fn label(self) -> &'static str {
        match self {
            Self::Records5 => "TLS records, 5 bytes",
            Self::Records2 => "TLS records, 2 bytes",
            Self::Off => "Off",
        }
    }

    /// Action string for this strategy.
    #[must_use]
    pub const fn action(self) -> &'static str {
        match self {
            Self::Records5 => "strategy.records5",
            Self::Records2 => "strategy.records2",
            Self::Off => "strategy.off",
        }
    }

    /// Parses an action string back into a strategy.
    #[must_use]
    pub fn from_action(action: &str) -> Option<Self> {
        match action {
            "strategy.records5" => Some(Self::Records5),
            "strategy.records2" => Some(Self::Records2),
            "strategy.off" => Some(Self::Off),
            _ => None,
        }
    }
}

/// DNS resolver choice.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Resolver {
    /// Cloudflare.
    Cloudflare,
    /// Google.
    Google,
    /// Quad9.
    Quad9,
}

impl Resolver {
    /// Label shown in the UI.
    #[must_use]
    pub const fn label(self) -> &'static str {
        match self {
            Self::Cloudflare => "Cloudflare",
            Self::Google => "Google",
            Self::Quad9 => "Quad9",
        }
    }

    /// Action string for this resolver.
    #[must_use]
    pub const fn action(self) -> &'static str {
        match self {
            Self::Cloudflare => "dns.cloudflare",
            Self::Google => "dns.google",
            Self::Quad9 => "dns.quad9",
        }
    }

    /// Parses an action string back into a resolver.
    #[must_use]
    pub fn from_action(action: &str) -> Option<Self> {
        match action {
            "dns.cloudflare" => Some(Self::Cloudflare),
            "dns.google" => Some(Self::Google),
            "dns.quad9" => Some(Self::Quad9),
            _ => None,
        }
    }
}

/// Everything the window shows.
#[derive(Debug, Clone, PartialEq)]
pub struct State {
    /// Local proxy address shown in the header.
    pub address: String,
    /// Selected fragmentation strategy.
    pub strategy: Strategy,
    /// Selected resolver.
    pub resolver: Resolver,
    /// Whether the system proxy is installed.
    pub system_proxy: bool,
    /// Whether the kill switch is armed.
    pub kill_switch: bool,
    /// Whether closing the window hides it to the tray.
    pub minimize_to_tray: bool,
    /// Whether the strategy section is expanded.
    pub strategy_open: bool,
    /// Whether the DNS section is expanded.
    pub dns_open: bool,
    /// Whether the options section is expanded.
    pub options_open: bool,
    /// Last measured throughput, in megabits per second.
    pub speed_mbps: Option<f32>,
}

impl Default for State {
    fn default() -> Self {
        Self {
            address: String::from("127.0.0.1:31284"),
            strategy: Strategy::Records5,
            resolver: Resolver::Cloudflare,
            system_proxy: false,
            kill_switch: false,
            minimize_to_tray: true,
            strategy_open: true,
            dns_open: true,
            options_open: true,
            speed_mbps: None,
        }
    }
}

/// What an action did, so the driver can react outside the state.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Outcome {
    /// The action changed the state; repaint.
    Changed,
    /// The action is unknown or produced no change.
    Ignored,
    /// The user asked to close the window.
    Close,
}

impl State {
    /// Builds the view for the current state.
    #[must_use]
    pub fn view(&self) -> View {
        let mut header = vec![Node::label(format!("Conduit · {}", self.address)), Node::Separator];
        header.push(Node::section_with(
            "Fragmentation",
            self.strategy_open,
            "section.strategy",
            Strategy::all()
                .iter()
                .map(|strategy| {
                    Node::radio(
                        strategy.label(),
                        *strategy == self.strategy,
                        strategy.action(),
                    )
                })
                .collect(),
        ));
        header.push(Node::section_with(
            "DNS",
            self.dns_open,
            "section.dns",
            Resolver::all()
                .iter()
                .map(|resolver| Node::radio(resolver.label(), *resolver == self.resolver, resolver.action()))
                .collect(),
        ));
        header.push(Node::section_with(
            "Options",
            self.options_open,
            "section.options",
            vec![
                Node::checkbox("System proxy", self.system_proxy, "option.system_proxy"),
                Node::checkbox("Kill switch", self.kill_switch, "option.kill_switch"),
                Node::checkbox("Minimize to tray", self.minimize_to_tray, "option.tray"),
            ],
        ));
        header.push(Node::Separator);
        header.push(Node::button(self.speed_label(), "action.speedtest"));
        View::new(Node::column(header)).with_title("Conduit")
    }

    /// Lays the view out for a default window size.
    ///
    /// Useful for tests and for anything that needs the same rects the window
    /// would show; the window itself uses [`App::layout`] with its real size.
    #[must_use]
    pub fn layout(&self) -> Layout {
        layout(&self.view(), Rect::new(0, 0, 420, 560))
    }

    /// Label for the speed-test button, showing the last result if any.
    #[must_use]
    pub fn speed_label(&self) -> String {
        match self.speed_mbps {
            Some(mbps) => format!("Speed test · {mbps:.1} Mbit/s"),
            None => String::from("Speed test"),
        }
    }

    /// Applies an action.
    ///
    /// This is the single place that maps an action string to a state change, so
    /// hit-testing, keyboard activation, and tests all behave identically.
    pub fn apply(&mut self, action: &Action) -> Outcome {
        let name = action.as_str();
        if name.is_empty() {
            return Outcome::Ignored;
        }
        if let Some(strategy) = Strategy::from_action(name) {
            self.strategy = strategy;
            return Outcome::Changed;
        }
        if let Some(resolver) = Resolver::from_action(name) {
            self.resolver = resolver;
            return Outcome::Changed;
        }
        match name {
            "section.strategy" => self.strategy_open = !self.strategy_open,
            "section.dns" => self.dns_open = !self.dns_open,
            "section.options" => self.options_open = !self.options_open,
            "option.system_proxy" => self.system_proxy = !self.system_proxy,
            "option.kill_switch" => self.kill_switch = !self.kill_switch,
            "option.tray" => self.minimize_to_tray = !self.minimize_to_tray,
            "action.speedtest" => {
                // A real measurement belongs to the backend; the UI only records
                // the fact that it was asked for.
                self.speed_mbps = Some(0.0);
            }
            "action.close" => return Outcome::Close,
            _ => return Outcome::Ignored,
        }
        Outcome::Changed
    }
}

impl Strategy {
    /// Every strategy, in display order.
    #[must_use]
    pub const fn all() -> &'static [Self] {
        &[Self::Records5, Self::Records2, Self::Off]
    }
}

impl Resolver {
    /// Every resolver, in display order.
    #[must_use]
    pub const fn all() -> &'static [Self] {
        &[Self::Cloudflare, Self::Google, Self::Quad9]
    }
}

/// A headless driver: state plus the frame it produces.
#[derive(Debug, Clone)]
pub struct App {
    /// Application state.
    pub state: State,
    /// Window size in pixels.
    pub size: (u16, u16),
    /// Colour scheme.
    pub theme: ThemeChoice,
}

impl App {
    /// Creates an app with the given window size.
    #[must_use]
    pub fn new(width: u16, height: u16) -> Self {
        Self {
            state: State::default(),
            size: (width, height),
            theme: ThemeChoice::Dark,
        }
    }

    /// Builds the layout for the current state and size.
    #[must_use]
    pub fn layout(&self) -> Layout {
        let view = self.state.view().with_theme(self.theme);
        layout(&view, Rect::new(0, 0, self.size.0, self.size.1))
    }

    /// Renders the current state into a canvas.
    #[must_use]
    pub fn canvas(&self) -> Canvas {
        paint(&self.layout(), self.size.0, self.size.1)
    }

    /// Handles a click at pixel coordinates.
    ///
    /// Returns the outcome, or `None` when the click hit nothing interactive.
    pub fn click(&mut self, x: i16, y: i16) -> Option<Outcome> {
        let action = self.layout().hit_test(x, y)?.action.clone();
        Some(self.state.apply(&action))
    }

    /// Handles a pointer move, returning whether the hover target changed.
    pub fn hover(&mut self, _x: i16, _y: i16) -> bool {
        // Hover highlighting needs a hover field in the layout; the hit table
        // already answers "what is under the pointer", so this stays a no-op
        // until the theme has a distinct hover state per row.
        false
    }

    /// Resizes the window.
    pub fn resize(&mut self, width: u16, height: u16) {
        self.size = (width.max(120), height.max(120));
    }
}

/// Runs the X11 event loop until the window closes.
///
/// # Errors
///
/// Returns any X11 or socket error that ends the loop.
pub fn run() -> Result<(), X11Error> {
    let display = DisplayName::from_env()?;
    let mut connection = display.connect()?;
    // Opening the connection already parsed the screen; a failure here means the
    // reply was inconsistent, which `connect` would have reported.
    let _screen = connection.screen()?;
    let mut app = App::new(420, 560);
    let window = connection.create_window(app.size.0, app.size.1)?;
    connection.set_title(&window, app.state.view().title.as_str())?;
    let gc = connection.create_gc(&window)?;
    connection.map_window(&window)?;
    blit(&mut connection, &window, gc, &app)?;

    loop {
        match connection.wait_event()? {
            Event::Expose => blit(&mut connection, &window, gc, &app)?,
            Event::Resized { width, height } => {
                app.resize(width, height);
                blit(&mut connection, &window, gc, &app)?;
            }
            Event::ButtonPress { x, y } => {
                if let Some(outcome) = app.click(x, y) {
                    if outcome == Outcome::Close {
                        return Ok(());
                    }
                    blit(&mut connection, &window, gc, &app)?;
                }
            }
            Event::CloseRequested => return Ok(()),
            _ => {}
        }
    }
}

/// Paints the app once and sends the pixels to the window.
fn blit(
    connection: &mut crate::x11::Connection,
    window: &Window,
    gc: u32,
    app: &App,
) -> Result<(), X11Error> {
    let canvas = app.canvas();
    debug_assert_eq!(canvas.width(), window.width);
    debug_assert_eq!(canvas.height(), window.height);
    let bands = connection.put_image(window, gc, canvas.width(), canvas.height(), canvas.pixels())?;
    if bands == 0 {
        return Err(X11Error::Protocol("frame produced no bands".to_owned()));
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::{App, Outcome, Resolver, State, Strategy};
    use crate::view::Action;

    fn action(name: &str) -> Action {
        Action::new(name)
    }

    #[test]
    fn default_state_matches_the_design() {
        let state = State::default();
        assert_eq!(state.strategy, Strategy::Records5);
        assert_eq!(state.resolver, Resolver::Cloudflare);
        assert!(state.minimize_to_tray);
        assert!(!state.system_proxy);
    }

    #[test]
    fn strategy_actions_round_trip() {
        for strategy in Strategy::all() {
            assert_eq!(Strategy::from_action(strategy.action()), Some(*strategy));
        }
    }

    #[test]
    fn resolver_actions_round_trip() {
        for resolver in Resolver::all() {
            assert_eq!(Resolver::from_action(resolver.action()), Some(*resolver));
        }
    }

    #[test]
    fn clicking_a_radio_selects_it() {
        let mut state = State::default();
        assert_eq!(
            state.apply(&action("strategy.records2")),
            Outcome::Changed
        );
        assert_eq!(state.strategy, Strategy::Records2);
    }

    #[test]
    fn clicking_a_checkbox_toggles_it() {
        let mut state = State::default();
        state.apply(&action("option.system_proxy"));
        assert!(state.system_proxy);
        state.apply(&action("option.system_proxy"));
        assert!(!state.system_proxy);
    }

    #[test]
    fn sections_collapse() {
        let mut state = State::default();
        state.apply(&action("section.dns"));
        assert!(!state.dns_open);
        // Collapsed sections hide their children: `View::actions` walks every
        // node including hidden ones, so the rendered layout is what matters.
        let visible = state.layout();
        assert!(visible
            .rows
            .iter()
            .any(|row| row.action.as_str() == "section.dns"));
        assert!(!visible
            .rows
            .iter()
            .any(|row| row.action.as_str().starts_with("dns.")));
    }

    #[test]
    fn unknown_actions_are_ignored() {
        let mut state = State::default();
        assert_eq!(state.apply(&action("nope")), Outcome::Ignored);
        assert_eq!(state.apply(&action("")), Outcome::Ignored);
    }

    #[test]
    fn close_action_is_reported() {
        let mut state = State::default();
        assert_eq!(state.apply(&action("action.close")), Outcome::Close);
    }

    #[test]
    fn clicking_hits_the_right_row() {
        let mut app = App::new(420, 560);
        let target = app
            .layout()
            .hits
            .iter()
            .find(|hit| hit.action.as_str() == "dns.google")
            .expect("google row")
            .rect;
        let outcome = app.click(
            target.x + i16::try_from(target.w / 2).unwrap_or(0),
            target.y + i16::try_from(target.h / 2).unwrap_or(0),
        );
        assert_eq!(outcome, Some(Outcome::Changed));
        assert_eq!(app.state.resolver, Resolver::Google);
    }

    #[test]
    fn clicking_empty_space_does_nothing() {
        let mut app = App::new(420, 560);
        assert_eq!(app.click(419, 2), None);
        assert_eq!(app.state, State::default());
    }

    #[test]
    fn frame_changes_when_state_changes() {
        let mut app = App::new(420, 560);
        let before = app.canvas();
        app.state.apply(&action("strategy.off"));
        let after = app.canvas();
        assert_ne!(before.pixels(), after.pixels());
    }

    #[test]
    fn resize_clamps_tiny_windows() {
        let mut app = App::new(420, 560);
        app.resize(10, 10);
        assert_eq!(app.size, (120, 120));
    }

    #[test]
    fn speed_button_label_reflects_measurement() {
        let mut state = State::default();
        assert_eq!(state.speed_label(), "Speed test");
        state.speed_mbps = Some(93.4);
        assert!(state.speed_label().contains("93.4"));
    }

    #[test]
    fn every_visible_action_is_handled() {
        // Guards against a button that looks interactive but does nothing.
        let state = State::default();
        for action in state.view().actions() {
            let mut probe = state.clone();
            let outcome = probe.apply(&action);
            assert_ne!(outcome, Outcome::Ignored, "unhandled action {action}");
        }
    }
}
