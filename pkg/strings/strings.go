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
