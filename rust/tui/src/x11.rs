//! X11 wire protocol client — just enough to open a window and blit pixels.
//!
//! The GUI must not depend on Xlib, GTK, or Wayland client libraries, so this
//! module speaks the X11 protocol directly over its Unix socket: setup
//! handshake, `InternAtom`, `CreateWindow`, `MapWindow`, `CreateGC`,
//! `PutImage`, and event decoding.
//!
//! Only the local Unix transport is implemented, and only Linux is wired up.
//! Windows and Android need their own surfaces and are not implemented yet.
//!
//! The parser is deliberately forgiving about fields it does not use and strict
//! about fields it does: a malformed setup reply is an error, never a guess.

#![forbid(unsafe_code)]

use std::io::{self, Read, Write};
use std::os::unix::net::UnixStream;
use std::path::{Path, PathBuf};

/// X11 protocol major version this client speaks.
pub const PROTOCOL_MAJOR: u16 = 11;
/// X11 protocol minor version this client speaks.
pub const PROTOCOL_MINOR: u16 = 0;

/// Core protocol opcodes.
pub mod opcode {
    /// `CreateWindow`.
    pub const CREATE_WINDOW: u8 = 1;
    /// `ChangeWindowAttributes`.
    pub const CHANGE_WINDOW_ATTRIBUTES: u8 = 2;
    /// `MapWindow`.
    pub const MAP_WINDOW: u8 = 8;
    /// `InternAtom`.
    pub const INTERN_ATOM: u8 = 16;
    /// `ChangeProperty`.
    pub const CHANGE_PROPERTY: u8 = 18;
    /// `CreateGC`.
    pub const CREATE_GC: u8 = 55;
    /// `PutImage`.
    pub const PUT_IMAGE: u8 = 72;
    /// `GetGeometry`.
    pub const GET_GEOMETRY: u8 = 14;
}

/// Core event codes, as they appear in the first byte of an event.
pub mod event {
    /// Key was pressed.
    pub const KEY_PRESS: u8 = 2;
    /// Key was released.
    pub const KEY_RELEASE: u8 = 3;
    /// Pointer button was pressed.
    pub const BUTTON_PRESS: u8 = 4;
    /// Pointer button was released.
    pub const BUTTON_RELEASE: u8 = 5;
    /// Pointer moved.
    pub const MOTION_NOTIFY: u8 = 6;
    /// Part of the window became visible.
    pub const EXPOSE: u8 = 12;
    /// Window was resized or moved.
    pub const CONFIGURE_NOTIFY: u8 = 22;
    /// Client message, e.g. from the window manager.
    pub const CLIENT_MESSAGE: u8 = 33;
    /// Window was destroyed.
    pub const DESTROY_NOTIFY: u8 = 17;
    /// Highest core event code.
    pub const LAST_CORE: u8 = 34;
}

/// `ChangeProperty` mode: replace whatever is there.
pub const PROP_MODE_REPLACE: u8 = 0;
/// `ChangeProperty` format: 8-bit data.
pub const PROP_FORMAT_8: u8 = 8;

/// `PutImage` format: ZPixmap.
pub const IMAGE_FORMAT_ZPIXMAP: u8 = 2;

/// `CreateWindow` class: InputOutput.
pub const WINDOW_CLASS_INPUT_OUTPUT: u16 = 1;

/// Event mask bits.
pub mod mask {
    /// Key press.
    pub const KEY_PRESS: u32 = 0x0000_0001;
    /// Key release.
    pub const KEY_RELEASE: u32 = 0x0000_0002;
    /// Button press.
    pub const BUTTON_PRESS: u32 = 0x0000_0004;
    /// Button release.
    pub const BUTTON_RELEASE: u32 = 0x0000_0008;
    /// Pointer motion.
    pub const POINTER_MOTION: u32 = 0x0000_0040;
    /// Enter and leave.
    pub const ENTER_LEAVE: u32 = 0x0000_0010;
    /// `Expose`.
    pub const EXPOSURE: u32 = 0x0000_8000;
    /// Structure changes, including `ConfigureNotify`.
    pub const STRUCTURE: u32 = 0x0001_0000;
}

/// Errors from opening or driving a display.
#[derive(Debug)]
pub enum Error {
    /// `DISPLAY` is not set.
    NoDisplay,
    /// The display string could not be parsed or names a remote server.
    BadDisplay(String),
    /// No X socket at any of the usual paths.
    NotRunning(PathBuf),
    /// Socket-level failure.
    Io(io::Error),
    /// The server rejected the connection.
    Refused(String),
    /// The server sent something this client cannot interpret.
    Protocol(String),
    /// The server reported an X error.
    ServerError {
        /// X error code.
        code: u8,
        /// Sequence number of the failing request.
        sequence: u16,
    },
}

impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::NoDisplay => write!(f, "DISPLAY is not set"),
            Self::BadDisplay(display) => write!(f, "cannot use DISPLAY={display:?}"),
            Self::NotRunning(path) => write!(f, "no X server listening at {}", path.display()),
            Self::Io(err) => write!(f, "X11 socket error: {err}"),
            Self::Refused(reason) => write!(f, "X server refused the connection: {reason}"),
            Self::Protocol(reason) => write!(f, "X11 protocol error: {reason}"),
            Self::ServerError { code, sequence } => {
                write!(f, "X server error {code} on request {sequence}")
            }
        }
    }
}

impl std::error::Error for Error {}

impl From<io::Error> for Error {
    fn from(err: io::Error) -> Self {
        Self::Io(err)
    }
}

/// A parsed `DISPLAY` value.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DisplayName {
    /// Display number (`:0` → `0`).
    pub display: u16,
    /// Screen number, the part after the dot.
    pub screen: u16,
}

impl DisplayName {
    /// Parses `DISPLAY` from the environment.
    ///
    /// # Errors
    ///
    /// Returns [`Error::NoDisplay`] when the variable is unset.
    pub fn from_env() -> Result<Self, Error> {
        let value = std::env::var("DISPLAY").map_err(|_| Error::NoDisplay)?;
        Self::parse(&value)
    }

    /// Parses a local `DISPLAY` string such as `:0`, `:0.0`, or `unix:1`.
    ///
    /// Remote (TCP) displays are rejected: this client only speaks the local
    /// transport, and silently pretending otherwise would be worse than failing.
    ///
    /// # Errors
    ///
    /// Returns [`Error::BadDisplay`] for anything that is not a local display.
    pub fn parse(value: &str) -> Result<Self, Error> {
        let trimmed = value.trim();
        let rest = trimmed
            .strip_prefix("unix:")
            .or_else(|| trimmed.strip_prefix(':'))
            .ok_or_else(|| Error::BadDisplay(value.to_owned()))?;
        let (display, screen) = rest.split_once('.').unwrap_or((rest, "0"));
        if display.is_empty() || screen.is_empty() {
            return Err(Error::BadDisplay(value.to_owned()));
        }
        let (Ok(display), Ok(screen)) = (display.parse::<u16>(), screen.parse::<u16>()) else {
            return Err(Error::BadDisplay(value.to_owned()));
        };
        Ok(Self { display, screen })
    }

    /// Socket paths to try, most likely first.
    #[must_use]
    pub fn socket_paths(&self) -> Vec<PathBuf> {
        let name = format!("X{}", self.display);
        let mut paths = Vec::with_capacity(2);
        if let Some(dir) = std::env::var_os("XDG_RUNTIME_DIR") {
            paths.push(Path::new(&dir).join(&name));
        }
        paths.push(Path::new("/tmp/.X11-unix").join(&name));
        paths
    }

    /// Connects to the display and completes the setup handshake.
    ///
    /// # Errors
    ///
    /// Fails when no socket answers, the server refuses the connection, or the
    /// setup reply is malformed.
    pub fn connect(&self) -> Result<Connection, Error> {
        let paths = self.socket_paths();
        let mut stream = None;
        let mut last = None;
        for path in &paths {
            match UnixStream::connect(path) {
                Ok(sock) => {
                    stream = Some(sock);
                    break;
                }
                Err(err) => last = Some((path.clone(), err)),
            }
        }
        let Some(stream) = stream else {
            let path = last.map_or_else(|| paths[0].clone(), |(path, _)| path);
            return Err(Error::NotRunning(path));
        };
        let mut connection = Connection::from_stream(stream, self.screen);
        connection.handshake()?;
        Ok(connection)
    }
}

/// One screen's geometry, from the setup reply.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Screen {
    /// Root window id.
    pub root: u32,
    /// Root window width in pixels.
    pub width: u16,
    /// Root window height in pixels.
    pub height: u16,
    /// Root visual id.
    pub root_visual: u32,
    /// Root depth in bits per pixel.
    pub root_depth: u8,
    /// Red channel mask of the root visual.
    pub red_mask: u32,
    /// Green channel mask of the root visual.
    pub green_mask: u32,
    /// Blue channel mask of the root visual.
    pub blue_mask: u32,
}

/// A connected display.
#[derive(Debug)]
pub struct Connection {
    stream: UnixStream,
    resource_base: u32,
    resource_mask: u32,
    next_serial: u32,
    sequence: u16,
    screen_index: u16,
    screen: Option<Screen>,
    atoms: Vec<(String, u32)>,
    /// Largest request the server accepts, in 4-byte words.
    max_request_words: u16,
}

/// Little-endian readers over a byte slice, returning `None` when short.
mod read {
    pub(super) fn u8_at(data: &[u8], offset: usize) -> Option<u8> {
        data.get(offset).copied()
    }

    pub(super) fn u16_at(data: &[u8], offset: usize) -> Option<u16> {
        let bytes = data.get(offset..offset + 2)?;
        Some(u16::from_le_bytes([bytes[0], bytes[1]]))
    }

    pub(super) fn u32_at(data: &[u8], offset: usize) -> Option<u32> {
        let bytes = data.get(offset..offset + 4)?;
        Some(u32::from_le_bytes([bytes[0], bytes[1], bytes[2], bytes[3]]))
    }
}

impl Connection {
    /// Wraps an already-connected socket.
    ///
    /// Used by [`DisplayName::connect`] after it finds a socket, and by tests
    /// that drive the protocol over a socket pair.
    fn from_stream(stream: UnixStream, screen_index: u16) -> Self {
        Self {
            stream,
            resource_base: 0,
            resource_mask: 0x001f_ffff,
            next_serial: 0,
            sequence: 0,
            screen_index,
            screen: None,
            atoms: Vec::new(),
            max_request_words: 8192,
        }
    }

    /// Performs the setup handshake and parses the reply.
    fn handshake(&mut self) -> Result<(), Error> {
        let mut request = Vec::with_capacity(12);
        request.push(0x6c); // 'l': little-endian client
        request.push(0);
        request.extend_from_slice(&PROTOCOL_MAJOR.to_le_bytes());
        request.extend_from_slice(&PROTOCOL_MINOR.to_le_bytes());
        request.extend_from_slice(&0u16.to_le_bytes()); // authorisation name length
        request.extend_from_slice(&0u16.to_le_bytes()); // authorisation data length
        request.extend_from_slice(&0u16.to_le_bytes()); // padding
        self.stream.write_all(&request)?;
        self.stream.flush()?;

        let mut head = [0u8; 8];
        self.stream.read_exact(&mut head)?;
        let status = head[0];
        let extra_words = u16::from_le_bytes([head[6], head[7]]);
        let mut body = vec![0u8; usize::from(extra_words) * 4];
        self.stream.read_exact(&mut body)?;

        match status {
            1 => self.parse_setup(&body),
            0 => {
                // Failure reply: header byte 1 is the reason length and the
                // reason itself starts the body.
                let length = usize::from(head[1]);
                let reason = body
                    .get(..length)
                    .map(|bytes| String::from_utf8_lossy(bytes).into_owned())
                    .unwrap_or_default();
                Err(Error::Refused(reason))
            }
            other => Err(Error::Protocol(format!("unexpected setup status {other}"))),
        }
    }

    /// Parses a successful setup reply.
    fn parse_setup(&mut self, body: &[u8]) -> Result<(), Error> {
        let truncated = || Error::Protocol("truncated setup reply".to_owned());
        self.resource_base = read::u32_at(body, 4).ok_or_else(truncated)?;
        self.resource_mask = read::u32_at(body, 8).ok_or_else(truncated)?;
        let vendor_len = usize::from(read::u16_at(body, 16).ok_or_else(truncated)?);
        self.max_request_words = read::u16_at(body, 18).ok_or_else(truncated)?.max(4096);
        let screen_count = usize::from(read::u8_at(body, 20).ok_or_else(truncated)?);
        let format_count = usize::from(read::u8_at(body, 21).ok_or_else(truncated)?);

        let mut offset = 32 + vendor_len.div_ceil(4) * 4;
        offset += format_count * 8;
        if self.screen_index >= u16::try_from(screen_count).unwrap_or(0) {
            return Err(Error::Protocol(format!(
                "screen {} requested, server has {screen_count}",
                self.screen_index
            )));
        }
        for index in 0..screen_count {
            let root = read::u32_at(body, offset).ok_or_else(truncated)?;
            let width = read::u16_at(body, offset + 20).ok_or_else(truncated)?;
            let height = read::u16_at(body, offset + 22).ok_or_else(truncated)?;
            let root_visual = read::u32_at(body, offset + 32).ok_or_else(truncated)?;
            let root_depth = read::u8_at(body, offset + 38).ok_or_else(truncated)?;
            let depth_count = usize::from(read::u8_at(body, offset + 39).ok_or_else(truncated)?);
            let mut depth_offset = offset + 40;

            // Walk the depth list looking for the root visual's masks.
            let (mut red_mask, mut green_mask, mut blue_mask) = (0u32, 0u32, 0u32);
            for _ in 0..depth_count {
                let depth = read::u8_at(body, depth_offset).ok_or_else(truncated)?;
                let visuals = usize::from(read::u16_at(body, depth_offset + 2).ok_or_else(truncated)?);
                let mut visual_offset = depth_offset + 8;
                for _ in 0..visuals {
                    let visual_id = read::u32_at(body, visual_offset).ok_or_else(truncated)?;
                    if visual_id == root_visual && depth == root_depth {
                        red_mask = read::u32_at(body, visual_offset + 8).ok_or_else(truncated)?;
                        green_mask = read::u32_at(body, visual_offset + 12).ok_or_else(truncated)?;
                        blue_mask = read::u32_at(body, visual_offset + 16).ok_or_else(truncated)?;
                    }
                    visual_offset += 24;
                }
                depth_offset = visual_offset;
            }
            if index == usize::from(self.screen_index) {
                self.screen = Some(Screen {
                    root,
                    width,
                    height,
                    root_visual,
                    root_depth,
                    red_mask,
                    green_mask,
                    blue_mask,
                });
                break;
            }
            // Skip to the next screen: depth list length is only known by walking
            // it, which the loop above already did for the wanted screen, so for
            // skipped screens walk cheaply here.
            offset = skip_screen_depths(body, offset + 40, depth_count).ok_or_else(truncated)?;
        }
        if self.screen.is_none() {
            return Err(Error::Protocol("screen missing from setup reply".to_owned()));
        }
        Ok(())
    }

    /// Screen information from the setup reply.
    ///
    /// # Errors
    ///
    /// Returns [`Error::Protocol`] if called before a successful handshake.
    pub fn screen(&self) -> Result<Screen, Error> {
        self.screen
            .ok_or_else(|| Error::Protocol("not connected".to_owned()))
    }

    /// Sequence number of the next request.
    #[must_use]
    pub const fn sequence(&self) -> u16 {
        self.sequence
    }

    /// Allocates a resource id.
    fn new_id(&mut self) -> u32 {
        let mask = self.resource_mask.max(0x001f_ffff);
        self.next_serial = self.next_serial.wrapping_add(1);
        self.resource_base | (self.next_serial & mask)
    }

    /// Writes one request whose header fields are supplied by the caller.
    fn send(&mut self, opcode: u8, data: u8, body: &[u8]) -> Result<(), Error> {
        let words = (1 + body.len()).div_ceil(4);
        let mut request = Vec::with_capacity(words * 4);
        request.push(opcode);
        request.push(data);
        request.extend_from_slice(&u16::try_from(words).unwrap_or(u16::MAX).to_le_bytes());
        request.extend_from_slice(body);
        request.resize(words * 4, 0);
        self.stream.write_all(&request)?;
        self.stream.flush()?;
        self.sequence = self.sequence.wrapping_add(1);
        Ok(())
    }

    /// Reads a reply, skipping events that arrive first.
    fn reply(&mut self) -> Result<Vec<u8>, Error> {
        loop {
            let mut head = [0u8; 32];
            self.stream.read_exact(&mut head)?;
            match head[0] {
                0 => {
                    return Err(Error::ServerError {
                        code: head[1],
                        sequence: u16::from_le_bytes([head[2], head[3]]),
                    });
                }
                1 => {
                    let length = read::u32_at(&head, 4).unwrap_or(0);
                    let mut reply = head.to_vec();
                    reply.resize(32 + usize::try_from(length).unwrap_or(0) * 4, 0);
                    self.stream.read_exact(&mut reply[32..])?;
                    return Ok(reply);
                }
                _ => continue, // an event; the caller will see it later
            }
        }
    }

    /// Interns an atom, returning its id.
    ///
    /// # Errors
    ///
    /// Propagates socket and protocol errors.
    pub fn intern_atom(&mut self, name: &str, only_if_exists: bool) -> Result<u32, Error> {
        if let Some((_, atom)) = self.atoms.iter().find(|(known, _)| known == name) {
            return Ok(*atom);
        }
        // InternAtom has a 6-byte header: opcode, only-if-exists, request length,
        // then the name length and two unused bytes. `send` writes a 4-byte
        // header, so this request is assembled and written here.
        let name_len = u16::try_from(name.len()).unwrap_or(0);
        let words = 2u16.saturating_add(name_len.div_ceil(4));
        let mut request = Vec::with_capacity(usize::from(words) * 4);
        request.push(opcode::INTERN_ATOM);
        request.push(u8::from(only_if_exists));
        request.extend_from_slice(&words.to_le_bytes());
        request.extend_from_slice(&name_len.to_le_bytes());
        request.extend_from_slice(&[0, 0]);
        request.extend_from_slice(name.as_bytes());
        request.resize(usize::from(words) * 4, 0);
        self.stream.write_all(&request)?;
        self.stream.flush()?;
        self.sequence = self.sequence.wrapping_add(1);

        let reply = self.reply()?;
        let atom = read::u32_at(&reply, 8)
            .ok_or_else(|| Error::Protocol("short InternAtom reply".to_owned()))?;
        if atom != 0 {
            self.atoms.push((name.to_owned(), atom));
        }
        Ok(atom)
    }

    /// Sets a string property on a window.
    ///
    /// # Errors
    ///
    /// Propagates socket and protocol errors.
    pub fn set_string_property(
        &mut self,
        window: u32,
        property: u32,
        kind: u32,
        value: &str,
    ) -> Result<(), Error> {
        let bytes = value.as_bytes();
        let mut body = Vec::with_capacity(16 + bytes.len());
        body.push(PROP_MODE_REPLACE);
        body.extend_from_slice(&[0, 0]);
        body.extend_from_slice(&window.to_le_bytes());
        body.extend_from_slice(&property.to_le_bytes());
        body.extend_from_slice(&kind.to_le_bytes());
        body.push(PROP_FORMAT_8);
        body.extend_from_slice(&[0, 0, 0]);
        body.extend_from_slice(&u32::try_from(bytes.len()).unwrap_or(0).to_le_bytes());
        body.extend_from_slice(bytes);
        // `ChangeProperty` writes data units, so pad to 4 bytes; `send` does that.
        self.send(opcode::CHANGE_PROPERTY, 0, &body)
    }

    /// Creates a top-level window sized for the screen's depth.
    ///
    /// # Errors
    ///
    /// Propagates socket and protocol errors.
    pub fn create_window(&mut self, width: u16, height: u16) -> Result<Window, Error> {
        let screen = self.screen()?;
        let id = self.new_id();
        let mut body = Vec::with_capacity(32);
        body.push(screen.root_depth); // depth
        body.extend_from_slice(&[0, 0, 0]);
        body.extend_from_slice(&id.to_le_bytes());
        body.extend_from_slice(&screen.root.to_le_bytes()); // parent
        body.extend_from_slice(&0i16.to_le_bytes()); // x
        body.extend_from_slice(&0i16.to_le_bytes()); // y
        body.extend_from_slice(&width.to_le_bytes());
        body.extend_from_slice(&height.to_le_bytes());
        body.extend_from_slice(&0u16.to_le_bytes()); // border width
        body.extend_from_slice(&WINDOW_CLASS_INPUT_OUTPUT.to_le_bytes());
        body.extend_from_slice(&screen.root_visual.to_le_bytes());
        // Value mask: background pixel + event mask.
        body.extend_from_slice(&(0x0000_0002u32 | 0x0000_0800).to_le_bytes());
        body.extend_from_slice(&0x0012_1c30u32.to_le_bytes()); // dark background
        body.extend_from_slice(
            &(mask::EXPOSURE | mask::STRUCTURE | mask::KEY_PRESS | mask::KEY_RELEASE
                | mask::BUTTON_PRESS | mask::BUTTON_RELEASE | mask::POINTER_MOTION
                | mask::ENTER_LEAVE)
                .to_le_bytes(),
        );
        self.send(opcode::CREATE_WINDOW, 0, &body)?;
        Ok(Window {
            id,
            width,
            height,
        })
    }

    /// Sets the window title through `WM_NAME` and `_NET_WM_NAME`.
    ///
    /// # Errors
    ///
    /// Propagates socket and protocol errors.
    pub fn set_title(&mut self, window: &Window, title: &str) -> Result<(), Error> {
        let wm_name = self.intern_atom("WM_NAME", false)?;
        let string = self.intern_atom("STRING", false)?;
        self.set_string_property(window.id, wm_name, string, title)?;
        let net_wm_name = self.intern_atom("_NET_WM_NAME", false)?;
        let utf8 = self.intern_atom("UTF8_STRING", false)?;
        self.set_string_property(window.id, net_wm_name, utf8, title)
    }

    /// Creates a graphics context for the window.
    ///
    /// # Errors
    ///
    /// Propagates socket and protocol errors.
    pub fn create_gc(&mut self, window: &Window) -> Result<u32, Error> {
        let id = self.new_id();
        let mut body = Vec::with_capacity(8);
        body.extend_from_slice(&id.to_le_bytes());
        body.extend_from_slice(&window.id.to_le_bytes());
        body.extend_from_slice(&0u32.to_le_bytes()); // no attributes
        self.send(opcode::CREATE_GC, 0, &body)?;
        Ok(id)
    }

    /// Maps the window, making it visible.
    ///
    /// # Errors
    ///
    /// Propagates socket and protocol errors.
    pub fn map_window(&mut self, window: &Window) -> Result<(), Error> {
        self.send(opcode::MAP_WINDOW, 0, &window.id.to_le_bytes())
    }

    /// Blits an RGBA buffer into the window with `PutImage`.
    ///
    /// The buffer must be `width * height * 4` bytes. X11 in 24/32-bit
    /// true-colour wants BGRX byte order, so channels are swapped on the way out;
    /// the alpha byte is dropped because the window is opaque.
    ///
    /// A single request is limited to 16-bit length in 4-byte words, so a full
    /// 420x560 frame (about 940 KB) cannot travel as one `PutImage`. The frame is
    /// therefore split into horizontal bands, each its own `PutImage` at the right
    /// destination offset. Returns the number of bands sent.
    ///
    /// # Errors
    ///
    /// Returns [`Error::Protocol`] when the buffer size does not match.
    pub fn put_image(
        &mut self,
        window: &Window,
        gc: u32,
        width: u16,
        height: u16,
        rgba: &[u8],
    ) -> Result<usize, Error> {
        let row_bytes = usize::from(width) * 4;
        let expected = row_bytes * usize::from(height);
        if rgba.len() != expected {
            return Err(Error::Protocol(format!(
                "PutImage wants {expected} bytes, got {}",
                rgba.len()
            )));
        }
        let max_bytes = usize::from(self.max_request_words) * 4;
        // 24 header bytes plus the pixel rows of one band.
        let budget = max_bytes.saturating_sub(32).max(row_bytes);
        let rows_per_band = usize::from(height).min((budget / row_bytes.max(1)).max(1));
        let mut bands = 0usize;
        let mut row = 0usize;
        while row < usize::from(height) {
            let rows = rows_per_band.min(usize::from(height) - row);
            let slice = &rgba[row * row_bytes..(row + rows) * row_bytes];
            self.put_image_band(window, gc, width, rows as u16, row as i16, slice)?;
            bands += 1;
            row += rows;
        }
        Ok(bands)
    }

    /// Sends one horizontal band of pixels.
    fn put_image_band(
        &mut self,
        window: &Window,
        gc: u32,
        width: u16,
        height: u16,
        dst_y: i16,
        rgba: &[u8],
    ) -> Result<(), Error> {
        let screen = self.screen()?;
        let mut body = Vec::with_capacity(20 + rgba.len());
        body.extend_from_slice(&window.id.to_le_bytes());
        body.extend_from_slice(&gc.to_le_bytes());
        body.extend_from_slice(&width.to_le_bytes());
        body.extend_from_slice(&height.to_le_bytes());
        body.extend_from_slice(&0i16.to_le_bytes()); // dst-x
        body.extend_from_slice(&dst_y.to_le_bytes()); // dst-y
        body.push(0); // left pad
        body.push(screen.root_depth);
        body.extend_from_slice(&[0, 0]);
        for pixel in rgba.chunks_exact(4) {
            body.push(pixel[2]);
            body.push(pixel[1]);
            body.push(pixel[0]);
            body.push(0);
        }
        self.send(opcode::PUT_IMAGE, IMAGE_FORMAT_ZPIXMAP, &body)
    }
}

/// Skips the depth block of a screen entry, returning the offset just past it.
fn skip_screen_depths(body: &[u8], start: usize, depth_count: usize) -> Option<usize> {
    let mut offset = start;
    for _ in 0..depth_count {
        let visuals = usize::from(read::u16_at(body, offset + 2)?);
        offset += 8 + visuals * 24;
    }
    Some(offset)
}

/// An X window handle.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Window {
    /// X resource id.
    pub id: u32,
    /// Width in pixels.
    pub width: u16,
    /// Height in pixels.
    pub height: u16,
}

/// A decoded core event.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Event {
    /// The window needs repainting.
    Expose,
    /// The window changed size.
    Resized {
        /// New width.
        width: u16,
        /// New height.
        height: u16,
    },
    /// Pointer button went down at window coordinates.
    ButtonPress {
        /// X coordinate.
        x: i16,
        /// Y coordinate.
        y: i16,
    },
    /// Pointer button went up.
    ButtonRelease {
        /// X coordinate.
        x: i16,
        /// Y coordinate.
        y: i16,
    },
    /// Pointer moved.
    Motion {
        /// X coordinate.
        x: i16,
        /// Y coordinate.
        y: i16,
    },
    /// Key was pressed; `keycode` is the server-side code.
    KeyPress {
        /// X keycode.
        keycode: u8,
    },
    /// The window manager asked the window to close.
    CloseRequested,
    /// Anything this client does not model.
    Other,
}

impl Event {
    /// Decodes one 32-byte core event.
    ///
    /// # Errors
    ///
    /// Returns [`Error::Protocol`] when the buffer is shorter than 32 bytes.
    pub fn decode(raw: &[u8]) -> Result<Self, Error> {
        if raw.len() < 32 {
            return Err(Error::Protocol("short event".to_owned()));
        }
        let code = raw[0] & 0x7f;
        let x = read::u16_at(raw, 24).unwrap_or(0) as i16;
        let y = read::u16_at(raw, 26).unwrap_or(0) as i16;
        Ok(match code {
            event::EXPOSE => Self::Expose,
            event::CONFIGURE_NOTIFY => Self::Resized {
                width: read::u16_at(raw, 20).unwrap_or(0),
                height: read::u16_at(raw, 22).unwrap_or(0),
            },
            event::BUTTON_PRESS => Self::ButtonPress { x, y },
            event::BUTTON_RELEASE => Self::ButtonRelease { x, y },
            event::MOTION_NOTIFY => Self::Motion { x, y },
            event::KEY_PRESS => Self::KeyPress { keycode: raw[1] },
            event::KEY_RELEASE => Self::Other,
            event::CLIENT_MESSAGE => Self::CloseRequested,
            _ => Self::Other,
        })
    }
}

impl Connection {
    /// Blocks for one event.
    ///
    /// # Errors
    ///
    /// Propagates socket errors and X error replies.
    pub fn wait_event(&mut self) -> Result<Event, Error> {
        let mut raw = [0u8; 32];
        self.stream.read_exact(&mut raw)?;
        if raw[0] == 0 {
            return Err(Error::ServerError {
                code: raw[1],
                sequence: u16::from_le_bytes([raw[2], raw[3]]),
            });
        }
        Event::decode(&raw)
    }
}

#[cfg(test)]
mod tests {
    use super::{opcode, Connection, DisplayName, Error, Event, Screen};
    use std::io::{Read, Write};
    use std::os::unix::net::UnixStream;

    /// Builds a setup reply body the way a real server lays it out.
    fn setup_body(width: u16, height: u16) -> Vec<u8> {
        let mut body = Vec::new();
        body.extend_from_slice(&[0u8; 4]); // release number
        body.extend_from_slice(&0x0040_0000u32.to_le_bytes()); // resource id base
        body.extend_from_slice(&0x003f_ffffu32.to_le_bytes()); // resource id mask
        body.extend_from_slice(&0u32.to_le_bytes()); // motion buffer
        body.extend_from_slice(&0u16.to_le_bytes()); // vendor length
        body.extend_from_slice(&8u16.to_le_bytes()); // max request length, words
        body.push(1); // one screen
        body.push(1); // one pixmap format
        body.push(0); // image byte order: LSB first
        body.push(0); // bitmap bit order
        body.push(32); // scanline unit
        body.push(32); // scanline pad
        body.push(8); // min keycode
        body.push(255); // max keycode
        body.extend_from_slice(&[0u8; 4]); // unused
        assert_eq!(body.len(), 32, "fixed setup part is 32 bytes");

        // Pixmap format: depth 24, ZPixmap, 32 bits per pixel.
        body.extend_from_slice(&[24, 32, 0, 0, 0, 0, 0, 0]);

        // One screen.
        body.extend_from_slice(&0x100u32.to_le_bytes()); // root window
        body.extend_from_slice(&0u32.to_le_bytes()); // default colormap
        body.extend_from_slice(&0x00ff_ffffu32.to_le_bytes()); // white pixel
        body.extend_from_slice(&0u32.to_le_bytes()); // black pixel
        body.extend_from_slice(&0u32.to_le_bytes()); // input masks
        body.extend_from_slice(&width.to_le_bytes());
        body.extend_from_slice(&height.to_le_bytes());
        body.extend_from_slice(&508u16.to_le_bytes());
        body.extend_from_slice(&285u16.to_le_bytes());
        body.extend_from_slice(&1u16.to_le_bytes());
        body.extend_from_slice(&1u16.to_le_bytes());
        body.extend_from_slice(&0x21u32.to_le_bytes()); // root visual
        body.push(0); // backing stores
        body.push(0); // save unders
        body.push(24); // root depth
        body.push(1); // one depth
        assert_eq!(body.len(), 80, "setup with one screen is 80 bytes");

        // Depth 24 with one TrueColor visual.
        body.push(24);
        body.push(0);
        body.extend_from_slice(&1u16.to_le_bytes()); // one visual
        body.extend_from_slice(&[0u8; 4]);
        body.extend_from_slice(&0x21u32.to_le_bytes()); // visual id
        body.push(4); // TrueColor
        body.push(8); // bits per rgb
        body.extend_from_slice(&256u16.to_le_bytes());
        body.extend_from_slice(&0x00ff_0000u32.to_le_bytes());
        body.extend_from_slice(&0x0000_ff00u32.to_le_bytes());
        body.extend_from_slice(&0x0000_00ffu32.to_le_bytes());
        body.extend_from_slice(&0u32.to_le_bytes());
        body
    }

    /// Writes a setup reply with the 8-byte header.
    fn write_setup(stream: &mut UnixStream, width: u16, height: u16) {
        let body = setup_body(width, height);
        let mut reply = vec![1u8, 0];
        reply.extend_from_slice(&11u16.to_le_bytes());
        reply.extend_from_slice(&0u16.to_le_bytes());
        reply.extend_from_slice(
            &u16::try_from(body.len() / 4)
                .expect("body words")
                .to_le_bytes(),
        );
        reply.extend_from_slice(&body);
        stream.write_all(&reply).expect("setup reply");
        stream.flush().expect("flush");
    }

    /// Reads one request header, returning opcode and the whole request.
    fn read_request(stream: &mut UnixStream) -> (u8, Vec<u8>) {
        let mut head = [0u8; 4];
        stream.read_exact(&mut head).expect("request header");
        let words = u16::from_le_bytes([head[2], head[3]]);
        let mut rest = vec![0u8; usize::from(words) * 4 - 4];
        stream.read_exact(&mut rest).expect("request body");
        (head[0], rest)
    }

    #[test]
    fn handshake_parses_screen_and_requests() {
        let (mut server, client) = UnixStream::pair().expect("pair");
        let worker = std::thread::spawn(move || {
            let mut setup = [0u8; 12];
            server.read_exact(&mut setup).expect("setup request");
            assert_eq!(setup[0], 0x6c, "client must announce little-endian");
            assert_eq!(
                u16::from_le_bytes([setup[2], setup[3]]),
                super::PROTOCOL_MAJOR
            );
            write_setup(&mut server, 1920, 1080);

            // InternAtom("WM_NAME") → reply with atom 0x123.
            let (code, body) = read_request(&mut server);
            assert_eq!(code, opcode::INTERN_ATOM);
            // `read_request` strips the first 4 bytes, so the remaining header
            // is the name length, two pad bytes, then the name.
            let name_len = usize::from(u16::from_le_bytes([body[0], body[1]]));
            assert_eq!(&body[4..4 + name_len], b"WM_NAME");

            let mut reply = vec![1u8, 0];
            reply.extend_from_slice(&[0u8; 6]);
            reply.extend_from_slice(&[0u8; 4]); // field after length
            reply.resize(32, 0);
            reply[8..12].copy_from_slice(&0x123u32.to_le_bytes());
            server.write_all(&reply).expect("atom reply");
            server.flush().expect("flush");

            // CreateWindow: record the geometry it was asked for. Its header is
            // 8 bytes and `read_request` returned everything after the first 4,
            // so `body` still holds the 4-byte depth/pad/length tail before the
            // window id.
            let (code, body) = read_request(&mut server);
            assert_eq!(code, opcode::CREATE_WINDOW);
            assert_eq!(body[0], 24, "depth should match the root window");
            let width = u16::from_le_bytes([body[16], body[17]]);
            let height = u16::from_le_bytes([body[18], body[19]]);
            (width, height)
        });

        let mut connection = Connection::from_stream(client, 0);
        connection.handshake().expect("handshake");

        let screen = connection.screen().expect("screen");
        assert_eq!(screen.root, 0x100);
        assert_eq!((screen.width, screen.height), (1920, 1080));
        assert_eq!(screen.root_depth, 24);
        assert_eq!(screen.red_mask, 0x00ff_0000);
        assert_eq!(screen.blue_mask, 0x0000_00ff);

        assert!(matches!(connection.intern_atom("WM_NAME", false), Ok(0x123)));
        // Second call is served from the cache: sending it would deadlock the
        // fake server, which only answers one InternAtom.
        assert!(matches!(connection.intern_atom("WM_NAME", false), Ok(0x123)));

        let window = connection.create_window(420, 560).expect("create window");
        assert_eq!((window.width, window.height), (420, 560));
        let seen = worker.join().expect("server thread");
        assert_eq!(seen, (420, 560));
    }

    #[test]
    fn refuses_unreachable_display() {
        let display = DisplayName {
            display: 999,
            screen: 0,
        };
        let result = display.connect();
        assert!(matches!(result, Err(Error::NotRunning(_))), "{result:?}");
    }

    #[test]
    fn server_rejection_is_reported() {
        let (mut server, client) = UnixStream::pair().expect("pair");
        let worker = std::thread::spawn(move || {
            let mut setup = [0u8; 12];
            server.read_exact(&mut setup).expect("setup request");
            // Failed setup reply: status 0, reason length in byte 1.
            let mut reply = vec![0u8, 4];
            reply.extend_from_slice(&11u16.to_le_bytes());
            reply.extend_from_slice(&0u16.to_le_bytes());
            reply.extend_from_slice(&1u16.to_le_bytes()); // one extra word
            reply.extend_from_slice(b"nope");
            server.write_all(&reply).expect("refusal");
            server.flush().expect("flush");
        });
        let mut connection = Connection::from_stream(client, 0);
        let result = connection.handshake();
        worker.join().expect("server thread");
        match result {
            Err(Error::Refused(reason)) => assert_eq!(reason, "nope"),
            other => panic!("expected refusal, got {other:?}"),
        }
    }

    #[test]
    fn too_few_screens_is_an_error() {
        let (mut server, client) = UnixStream::pair().expect("pair");
        let worker = std::thread::spawn(move || {
            let mut setup = [0u8; 12];
            server.read_exact(&mut setup).expect("setup request");
            write_setup(&mut server, 800, 600);
            // Drain whatever the client writes before giving up.
            let mut sink = Vec::new();
            let _ = server.read_to_end(&mut sink);
        });
        let mut connection = Connection::from_stream(client, 3);
        let result = connection.handshake();
        drop(connection);
        worker.join().expect("server thread");
        assert!(matches!(result, Err(Error::Protocol(_))), "{result:?}");
    }

    #[test]
    fn put_image_rejects_wrong_buffer_size() {
        let (mut server, client) = UnixStream::pair().expect("pair");
        let worker = std::thread::spawn(move || {
            let mut setup = [0u8; 12];
            server.read_exact(&mut setup).expect("setup request");
            write_setup(&mut server, 800, 600);
            let mut sink = Vec::new();
            let _ = server.read_to_end(&mut sink);
        });
        let mut connection = Connection::from_stream(client, 0);
        connection.handshake().expect("handshake");
        let window = super::Window {
            id: 0x200,
            width: 4,
            height: 4,
        };
        let result = connection.put_image(&window, 0x201, 4, 4, &[0u8; 8]);
        drop(connection);
        worker.join().expect("server thread");
        assert!(matches!(result, Err(Error::Protocol(_))), "{result:?}");
    }

    #[test]
    fn parses_bare_display() {
        let d = DisplayName::parse(":0").expect("display");
        assert_eq!((d.display, d.screen), (0, 0));
    }

    #[test]
    fn parses_display_with_screen() {
        let d = DisplayName::parse(":12.1").expect("display");
        assert_eq!((d.display, d.screen), (12, 1));
    }

    #[test]
    fn parses_explicit_unix_prefix() {
        let d = DisplayName::parse("unix:7").expect("display");
        assert_eq!(d.display, 7);
    }

    #[test]
    fn rejects_remote_and_garbage() {
        for value in ["localhost:0", "10.0.0.2:0.0", "", ":", ":.", ":a", ":0.b"] {
            assert!(DisplayName::parse(value).is_err(), "{value:?} parsed");
        }
    }

    #[test]
    fn socket_paths_point_at_the_display() {
        let d = DisplayName::parse(":3").expect("display");
        assert!(d.socket_paths().iter().all(|p| p.ends_with("X3")));
    }

    #[test]
    fn decode_expose() {
        let mut raw = [0u8; 32];
        raw[0] = super::event::EXPOSE;
        assert!(matches!(Event::decode(&raw), Ok(Event::Expose)));
    }

    #[test]
    fn decode_button_press_coordinates() {
        let mut raw = [0u8; 32];
        raw[0] = super::event::BUTTON_PRESS;
        raw[24..26].copy_from_slice(&120u16.to_le_bytes());
        raw[26..28].copy_from_slice(&34u16.to_le_bytes());
        assert!(matches!(
            Event::decode(&raw),
            Ok(Event::ButtonPress { x: 120, y: 34 })
        ));
    }

    #[test]
    fn decode_configure_gives_new_size() {
        let mut raw = [0u8; 32];
        raw[0] = super::event::CONFIGURE_NOTIFY;
        raw[20..22].copy_from_slice(&640u16.to_le_bytes());
        raw[22..24].copy_from_slice(&480u16.to_le_bytes());
        assert!(matches!(
            Event::decode(&raw),
            Ok(Event::Resized {
                width: 640,
                height: 480
            })
        ));
    }

    #[test]
    fn decode_rejects_short_events() {
        assert!(Event::decode(&[0u8; 8]).is_err());
    }

    #[test]
    fn decode_unknown_event_is_other() {
        let mut raw = [0u8; 32];
        raw[0] = 90;
        assert!(matches!(Event::decode(&raw), Ok(Event::Other)));
    }

    #[test]
    fn empty_display_value_is_rejected() {
        assert!(matches!(
            DisplayName::parse("   "),
            Err(Error::BadDisplay(_))
        ));
    }

    #[test]
    fn screen_struct_is_copyable() {
        let s = Screen {
            root: 1,
            width: 2,
            height: 3,
            root_visual: 4,
            root_depth: 24,
            red_mask: 0x00ff_0000,
            green_mask: 0x0000_ff00,
            blue_mask: 0x0000_00ff,
        };
        let copy = s;
        assert_eq!(copy, s);
    }
}
