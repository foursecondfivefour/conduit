package dns

// Provider selects the primary DNS-over-HTTPS endpoint.
type Provider string

const (
	ProviderCloudflare Provider = "cloudflare"
	ProviderGoogle     Provider = "google"
	ProviderQuad9      Provider = "quad9"
)

func (p Provider) Valid() bool {
	switch p {
	case ProviderCloudflare, ProviderGoogle, ProviderQuad9:
		return true
	default:
		return false
	}
}

func (p Provider) String() string {
	return string(p)
}

func (p Provider) Endpoint() string {
	switch p {
	case ProviderGoogle:
		return "https://dns.google/resolve"
	case ProviderQuad9:
		return "https://dns.quad9.net:5053/dns-query"
	default:
		return "https://cloudflare-dns.com/dns-query"
	}
}

func (p Provider) ShortLabel() string {
	switch p {
	case ProviderGoogle:
		return "Google"
	case ProviderQuad9:
		return "Quad9"
	default:
		return "CF"
	}
}

// Fallback returns a secondary provider used when the primary fails.
func (p Provider) Fallback() Provider {
	switch p {
	case ProviderCloudflare:
		return ProviderGoogle
	case ProviderGoogle:
		return ProviderCloudflare
	default:
		return ProviderCloudflare
	}
}
