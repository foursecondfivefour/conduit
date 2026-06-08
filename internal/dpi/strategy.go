package dpi

// Strategy selects how the first TLS ClientHello is sent upstream.
type Strategy string

const (
	StrategyTCPSegmentation Strategy = "tcp_segmentation"
	StrategyTLSRecordFrag   Strategy = "tls_record_frag"
)

func (s Strategy) Valid() bool {
	switch s {
	case StrategyTCPSegmentation, StrategyTLSRecordFrag:
		return true
	default:
		return false
	}
}

func (s Strategy) String() string {
	return string(s)
}
