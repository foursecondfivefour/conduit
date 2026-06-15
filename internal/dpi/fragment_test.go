package dpi

import (
	"context"
	"net"
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
		{StrategyTCPSegmentation, "tcp_segmentation"},
		{StrategyTCPSegmentation1, "tcp_segmentation_1"},
		{StrategyTCPSegmentation8, "tcp_segmentation_8"},
		{StrategyTLSRecordFrag, "tls_record_frag"},
		{StrategyTLSRecordFrag2, "tls_record_frag_2"},
		{StrategyECH, "ech"},
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
		{StrategyTCPSegmentation, "tcp_seg"},
		{StrategyTCPSegmentation1, "tcp_1"},
		{StrategyTCPSegmentation8, "tcp_8"},
		{StrategyTLSRecordFrag, "tls_frag"},
		{StrategyTLSRecordFrag2, "tls_2"},
		{StrategyECH, "ech"},
	}
	for _, tt := range tests {
		if got := tt.s.ShortLabel(); got != tt.want {
			t.Errorf("Strategy(%s).ShortLabel() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestAllStrategiesIncludesECH(t *testing.T) {
	all := AllStrategies()
	found := false
	for _, s := range all {
		if s == StrategyECH {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("AllStrategies should include StrategyECH")
	}
}

func TestNewFragmentWriterDefaults(t *testing.T) {
	w := NewFragmentWriter(Strategy("bad"))
	if w.strategy != StrategyTCPSegmentation {
		t.Fatalf("got %s", w.strategy)
	}
}

func TestAIStrategySelectorGetDefault(t *testing.T) {
	selector := NewAIStrategySelector(StrategyTCPSegmentation8)
	if got := selector.GetStrategyForHost("example.com"); got != StrategyTCPSegmentation8 {
		t.Fatalf("default strategy = %s, want %s", got, StrategyTCPSegmentation8)
	}
}

func TestAIStrategySelectorRecordAndGet(t *testing.T) {
	selector := NewAIStrategySelector(StrategyNone)
	selector.RecordStrategy("www.youtube.com", StrategyTCPSegmentation8)
	selector.RecordStrategy("www.example.com", StrategyTLSRecordFrag)

	if got := selector.GetStrategyForHost("www.youtube.com"); got != StrategyTCPSegmentation8 {
		t.Fatalf("youtube strategy = %s, want %s", got, StrategyTCPSegmentation8)
	}
	if got := selector.GetStrategyForHost("www.example.com"); got != StrategyTLSRecordFrag {
		t.Fatalf("example strategy = %s, want %s", got, StrategyTLSRecordFrag)
	}
	if got := selector.GetStrategyForHost("unknown.com"); got != StrategyNone {
		t.Fatalf("unknown host = %s, want %s", got, StrategyNone)
	}
}

func TestAIStrategySelectorOverwrite(t *testing.T) {
	selector := NewAIStrategySelector(StrategyNone)
	selector.RecordStrategy("test.com", StrategyTCPSegmentation)
	selector.RecordStrategy("test.com", StrategyTLSRecordFrag)

	if got := selector.GetStrategyForHost("test.com"); got != StrategyTLSRecordFrag {
		t.Fatalf("overwritten strategy = %s, want %s", got, StrategyTLSRecordFrag)
	}
}

func TestAIStrategySelectorProbeHost(t *testing.T) {
	selector := NewAIStrategySelector(StrategyTCPSegmentation)

	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		client, server := net.Pipe()
		_ = server.Close()
		return client, nil
	}

	strat, err := selector.ProbeHost(context.Background(), "test.com", 443, dial)
	if err != nil || strat.Valid() == false {
		t.Fatalf("ProbeHost failed: err=%v strat=%s", err, strat)
	}
	if strat == StrategyNone {
		t.Fatal("probe should not return StrategyNone")
	}
	cached := selector.GetStrategyForHost("test.com")
	if cached != strat {
		t.Fatalf("cached = %s, want %s", cached, strat)
	}
}

func TestAIStrategySelectorProbeAllFail(t *testing.T) {
	selector := NewAIStrategySelector(StrategyTCPSegmentation8)

	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		return nil, net.ErrClosed
	}

	strat, err := selector.ProbeHost(context.Background(), "bad.com", 443, dial)
	if err == nil {
		t.Fatal("expected error when all probes fail")
	}
	if strat != StrategyTCPSegmentation8 {
		t.Fatalf("fallback strategy = %s, want %s", strat, StrategyTCPSegmentation8)
	}
}

func TestECHConn(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	go func() {
		tlsConn, err := ECHConn(client, "example.com")
		if err != nil {
			t.Errorf("ECHConn: %v", err)
			return
		}
		_ = tlsConn.Close()
	}()

	buf := make([]byte, 1024)
	_, err := server.Read(buf)
	if err == nil {
		t.Log("ECHConn initiated TLS handshake")
	}
}

func TestDomainPolicyDefault(t *testing.T) {
	policy := NewDomainPolicy(StrategyTCPSegmentation8)
	if got := policy.Get("any-host.com"); got != StrategyTCPSegmentation8 {
		t.Fatalf("default = %s, want %s", got, StrategyTCPSegmentation8)
	}
}

func TestDomainPolicyExactMatch(t *testing.T) {
	policy := NewDomainPolicy(StrategyNone)
	policy.Set("www.youtube.com", StrategyTCPSegmentation8)

	if got := policy.Get("www.youtube.com"); got != StrategyTCPSegmentation8 {
		t.Fatalf("exact match = %s, want %s", got, StrategyTCPSegmentation8)
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
	policy.Set("youtube.com", StrategyTCPSegmentation)
	policy.Set("www.youtube.com", StrategyTLSRecordFrag)

	if got := policy.Get("www.youtube.com"); got != StrategyTLSRecordFrag {
		t.Fatalf("specific = %s, want %s", got, StrategyTLSRecordFrag)
	}
	if got := policy.Get("music.youtube.com"); got != StrategyTCPSegmentation {
		t.Fatalf("parent = %s, want %s", got, StrategyTCPSegmentation)
	}
}

func TestDomainPolicySetDefault(t *testing.T) {
	policy := NewDomainPolicy(StrategyNone)
	policy.SetDefault(StrategyTCPSegmentation8)

	if got := policy.Get("test.com"); got != StrategyTCPSegmentation8 {
		t.Fatalf("new default = %s, want %s", got, StrategyTCPSegmentation8)
	}
}

func TestDomainPolicyNilMap(t *testing.T) {
	policy := &DomainPolicy{defaultStrategy: StrategyNone}
	if got := policy.Get("test.com"); got != StrategyNone {
		t.Fatalf("nil map = %s, want %s", got, StrategyNone)
	}
}
