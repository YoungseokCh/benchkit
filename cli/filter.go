package cli

import "strings"

func filterCases(cases []Case, names []string, tags []string, match string) []Case {
	nameSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" {
			nameSet[name] = struct{}{}
		}
	}

	tagSet := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tagSet[tag] = struct{}{}
		}
	}

	var selected []Case
	for _, c := range cases {
		if len(nameSet) > 0 {
			if _, ok := nameSet[c.Name]; !ok {
				continue
			}
		}
		if match != "" && !strings.Contains(c.Name, match) {
			continue
		}
		if len(tagSet) > 0 && !hasAllTags(c.Tags, tagSet) {
			continue
		}
		selected = append(selected, c)
	}
	return selected
}

func hasAllTags(tags []string, required map[string]struct{}) bool {
	have := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		have[tag] = struct{}{}
	}
	for tag := range required {
		if _, ok := have[tag]; !ok {
			return false
		}
	}
	return true
}
