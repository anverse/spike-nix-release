package project

import "testing"

func TestParseTag(t *testing.T) {
	c, v, err := ParseTag("spikectl/v0.1.0")
	if err != nil || c != "spikectl" || v != "0.1.0" {
		t.Fatalf("got %q %q %v", c, v, err)
	}
	for _, bad := range []string{"v0.1.0", "spikectl/0.1.0", "/v1.0.0", "spikectl/v"} {
		if _, _, err := ParseTag(bad); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
	if Tag("spiked", "1.2.3") != "spiked/v1.2.3" {
		t.Error("Tag")
	}
}
