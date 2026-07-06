package client

import "testing"

func TestProjectSlugUsesConfiguredSlug(t *testing.T) {
	if got, want := ProjectSlug("custom-slug", "sess_abcdef"), "custom-slug"; got != want {
		t.Fatalf("ProjectSlug() = %q, want %q", got, want)
	}
}

func TestProjectSlugFallsBackToFakeSlug(t *testing.T) {
	got := ProjectSlug("", "abcd1234")
	want := FakeProjectSlug("abcd1234")
	if got != want {
		t.Fatalf("ProjectSlug() = %q, want %q", got, want)
	}
}
