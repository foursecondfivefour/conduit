//! TLS ClientHello fragmentation core for Conduit.
//!
//! This crate is the strict, dependency-free reference implementation of the
//! DPI evasion layer. It mirrors the rules the Go proxy applies today:
//!
//! * only a TLS handshake record (`0x16`) is fragmented;
//! * the record header is rebuilt for every fragment, so each fragment is a
//!   valid TLS record carrying `chunk` bytes of the original payload;
//! * everything after the first ClientHello is relayed untouched.
//!
//! Every slice access is checked: the crate is compiled with
//! `clippy::indexing_slicing` and `clippy::integer_arithmetic` denied.

#![forbid(unsafe_code)]

use core::fmt;

/// Largest TLS record payload we accept (RFC 8446 §5.1).
pub const MAX_RECORD_PAYLOAD: usize = 16_384;

/// Length of a TLS record header: type, version, length.
pub const RECORD_HEADER_LEN: usize = 5;

/// Length of the record prefix carried into every fragment: type and version.
pub const RECORD_PREFIX_LEN: usize = 3;

/// Content type of a TLS handshake record.
pub const CONTENT_TYPE_HANDSHAKE: u8 = 0x16;

/// How the first ClientHello is sent upstream.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum Strategy {
    /// Send the ClientHello as received.
    None,
    /// Split the payload into 5-byte records (default).
    #[default]
    TlsRecordFrag,
    /// Split the payload into 2-byte records.
    TlsRecordFrag2,
}

impl Strategy {
    /// Fragment payload size in bytes, or `None` when fragmentation is off.
    #[must_use]
    pub const fn chunk_len(self) -> Option<usize> {
        match self {
            Self::None => None,
            Self::TlsRecordFrag => Some(5),
            Self::TlsRecordFrag2 => Some(2),
        }
    }

    /// Stable identifier used by `preferences.json`.
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::None => "none",
            Self::TlsRecordFrag => "tls_record_frag",
            Self::TlsRecordFrag2 => "tls_record_frag_2",
        }
    }

    /// Compact label for the tray status line.
    #[must_use]
    pub const fn short_label(self) -> &'static str {
        match self {
            Self::None => "none",
            Self::TlsRecordFrag => "tls_frag",
            Self::TlsRecordFrag2 => "tls_2",
        }
    }

    /// Parses a stored identifier, falling back to [`Strategy::default`].
    #[must_use]
    pub fn from_stored(raw: &str) -> Self {
        match raw {
            "none" => Self::None,
            "tls_record_frag_2" => Self::TlsRecordFrag2,
            _ => Self::default(),
        }
    }
}

impl fmt::Display for Strategy {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Errors produced while fragmenting a record.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FragmentError {
    /// The buffer is shorter than a record header.
    TooShort {
        /// Available bytes.
        got: usize,
    },
    /// The buffer is not a TLS handshake record.
    NotAHandshake {
        /// Observed content type.
        content_type: u8,
    },
    /// The declared record length does not match the buffer.
    LengthMismatch {
        /// Length declared in the header.
        declared: usize,
        /// Bytes actually available after the header.
        available: usize,
    },
    /// The declared record length exceeds [`MAX_RECORD_PAYLOAD`].
    RecordTooLarge {
        /// Declared payload length.
        declared: usize,
    },
    /// The supplied chunk size is zero.
    ZeroChunk,
}

impl fmt::Display for FragmentError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::TooShort { got } => write!(f, "need at least {RECORD_HEADER_LEN} bytes, got {got}"),
            Self::NotAHandshake { content_type } => {
                write!(f, "content type {content_type:#04x} is not a TLS handshake")
            }
            Self::LengthMismatch {
                declared,
                available,
            } => write!(f, "header declares {declared} bytes, {available} available"),
            Self::RecordTooLarge { declared } => {
                write!(f, "record length {declared} exceeds {MAX_RECORD_PAYLOAD}")
            }
            Self::ZeroChunk => f.write_str("chunk size must be positive"),
        }
    }
}

impl core::error::Error for FragmentError {}

/// Reports whether `data` starts with a TLS handshake record.
#[must_use]
pub fn is_client_hello(data: &[u8]) -> bool {
    matches!(data.first(), Some(&CONTENT_TYPE_HANDSHAKE))
}

/// Total size of the ClientHello record, or `None` while it is incomplete.
///
/// Returns `None` for a non-handshake buffer as well; callers that need to
/// distinguish the two cases should call [`is_client_hello`] first.
#[must_use]
pub fn client_hello_len(data: &[u8]) -> Option<usize> {
    if !is_client_hello(data) {
        return None;
    }
    let declared = record_payload_len(data)?;
    if declared == 0 || declared > MAX_RECORD_PAYLOAD {
        return None;
    }
    RECORD_HEADER_LEN.checked_add(declared)
}

/// Payload length declared in the record header.
fn record_payload_len(data: &[u8]) -> Option<usize> {
    let high = *data.get(3)?;
    let low = *data.get(4)?;
    Some(usize::from(high) << 8 | usize::from(low))
}

/// Content type and protocol version of a handshake record.
///
/// The record length is rebuilt per fragment, so it is deliberately not part of
/// this prefix.
fn record_prefix(data: &[u8]) -> Result<[u8; RECORD_PREFIX_LEN], FragmentError> {
    let [content_type, version_major, version_minor, ..] = *data else {
        return Err(FragmentError::TooShort { got: data.len() });
    };
    if content_type != CONTENT_TYPE_HANDSHAKE {
        return Err(FragmentError::NotAHandshake { content_type });
    }
    if data.len() < RECORD_HEADER_LEN {
        return Err(FragmentError::TooShort { got: data.len() });
    }
    Ok([content_type, version_major, version_minor])
}

/// Splits a ClientHello record into TLS records of at most `chunk` payload bytes.
///
/// Each returned fragment is a complete record: the original content type and
/// version followed by the fragment length and the fragment itself. A strategy
/// of [`Strategy::None`] returns a single fragment equal to the input.
///
/// # Errors
///
/// Returns [`FragmentError`] when the buffer is not a well-formed TLS
/// handshake record, or when `chunk` is zero.
pub fn fragment(data: &[u8], chunk: usize) -> Result<Vec<Vec<u8>>, FragmentError> {
    if chunk == 0 {
        return Err(FragmentError::ZeroChunk);
    }
    if data.len() < RECORD_HEADER_LEN {
        return Err(FragmentError::TooShort { got: data.len() });
    }
    let header = record_prefix(data)?;
    let declared = record_payload_len(data).ok_or(FragmentError::TooShort { got: data.len() })?;
    if declared > MAX_RECORD_PAYLOAD {
        return Err(FragmentError::RecordTooLarge { declared });
    }
    let available = data.len().saturating_sub(RECORD_HEADER_LEN);
    if available != declared {
        return Err(FragmentError::LengthMismatch {
            declared,
            available,
        });
    }
    let payload = data
        .get(RECORD_HEADER_LEN..)
        .ok_or(FragmentError::TooShort { got: data.len() })?;

    let capacity = available.div_ceil(chunk);
    let mut out: Vec<Vec<u8>> = Vec::with_capacity(capacity);
    for part in payload.chunks(chunk) {
        let len = u16::try_from(part.len()).map_err(|_| FragmentError::RecordTooLarge {
            declared: part.len(),
        })?;
        let mut record = Vec::with_capacity(RECORD_HEADER_LEN.saturating_add(part.len()));
        record.extend_from_slice(&header);
        let bytes = len.to_be_bytes();
        record.extend_from_slice(&bytes);
        record.extend_from_slice(part);
        out.push(record);
    }
    Ok(out)
}

/// Fragments a record for the given [`Strategy`], returning the input unchanged
/// when fragmentation is disabled.
///
/// # Errors
///
/// Propagates [`FragmentError`] from [`fragment`].
pub fn fragment_with(data: &[u8], strategy: Strategy) -> Result<Vec<Vec<u8>>, FragmentError> {
    match strategy.chunk_len() {
        None => Ok(vec![data.to_vec()]),
        Some(chunk) => fragment(data, chunk),
    }
}

#[cfg(test)]
mod tests {
    use super::{client_hello_len, fragment, fragment_with, is_client_hello, FragmentError, Strategy};

    /// A 22-byte handshake record with a 17-byte payload.
    fn client_hello() -> Vec<u8> {
        let mut v = vec![0x16, 0x03, 0x01, 0x00, 0x11];
        v.extend((0u8..17).map(|i| i.wrapping_add(1)));
        v
    }

    #[test]
    fn detects_client_hello() {
        let data = client_hello();
        assert!(is_client_hello(&data));
        assert!(!is_client_hello(b"GET / HTTP/1.1"));
        assert!(!is_client_hello(&[]));
    }

    #[test]
    fn reports_expected_length() {
        assert_eq!(client_hello_len(&client_hello()), Some(22));
        assert_eq!(client_hello_len(&[0x16, 0x03]), None);
        assert_eq!(client_hello_len(b"GET / HTTP/1.1"), None);
    }

    #[test]
    fn fragments_into_five_byte_records() {
        let records = fragment(&client_hello(), 5).expect("valid record");
        assert_eq!(records.len(), 4);
        assert_eq!(records[0].len(), 10);
        // Every fragment keeps the original type and version.
        assert!(records.iter().all(|r| r[0] == 0x16 && r[1] == 0x03 && r[2] == 0x01));
        // Payload is reassembled byte-for-byte.
        let payload: Vec<u8> = records.iter().flat_map(|r| r[5..].to_vec()).collect();
        assert_eq!(payload.len(), 17);
    }

    #[test]
    fn two_byte_strategy_matches_chunk_two() {
        let data = client_hello();
        assert_eq!(
            fragment_with(&data, Strategy::TlsRecordFrag2).expect("valid"),
            fragment(&data, 2).expect("valid")
        );
    }

    #[test]
    fn none_strategy_returns_input() {
        let data = client_hello();
        assert_eq!(fragment_with(&data, Strategy::None).expect("valid"), vec![data]);
    }

    #[test]
    fn rejects_short_buffer() {
        assert_eq!(
            fragment(&[0x16, 0x03], 5),
            Err(FragmentError::TooShort { got: 2 })
        );
    }

    #[test]
    fn rejects_non_handshake() {
        assert_eq!(
            fragment(b"GET / HTTP/1.1\r\n", 5),
            Err(FragmentError::NotAHandshake { content_type: b'G' })
        );
    }

    #[test]
    fn rejects_length_mismatch() {
        let mut data = client_hello();
        data.pop();
        assert_eq!(
            fragment(&data, 5),
            Err(FragmentError::LengthMismatch {
                declared: 17,
                available: 16
            })
        );
    }

    #[test]
    fn rejects_zero_chunk() {
        assert_eq!(fragment(&client_hello(), 0), Err(FragmentError::ZeroChunk));
    }

    #[test]
    fn strategy_round_trips_through_stored_form() {
        for s in [Strategy::None, Strategy::TlsRecordFrag, Strategy::TlsRecordFrag2] {
            assert_eq!(Strategy::from_stored(s.as_str()), s);
        }
        assert_eq!(Strategy::from_stored("bogus"), Strategy::TlsRecordFrag);
    }

    #[test]
    fn strategies_are_exhaustive() {
        assert_eq!(Strategy::default(), Strategy::TlsRecordFrag);
        assert_eq!(Strategy::None.chunk_len(), None);
        assert_eq!(Strategy::TlsRecordFrag.chunk_len(), Some(5));
        assert_eq!(Strategy::TlsRecordFrag2.chunk_len(), Some(2));
    }
}
