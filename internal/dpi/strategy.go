package dpi

// Strategy selects how the first TLS ClientHello is sent upstream.
type Strategy string

const (
	StrategyNone              Strategy = "none"
	StrategyTCPSegmentation   Strategy = "tcp_segmentation"
	StrategyTCPSegmentation1  Strategy = "tcp_segmentation_1"
	StrategyTCPSegmentation8  Strategy = "tcp_segmentation_8"
	StrategyTLSRecordFrag     Strategy = "tls_record_frag"
	StrategyTLSRecordFrag2    Strategy = "tls_record_frag_2"
)

func AllStrategies() []Strategy {
	return []Strategy{
		StrategyNone,
		StrategyTCPSegmentation,
		StrategyTCPSegmentation1,
		StrategyTCPSegmentation8,
		StrategyTLSRecordFrag,
		StrategyTLSRecordFrag2,
	}
}

func (s Strategy) Valid() bool {
	switch s {
	case StrategyNone,
		StrategyTCPSegmentation,
		StrategyTCPSegmentation1,
		StrategyTCPSegmentation8,
		StrategyTLSRecordFrag,
		StrategyTLSRecordFrag2:
		return true
	default:
		return false
	}
}

func (s Strategy) String() string {
	return string(s)
}

func (s Strategy) ShortLabel() string {
	switch s {
	case StrategyNone:
		return "none"
	case StrategyTCPSegmentation:
		return "tcp_seg"
	case StrategyTCPSegmentation1:
		return "tcp_1"
	case StrategyTCPSegmentation8:
		return "tcp_8"
	case StrategyTLSRecordFrag:
		return "tls_frag"
	case StrategyTLSRecordFrag2:
		return "tls_2"
	default:
		return string(s)
	}
}
