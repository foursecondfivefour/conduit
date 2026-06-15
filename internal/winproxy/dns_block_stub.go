//go:build !windows

package winproxy

// DNSBlocker is a no-op on non-Windows platforms.
type DNSBlocker struct{}

func NewDNSBlocker() *DNSBlocker { return &DNSBlocker{} }

func (b *DNSBlocker) Block() error    { return nil }
func (b *DNSBlocker) Unblock() error  { return nil }
func (b *DNSBlocker) IsBlocked() bool { return false }
