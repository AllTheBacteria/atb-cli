package agc

import (
	"runtime"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

func TestBinaryName(t *testing.T) {
	got := binaryName()
	want := "agc"
	if runtime.GOOS == "windows" {
		want = "agc.exe"
	}
	if got != want {
		t.Errorf("binaryName() = %q, want %q", got, want)
	}
}

func TestVersionPins(t *testing.T) {
	if sources.AGCVersion == "" {
		t.Error("sources.AGCVersion is empty")
	}
	if sources.AGCRepo != "refresh-bio/agc" {
		t.Errorf("sources.AGCRepo = %q, want refresh-bio/agc", sources.AGCRepo)
	}
}

func TestPlatformAsset(t *testing.T) {
	v := sources.AGCVersion
	cases := []struct {
		goos, goarch string
		wantAsset    string
		wantZip      bool
		wantErr      bool
	}{
		{"linux", "amd64", "agc-" + v + "-x64_linux.tar.gz", false, false},
		{"linux", "arm64", "agc-" + v + "-arm64_linux.tar.gz", false, false},
		{"darwin", "amd64", "agc-" + v + "-x64_mac.tar.gz", false, false},
		{"darwin", "arm64", "agc-" + v + "-m1_mac.tar.gz", false, false},
		{"windows", "amd64", "agc-" + v + "-x64_windows.zip", true, false},
		{"windows", "arm64", "", false, true},
		{"plan9", "amd64", "", false, true},
	}
	for _, c := range cases {
		asset, isZip, err := platformAsset(c.goos, c.goarch)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s/%s: expected error, got asset %q", c.goos, c.goarch, asset)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s/%s: unexpected error: %v", c.goos, c.goarch, err)
			continue
		}
		if asset != c.wantAsset || isZip != c.wantZip {
			t.Errorf("%s/%s: got (%q, %v), want (%q, %v)",
				c.goos, c.goarch, asset, isZip, c.wantAsset, c.wantZip)
		}
	}
}
