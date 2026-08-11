package update

import "testing"

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{current: "1.0.0", latest: "1.0.1", want: true},
		{current: "1.2.0", latest: "1.1.9", want: false},
		{current: "1.2.0", latest: "1.2.0", want: false},
		{current: "1.2.0", latest: "2.0.0", want: true},
	}
	for _, test := range tests {
		got, err := IsNewer(test.current, test.latest)
		if err != nil {
			t.Fatalf("IsNewer(%q, %q): %v", test.current, test.latest, err)
		}
		if got != test.want {
			t.Errorf("IsNewer(%q, %q) = %t, want %t", test.current, test.latest, got, test.want)
		}
	}
}

func TestIsNewerRejectsDevelopmentAndInvalidVersions(t *testing.T) {
	if _, err := IsNewer("dev", "1.0.0"); err != ErrDevelopmentBuild {
		t.Fatalf("development build error = %v", err)
	}
	if _, err := IsNewer("one", "1.0.0"); err == nil {
		t.Fatal("invalid current version was accepted")
	}
}
