package strings

import "strings"

func ContainsAny(s string, v []string) bool {
	for _, val := range v {
		if strings.Contains(s, val) {
			return true
		}
	}
	return false
}

func HasAnySuffix(s string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}
