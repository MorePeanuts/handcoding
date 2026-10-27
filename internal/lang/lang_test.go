package lang

import "testing"

func TestAtLeast(t *testing.T) {
	tests := []struct {
		version, min string
		want         bool
	}{
		{"3.10.0", "3.10", true},
		{"3.12.9", "3.10", true},
		{"3.9.6", "3.10", false},
		{"4.0", "3.10", true},
		{"3.10", "3.10.1", false},
		{"1.22rc1", "1.22", true},
		{"1.21.13", "1.22", false},
		{"1.27.2", "1.22", true},
	}
	for _, tt := range tests {
		if got := AtLeast(tt.version, tt.min); got != tt.want {
			t.Errorf("AtLeast(%q, %q) = %v, want %v", tt.version, tt.min, got, tt.want)
		}
	}
}

func TestToolchainErr(t *testing.T) {
	tc := Toolchain{Name: "Python", Version: "3.9.6", Min: "3.10", Upgrade: "install a newer one"}
	err := tc.Err()
	if err == nil {
		t.Fatal("Python 3.9.6 accepted")
	}
	if want := "Python 3.9.6 is older than 3.10, the oldest version hc supports; install a newer one"; err.Error() != want {
		t.Errorf("Err() = %q, want %q", err, want)
	}
	tc.Version = "3.10.0"
	if err := tc.Err(); err != nil {
		t.Errorf("Python 3.10.0 rejected: %v", err)
	}
}
