package dpi

import (
	"testing"
)

func TestClientHelloSize(t *testing.T) {
	data := []byte{0x16, 0x03, 0x01, 0x00, 0x0a, 0x01, 0x00, 0x00, 0x06, 0x03, 0x03, 0xaa, 0xbb, 0xcc, 0xdd}
	if !IsTLSClientHello(data) {
		t.Fatal("expected TLS ClientHello")
	}
	if got := ClientHelloSize(data); got != 15 {
		t.Fatalf("ClientHelloSize = %d, want 15", got)
	}
}

func TestStrategyValid(t *testing.T) {
	for _, st := range AllStrategies() {
		if !st.Valid() {
			t.Fatalf("expected valid strategy %s", st)
		}
	}
	if Strategy("unknown").Valid() {
		t.Fatal("unexpected valid strategy")
	}
}

func TestStrategyString(t *testing.T) {
	tests := []struct {
		s    Strategy
		want string
	}{
		{StrategyNone, "none"},
		{StrategyTLSRecordFrag, "tls_record_frag"},
		{StrategyTLSRecordFrag2, "tls_record_frag_2"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Strategy(%s).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestStrategyShortLabel(t *testing.T) {
	tests := []struct {
		s    Strategy
		want string
	}{
		{StrategyNone, "none"},
		{StrategyTLSRecordFrag, "tls_frag"},
		{StrategyTLSRecordFrag2, "tls_2"},
	}
	for _, tt := range tests {
		if got := tt.s.ShortLabel(); got != tt.want {
			t.Errorf("Strategy(%s).ShortLabel() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestAllStrategiesOnlyTLSFragmentation(t *testing.T) {
	for _, s := range AllStrategies() {
		switch s {
		case StrategyNone, StrategyTLSRecordFrag, StrategyTLSRecordFrag2:
		default:
			t.Fatalf("unexpected strategy in AllStrategies: %s", s)
		}
	}
}

func TestNewFragmentWriterDefaults(t *testing.T) {
	w := NewFragmentWriter(Strategy("bad"))
	if w.strategy != DefaultStrategy {
		t.Fatalf("got %s", w.strategy)
	}
}

func TestDomainPolicyDefault(t *testing.T) {
	policy := NewDomainPolicy(StrategyTLSRecordFrag2)
	if got := policy.Get("any-host.com"); got != StrategyTLSRecordFrag2 {
		t.Fatalf("default = %s, want %s", got, StrategyTLSRecordFrag2)
	}
}

func TestDomainPolicyExactMatch(t *testing.T) {
	policy := NewDomainPolicy(StrategyNone)
	policy.Set("www.youtube.com", StrategyTLSRecordFrag2)

	if got := policy.Get("www.youtube.com"); got != StrategyTLSRecordFrag2 {
		t.Fatalf("exact match = %s, want %s", got, StrategyTLSRecordFrag2)
	}
}

func TestDomainPolicyParentDomain(t *testing.T) {
	policy := NewDomainPolicy(StrategyNone)
	policy.Set("youtube.com", StrategyTLSRecordFrag)

	if got := policy.Get("www.youtube.com"); got != StrategyTLSRecordFrag {
		t.Fatalf("parent match = %s, want %s", got, StrategyTLSRecordFrag)
	}
	if got := policy.Get("music.youtube.com"); got != StrategyTLSRecordFrag {
		t.Fatalf("deep parent match = %s, want %s", got, StrategyTLSRecordFrag)
	}
}

func TestDomainPolicyPriority(t *testing.T) {
	policy := NewDomainPolicy(StrategyNone)
	policy.Set("youtube.com", StrategyTLSRecordFrag)
	policy.Set("www.youtube.com", StrategyTLSRecordFrag)

	if got := policy.Get("www.youtube.com"); got != StrategyTLSRecordFrag {
		t.Fatalf("specific = %s, want %s", got, StrategyTLSRecordFrag)
	}
	if got := policy.Get("music.youtube.com"); got != StrategyTLSRecordFrag {
		t.Fatalf("parent = %s, want %s", got, StrategyTLSRecordFrag)
	}
}

func TestDomainPolicySetDefault(t *testing.T) {
	policy := NewDomainPolicy(StrategyNone)
	policy.SetDefault(StrategyTLSRecordFrag2)

	if got := policy.Get("test.com"); got != StrategyTLSRecordFrag2 {
		t.Fatalf("new default = %s, want %s", got, StrategyTLSRecordFrag2)
	}
}

func TestDomainPolicyNilMap(t *testing.T) {
	policy := &DomainPolicy{defaultStrategy: StrategyNone}
	if got := policy.Get("test.com"); got != StrategyNone {
		t.Fatalf("nil map = %s, want %s", got, StrategyNone)
	}
}
