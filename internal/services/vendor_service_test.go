package services

import "testing"

func TestNormalizeImageURLs_PreservesPrimaryFirstAndDeduplicates(t *testing.T) {
	primary := "https://img.example.com/cover.jpg"

	got := normalizeImageURLs(&primary, []string{
		" https://img.example.com/cover.jpg ",
		"https://img.example.com/detail-1.jpg",
		"https://img.example.com/detail-2.jpg",
		"https://img.example.com/detail-1.jpg",
		"",
	})

	want := []string{
		"https://img.example.com/cover.jpg",
		"https://img.example.com/detail-1.jpg",
		"https://img.example.com/detail-2.jpg",
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d images, got %d (%v)", len(want), len(got), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected image %d to be %q, got %q", i, want[i], got[i])
		}
	}
}

func TestNormalizeImageURLs_AllowsGalleryWithoutPrimary(t *testing.T) {
	got := normalizeImageURLs(nil, []string{
		"https://img.example.com/detail-1.jpg",
		"  https://img.example.com/detail-2.jpg  ",
	})

	want := []string{
		"https://img.example.com/detail-1.jpg",
		"https://img.example.com/detail-2.jpg",
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d images, got %d (%v)", len(want), len(got), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected image %d to be %q, got %q", i, want[i], got[i])
		}
	}
}

