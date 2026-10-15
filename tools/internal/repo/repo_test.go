package repo

import "testing"

func TestParseTag(t *testing.T) {
	component, version, err := ParseTag("spikectl/v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := component, "spikectl"; got != want {
		t.Errorf("component=%s, want=%s", got, want)
	}
	if got, want := version, "0.1.0"; got != want {
		t.Errorf("version=%s, want=%s", got, want)
	}
	for _, tag := range []string{"v0.1.0", "spikectl/0.1.0", "/v1.0.0", "spikectl/v"} {
		if _, _, err := ParseTag(tag); err == nil {
			t.Errorf("ParseTag(%q) gave no error", tag)
		}
	}
	if got, want := Tag("spiked", "1.2.3"), "spiked/v1.2.3"; got != want {
		t.Errorf("Tag=%s, want=%s", got, want)
	}
}
