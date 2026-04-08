package tools

import "testing"

func TestCuratedCatalogContainsExpectedTools(t *testing.T) {
	catalog := CuratedCatalog()
	foundNmap := false
	foundYTDLP := false
	for _, item := range catalog {
		if item.Name == "nmap" {
			foundNmap = true
		}
		if item.Name == "yt-dlp" {
			foundYTDLP = true
		}
	}
	if !foundNmap || !foundYTDLP {
		t.Fatalf("expected nmap and yt-dlp in curated catalog")
	}
}
