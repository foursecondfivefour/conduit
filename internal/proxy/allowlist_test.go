package proxy

import "testing"

func TestAllowedHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"www.youtube.com", true},
		{"r1---sn-abc.googlevideo.com", true},
		{"i.ytimg.com", true},
		{"youtube.googleapis.com", true},
		{"www.google.com", true},
		{"youtu.be", true},
		{"music.youtube.com", true},
		{"example.com", false},
		{"evil-google.com", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := AllowedHost(tt.host); got != tt.want {
			t.Errorf("AllowedHost(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}
