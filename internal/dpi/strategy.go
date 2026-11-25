package dpi

// Strategy selects how the first TLS ClientHello is sent upstream.
//
// Conduit deliberately keeps this surface small: the only evasion mechanism
// that ships today is TLS record fragmentation. TCP segmentation, ECH and the
// heuristic "AI" selector were removed because they only added menu noise
// without a working implementation behind them.
type Strategy string

const (
	// StrategyNone disables fragmentation and sends the ClientHello as-is.
	StrategyNone Strategy = "none"
	// StrategyTLSRecordFrag splits the ClientHello into 5-byte TLS records.
	StrategyTLSRecordFrag Strategy = "tls_record_frag"
	// StrategyTLSRecordFrag2 splits the ClientHello into 2-byte TLS records.
	StrategyTLSRecordFrag2 Strategy = "tls_record_frag_2"
)

// DefaultStrategy is used when a stored strategy is missing or invalid.
const DefaultStrategy = StrategyTLSRecordFrag

// AllStrategies returns the supported strategies in menu order.
func AllStrategies() []Strategy {
	return []Strategy{
		StrategyTLSRecordFrag,
		StrategyTLSRecordFrag2,
		StrategyNone,
	}
}

// Valid reports whether the strategy is supported.
func (s Strategy) Valid() bool {
	switch s {
	case StrategyNone, StrategyTLSRecordFrag, StrategyTLSRecordFrag2:
		return true
	default:
		return false
	}
}

func (s Strategy) String() string {
	return string(s)
}

// ShortLabel returns a compact label for the tray status line.
func (s Strategy) ShortLabel() string {
	switch s {
	case StrategyNone:
		return "none"
	case StrategyTLSRecordFrag:
		return "tls_frag"
	case StrategyTLSRecordFrag2:
		return "tls_2"
	default:
		return string(s)
	}
}
