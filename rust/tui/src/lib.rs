//! Data-described UI for Conduit.
//!
//! A screen is a value ([`view::View`]); layout turns it into rects; paint turns
//! rects into drawing commands; input is matched against the rect table. Nothing
//! in this crate owns a socket, a thread, or a font.

#![forbid(unsafe_code)]

pub mod view;

pub mod layout;

pub mod ascii;

pub mod font;

pub mod paint;

pub mod x11;

pub mod app;
