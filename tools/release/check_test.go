package main

import (
	"slices"
	"testing"
)

func TestSameSets(t *testing.T) {
	got := sameSets("a", []string{"x", "y"}, "b", []string{"y", "z"})
	want := []string{"a has x, b does not", "b has z, a does not"}
	if !slices.Equal(got, want) {
		t.Errorf("sameSets=%q, want=%q", got, want)
	}
	if got := sameSets("a", []string{"x"}, "b", []string{"x"}); len(got) != 0 {
		t.Errorf("sameSets of equal lists=%q, want none", got)
	}
}
