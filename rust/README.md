# Rust port

Cross-platform rewrite of Conduit. Linux is the first target; Windows and Android
follow.

## Status

| Piece | State |
|---|---|
| `dpi` — TLS ClientHello fragmentation | done, 11 tests |
| `allowlist` — host presets and custom-domain validation | done, 14 tests |
| `tui` — data-described GUI (view, layout, paint, X11) | done, 79 tests |
| `proxy` — CONNECT server | not started |
| `dns` — DoH, cache, ad blocking | not started |
| tray / Win32 / Android surfaces | not started |

Every crate is `#![forbid(unsafe_code)]`, has no dependencies, and shares the
workspace lint set: `clippy::all`/`pedantic`/`cargo` at `deny`, plus
`integer_arithmetic`, `indexing_slicing`, `unwrap_used`, `expect_used`, `panic`,
`todo`, `print_stdout`, and `allow_attributes_without_reason`.

Release profile: `lto = "fat"`, `codegen-units = 1`, `panic = "abort"`,
`strip = "symbols"`, `overflow-checks = false`.

## Building

`cargo` needs a linker and, on this class of image, two flags because the
prebuilt toolchain ships without `rust-lld`:

```bash
cargo build --offline --workspace
cargo test  --offline --workspace

# the window itself
cargo run --offline -p conduit-tui

# a frame without any display — useful in CI and on this image
cargo run --offline -p conduit-tui -- --dump-ppm frame.ppm
cargo run --offline -p conduit-tui -- --dump-ascii
cargo run --offline -p conduit-tui -- --size 800x600 --dump-ppm wide.ppm
```

`.cargo/config.toml` already carries `-C link-self-contained=no` and
`-C link-arg=-fuse-ld=bfd`.

## The GUI is a value, not a widget tree

`conduit-tui` inverts the usual toolkit design: there is no retained widget
graph, no callback registration, and no markup file. A screen is a plain Rust
value and everything else is derived from it.

```
State ──view()──▶ View ──layout()──▶ Layout ──paint()──▶ Canvas ──▶ X11 PutImage
  ▲                                    │
  └──────────── Action ◀──hit_test()───┘
```

| Module | Role |
|---|---|
| `view` | The data: `Node::{Column, Row, Panel, Label, Checkbox, Radio, Button, Separator, Section, Spacer, Fixed}`, `Theme`, `Rect`, `Action` |
| `layout` | Turns a `View` into flat `Region` rects, `Hit` rects, and focusable `Row`s — the single source of truth for both drawing and input |
| `paint` | Software rasteriser: rounded rects, hairlines, and glyphs into an RGBA `Canvas`; no image library |
| `font` | Bitmap font baked from DejaVu Sans Mono into `src/font.rs` (113 glyphs, 6x13) |
| `ascii` | Text backend, so the whole pipeline can be asserted without a display |
| `x11` | X11 wire-protocol client over the local Unix socket |
| `app` | State, action handling, and the event loop |

Input is matched against the same rect table that produced the pixels, so a
control cannot be drawn in one place and clickable in another. Collapsing a
section changes the layout, which changes both painting and hit-testing in the
same step.

### Why the wire protocol instead of Xlib

The image has `libX11.so.6` but no `libX11.so` development symlink and no
`pkg-config`, and there is no X server, Xvfb, or `apt` index. Linking Xlib would
need an `-l:libX11.so.6` hack and would still be untestable here. The protocol is
stable and documented, so the client speaks it directly: setup handshake,
`InternAtom`, `ChangeProperty` for the title, `CreateWindow`, `CreateGC`,
`MapWindow`, `PutImage`, and event decoding for `Expose`, resize, button, motion,
and `WM_DELETE_WINDOW`.

Two details worth knowing:

* A request length is 16 bits of 4-byte words, so one `PutImage` tops out near
  256 KB. A 420x560 RGBA frame is 940 KB and is therefore sent as horizontal
  bands (`put_image` returns how many).
* Pixels go out as BGRX (24-bit true colour, alpha dropped).

When there is no `DISPLAY`, the binary says so and points at `--dump-ppm` instead
of failing silently.

### Verifying without a display

`tools/fake_x_server.py` is a scripted X server: it answers the handshake,
records every request, replays `Expose` and a button press, validates that the
frame's bands tile the window exactly, then sends `WM_DELETE_WINDOW`.

```bash
python3 tools/fake_x_server.py &   # listens on /tmp/.X11-unix/X9
DISPLAY=:9 cargo run --offline -p conduit-tui
```

`rust/tui/docs/window.png` is a frame rendered by this code (420x560), and
`tests/x11_fake_server.rs` runs the whole X11 conversation as part of
`cargo test`. `paint::tests::write_frame` (`--ignored`) writes a real frame to
`/tmp/conduit_frame.ppm`.

### Regenerating the font

```bash
python3 tools/bake_font.py /usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf
```

The script parses the TTF, flattens quadratic contours, supersamples coverage
into 6x13 cells, and emits `src/font.rs`. Glyph output is embedded; the font file
is not. The code point list must stay sorted because lookup is a binary search —
a test enforces that.

## Dependencies without crates.io

`crates.io` is unreachable here, and so is every index mirror tried
(`rsproxy.cn`, USTC, TUNA, SJTUG, aliyun, tencent) — all return connection
errors, while `github.com` works. Two workarounds were verified:

**1. Git dependencies** — fine for crates with no dependencies of their own.

```toml
libc = { git = "https://github.com/rust-lang/libc", tag = "0.2.174" }
```

Verified building and linking. Breaks as soon as a crate needs a second crate:
`serde` with `derive` pulls `proc-macro2`, `quote`, and `syn` from the registry,
and cargo fails at resolution.

**2. Vendored directory** — works for transitive trees, but each crate has to be
downloaded by hand from GitHub and given a `.cargo-checksum.json`:

```toml
[source.crates-io]
replace-with = "vendored-sources"

[source.vendored-sources]
directory = "vendor"
```

Verified with `libc`. Scripting this is only practical for a handful of crates.

The GUI needs neither: it is written against the X11 protocol and a baked font,
so `rust/tui` still has an empty dependency list.

## Cross-platform notes

| Target | Consequence |
|---|---|
| Linux | X11 backend implemented; Wayland would need its own wire client |
| Windows | window and blit belong in a `cfg(windows)` module using Win32 `BitBlt`/DIB; keep the WinINET system-proxy and `netsh` kill switch there |
| Android | surface comes from `ANativeWindow`; no system proxy is settable, so the CONNECT design changes to `VpnService` with per-app opt-in |

The `view`, `layout`, `paint`, `font`, and `ascii` modules are platform
independent — only `x11` and the `run` loop in `app` are Linux-specific. That is
the seam a Windows or Android backend plugs into: produce an `Event`, call
`App::click`, blit the `Canvas`.
