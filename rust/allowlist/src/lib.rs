//! Host allowlist and custom-domain validation for Conduit.
//!
//! This is the gate in front of the CONNECT proxy: a host is proxied only if it
//! matches the active preset or an explicitly validated custom domain. The rules
//! mirror the Go implementation byte for byte, including the over-broad suffix
//! deny list that keeps `com` and `google.com` out of custom allowlists.

#![forbid(unsafe_code)]

use core::fmt;

/// Preset selected from the tray.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum Preset {
    /// YouTube plus the Google hosts the player needs.
    #[default]
    YouTube,
    /// Google media hosts, without the broader `.google.com` entry.
    GoogleMedia,
    /// Custom domains, falling back to [`Preset::YouTube`] when empty.
    Custom,
}

impl Preset {
    /// Stable identifier used by `preferences.json`.
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::YouTube => "youtube",
            Self::GoogleMedia => "google_media",
            Self::Custom => "custom",
        }
    }

    /// Parses a stored identifier, falling back to [`Preset::GoogleMedia`].
    #[must_use]
    pub fn from_stored(raw: &str) -> Self {
        match raw {
            "youtube" => Self::YouTube,
            "custom" => Self::Custom,
            _ => Self::GoogleMedia,
        }
    }

    /// Suffix list for this preset.
    #[must_use]
    pub const fn suffixes(self) -> &'static [&'static str] {
        match self {
            Self::YouTube => &YOUTUBE_ONLY_SUFFIXES,
            Self::GoogleMedia => &GOOGLE_MEDIA_SUFFIXES,
            Self::Custom => &YOUTUBE_ONLY_SUFFIXES,
        }
    }
}

impl fmt::Display for Preset {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Hosts required for player session auth and API calls. Ad hosts such as
/// `doubleclick` and `pagead` are deliberately absent.
pub const YOUTUBE_ONLY_SUFFIXES: [&str; 10] = [
    ".youtube.com",
    ".googlevideo.com",
    ".ytimg.com",
    ".googleapis.com",
    ".google.com",
    ".gstatic.com",
    ".ggpht.com",
    ".googleusercontent.com",
    ".youtu.be",
    ".youtube-nocookie.com",
];

/// Google media hosts.
pub const GOOGLE_MEDIA_SUFFIXES: [&str; 10] = [
    ".youtube.com",
    ".googlevideo.com",
    ".ytimg.com",
    ".googleapis.com",
    ".ggpht.com",
    ".gstatic.com",
    ".googleusercontent.com",
    ".google.com",
    ".youtube-nocookie.com",
    ".youtu.be",
];

/// Suffixes that must never appear in a custom allowlist.
pub const BLOCKED_SUFFIXES: [&str; 13] = [
    ".com", ".net", ".org", ".io", ".co", ".ru", ".uk", ".de", ".fr", ".cn", ".google.com",
    ".youtube.com", ".googlevideo.com",
];

/// Maximum number of custom domains accepted.
pub const MAX_CUSTOM_DOMAINS: usize = 64;

/// Rejection reasons for a custom domain.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DomainError {
    /// The entry was empty after trimming.
    Empty,
    /// Wildcards are not supported.
    Wildcard,
    /// The entry contains a path, query, fragment, or percent escape.
    InvalidCharacters,
    /// Bare IP addresses are rejected (SSRF).
    IpAddress,
    /// Fewer than two labels.
    TooFewLabels,
    /// A label is empty or longer than 63 bytes.
    InvalidLabel,
    /// A label contains characters outside `[A-Za-z0-9-]`.
    InvalidLabelCharacters,
    /// The suffix is an over-broad entry such as `.com` or `.google.com`.
    TooBroad,
}

impl fmt::Display for DomainError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let msg = match self {
            Self::Empty => "empty domain",
            Self::Wildcard => "wildcards not allowed",
            Self::InvalidCharacters => "invalid characters",
            Self::IpAddress => "IP addresses not allowed",
            Self::TooFewLabels => "domain must have at least two labels",
            Self::InvalidLabel => "invalid label",
            Self::InvalidLabelCharacters => "invalid label characters",
            Self::TooBroad => "suffix is too broad",
        };
        f.write_str(msg)
    }
}

impl core::error::Error for DomainError {}

/// Normalizes a host for comparison: lowercased, single trailing dot removed.
#[must_use]
pub fn normalize_host(host: &str) -> String {
    let lower = host.trim().to_ascii_lowercase();
    lower.strip_suffix('.').unwrap_or(&lower).to_owned()
}

/// Validates a custom domain entry.
///
/// # Errors
///
/// Returns [`DomainError`] describing the first rule the entry violates.
pub fn validate_custom_domain(domain: &str) -> Result<(), DomainError> {
    let normalized = normalize_host(domain);
    if normalized.is_empty() {
        return Err(DomainError::Empty);
    }
    if normalized.contains('*') {
        return Err(DomainError::Wildcard);
    }
    if normalized
        .chars()
        .any(|c| matches!(c, '/' | '\\' | '?' | '#' | '%'))
    {
        return Err(DomainError::InvalidCharacters);
    }
    if is_ip_literal(&normalized) {
        return Err(DomainError::IpAddress);
    }
    if normalized.split('.').count() < 2 {
        return Err(DomainError::TooFewLabels);
    }
    for label in normalized.split('.') {
        if label.is_empty() || label.len() > 63 {
            return Err(DomainError::InvalidLabel);
        }
        if !label
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || c == '-')
        {
            return Err(DomainError::InvalidLabelCharacters);
        }
    }
    let suffix = if normalized.starts_with('.') {
        normalized.clone()
    } else {
        format!(".{normalized}")
    };
    if BLOCKED_SUFFIXES.contains(&suffix.as_str()) {
        return Err(DomainError::TooBroad);
    }
    Ok(())
}

/// Reports whether `value` is a bare IPv4 or IPv6 literal.
fn is_ip_literal(value: &str) -> bool {
    value.parse::<std::net::IpAddr>().is_ok()
}

/// Keeps only valid custom domains, preserving order.
#[must_use]
pub fn filter_custom_domains(domains: &[String]) -> Vec<String> {
    let mut seen = std::collections::HashSet::new();
    let mut out = Vec::with_capacity(domains.len().min(MAX_CUSTOM_DOMAINS));
    for domain in domains {
        if out.len() >= MAX_CUSTOM_DOMAINS {
            break;
        }
        if validate_custom_domain(domain).is_err() {
            continue;
        }
        let key = domain.trim().to_ascii_lowercase();
        if !seen.insert(key) {
            continue;
        }
        out.push(domain.clone());
    }
    out
}

/// Reports whether `host` is allowed under `preset` with `custom` domains.
#[must_use]
pub fn allowed_host(host: &str, preset: Preset, custom: &[String]) -> bool {
    let normalized = normalize_host(host);
    if normalized.is_empty() {
        return false;
    }
    let suffixes = suffixes_for(preset, custom);
    suffixes.iter().any(|suffix| {
        let bare = suffix.strip_prefix('.').unwrap_or(suffix);
        normalized == bare || normalized.ends_with(suffix.as_str())
    })
}

/// Suffix list for a preset, with custom domains taking over for [`Preset::Custom`].
fn suffixes_for(preset: Preset, custom: &[String]) -> Vec<String> {
    if preset != Preset::Custom {
        return preset.suffixes().iter().map(|s| (*s).to_owned()).collect();
    }
    let filtered = filter_custom_domains(custom);
    if filtered.is_empty() {
        return YOUTUBE_ONLY_SUFFIXES
            .iter()
            .map(|s| (*s).to_owned())
            .collect();
    }
    filtered
        .iter()
        .map(|d| {
            let normalized = normalize_host(d);
            if normalized.starts_with('.') {
                normalized
            } else {
                format!(".{normalized}")
            }
        })
        .collect()
}

/// Report produced when sanitizing a stored allowlist.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct AllowlistSanitizeReport {
    /// Domains that survived validation, in input order.
    pub kept: Vec<String>,
    /// Domains that were rejected, paired with the reason.
    pub rejected: Vec<(String, DomainError)>,
}

/// Validates and truncates a stored custom allowlist.
#[must_use]
pub fn sanitize_custom_domains(domains: &[String]) -> AllowlistSanitizeReport {
    let mut report = AllowlistSanitizeReport::default();
    for domain in domains {
        match validate_custom_domain(domain) {
            Ok(()) => {
                if report.kept.len() < MAX_CUSTOM_DOMAINS {
                    report.kept.push(domain.clone());
                }
            }
            Err(err) => report.rejected.push((domain.clone(), err)),
        }
    }
    report
}

#[cfg(test)]
mod tests {
    use super::{
        allowed_host, filter_custom_domains, normalize_host, sanitize_custom_domains,
        validate_custom_domain, DomainError, Preset, MAX_CUSTOM_DOMAINS,
    };

    fn owned(list: &[&str]) -> Vec<String> {
        list.iter().map(|s| (*s).to_owned()).collect()
    }

    #[test]
    fn youtube_preset_allows_playback_hosts() {
        for host in [
            "www.youtube.com",
            "m.youtube.com",
            "rr1---sn-abc.googlevideo.com",
            "i.ytimg.com",
            "www.googleapis.com",
            "youtu.be",
        ] {
            assert!(allowed_host(host, Preset::YouTube, &[]), "{host} should pass");
        }
    }

    #[test]
    fn youtube_preset_blocks_ads_and_others() {
        for host in ["doubleclick.net", "pagead2.googlesyndication.com", "example.com"] {
            assert!(!allowed_host(host, Preset::YouTube, &[]), "{host} should fail");
        }
    }

    #[test]
    fn match_is_case_insensitive_and_dot_tolerant() {
        assert!(allowed_host("WWW.YouTube.COM.", Preset::YouTube, &[]));
        assert_eq!(normalize_host("  WWW.Example.COM. "), "www.example.com");
    }

    #[test]
    fn empty_host_is_rejected() {
        assert!(!allowed_host("", Preset::YouTube, &[]));
        assert!(!allowed_host(".", Preset::YouTube, &[]));
    }

    #[test]
    fn custom_preset_falls_back_to_youtube_when_empty() {
        assert!(allowed_host("www.youtube.com", Preset::Custom, &[]));
        assert!(!allowed_host("example.org", Preset::Custom, &[]));
    }

    #[test]
    fn custom_preset_uses_validated_domains() {
        let custom = owned(&["cdn.example.org", "com"]);
        assert!(allowed_host("cdn.example.org", Preset::Custom, &custom));
        assert!(allowed_host("static.cdn.example.org", Preset::Custom, &custom));
        assert!(!allowed_host("example.org", Preset::Custom, &custom));
    }

    #[test]
    fn rejects_broad_and_malformed_domains() {
        // Single-label entries fail the label-count check first, exactly as in Go.
        assert_eq!(validate_custom_domain("com"), Err(DomainError::TooFewLabels));
        // Leading dot leaves an empty first label, matching Go's "invalid label".
        assert_eq!(validate_custom_domain(".com"), Err(DomainError::InvalidLabel));
        assert_eq!(
            validate_custom_domain("google.com"),
            Err(DomainError::TooBroad)
        );
        assert_eq!(validate_custom_domain("localhost"), Err(DomainError::TooFewLabels));
        assert_eq!(validate_custom_domain("*.example.com"), Err(DomainError::Wildcard));
        assert_eq!(
            validate_custom_domain("evil.com/path"),
            Err(DomainError::InvalidCharacters)
        );
        assert_eq!(validate_custom_domain(""), Err(DomainError::Empty));
    }

    #[test]
    fn rejects_ip_addresses() {
        assert_eq!(validate_custom_domain("127.0.0.1"), Err(DomainError::IpAddress));
        assert_eq!(validate_custom_domain("::1"), Err(DomainError::IpAddress));
    }

    #[test]
    fn accepts_ordinary_domains() {
        for d in ["cdn.example.org", "example.co.uk", "a-b.example.io"] {
            assert_eq!(validate_custom_domain(d), Ok(()), "{d}");
        }
    }

    #[test]
    fn filter_preserves_order_and_drops_invalid() {
        let input = owned(&["b.example.org", "com", "a.example.org"]);
        assert_eq!(
            filter_custom_domains(&input),
            owned(&["b.example.org", "a.example.org"])
        );
    }

    #[test]
    fn sanitize_reports_rejections() {
        let input = owned(&["ok.example.org", "com"]);
        let report = sanitize_custom_domains(&input);
        assert_eq!(report.kept, owned(&["ok.example.org"]));
        assert_eq!(report.rejected.len(), 1);
        assert_eq!(report.rejected[0].1, DomainError::TooFewLabels);
    }

    #[test]
    fn sanitize_truncates_to_cap() {
        let input: Vec<String> = (0..(MAX_CUSTOM_DOMAINS + 10))
            .map(|i| format!("d{i}.example.org"))
            .collect();
        let report = sanitize_custom_domains(&input);
        assert_eq!(report.kept.len(), MAX_CUSTOM_DOMAINS);
    }

    #[test]
    fn preset_round_trips_through_stored_form() {
        for p in [Preset::YouTube, Preset::GoogleMedia, Preset::Custom] {
            assert_eq!(Preset::from_stored(p.as_str()), p);
        }
        assert_eq!(Preset::from_stored("bogus"), Preset::GoogleMedia);
    }

    #[test]
    fn domain_error_messages_are_stable() {
        assert_eq!(DomainError::TooBroad.to_string(), "suffix is too broad");
        assert_eq!(DomainError::Wildcard.to_string(), "wildcards not allowed");
    }
}
