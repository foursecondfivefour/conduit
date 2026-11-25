# Rust port

Cross-platform rewrite of Conduit. Linux is the first target; Windows and Android
follow.

## Status

| Piece | State |
|---|---|
| `dpi` — TLS ClientHello fragmentation | done, 11 tests |
| `allowlist` — host presets and custom-domain validation | done, 14 tests |
| `proxy` — CONNECT server | not started |
| `dns` — DoH, cache, ad blocking | not started |
| `gui` — window, tray | blocked, see below |

Both crates are `#![forbid(unsafe_code)]`, have no dependencies, and share the
workspace lint set: `clippy::all`/`pedantic`/`cargo` at `deny`, plus
`integer_arithmetic`, `indexing_slicing`, `unwrap_used`, `expect_used`, `panic`,
`todo`, `print_stdout`, and `allow_attributes_without_reason`.

Release profile: `lto = "fat"`, `codegen-units = 1`, `panic = "abort"`,
`strip = "symbols"`, `overflow-checks = false`.

## Building

`cargo` needs a linker and, on this class of image, two flags because the
prebuilt toolchain ships without `rust-lld`:

```bash
cargo build --release --offline --workspace
cargo test  --offline --workspace
```

`.cargo/config.toml` already carries `-C link-self-contained=no` and
`-C link-arg=-fuse-ld=bfd`.

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

## What blocks the GUI

A Linux window needs GTK 3 and WebKitGTK 4.1 development files (for `wry` or
`tao`). This image has neither `libgtk-3.so.0` nor `libwebkit2gtk-4.1.so.0`, no
`pkg-config`, and no working package manager, so the GUI cannot be built or run
here even if the crates were available.

## Cross-platform notes

| Target | Consequence |
|---|---|
| Linux | proxy and DNS crates are portable as-is; GUI needs GTK/WebKit |
| Windows | keep the WinINET system-proxy and `netsh` kill switch behind `cfg(windows)` |
| Android | no system proxy is settable from an app; needs `VpnService` + per-app opt-in, so the CONNECT design changes |

The `dpi` and `allowlist` crates are platform-independent, which is why they were
ported first.
