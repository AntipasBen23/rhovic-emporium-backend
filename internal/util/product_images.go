package util

import "strings"

func NormalizeImageURLs(primary *string, images []string) []string {
	out := make([]string, 0, len(images)+1)
	seen := map[string]struct{}{}

	add := func(raw string) {
		url := strings.TrimSpace(raw)
		if url == "" {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		out = append(out, url)
	}

	if primary != nil {
		add(*primary)
	}
	for _, img := range images {
		add(img)
	}

	return out
}
