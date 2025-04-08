package utils

import "testing"

/* func MatchKeysIgnoreOrder(expected []string) func([]string) bool {
	return func(actual []string) bool {
		if len(actual) != len(expected) {
			return false
		}
		expectedMap := make(map[string]bool)
		for _, k := range expected {
			expectedMap[k] = true
		}
		for _, k := range actual {
			if !expectedMap[k] {
				return false
			}
		}
		return true
	}
} */

func TestMatchKeysIgnoreOrder(t *testing.T) {
	expected := []string{"key1", "key2", "key3"}
	actual := []string{"key3", "key1", "key2"}
	matchFunc := MatchKeysIgnoreOrder(expected)

	if !matchFunc(actual) {
		t.Errorf("Expected keys to match, but they didn't")
	}

	actual = []string{"key1", "key2"}
	if matchFunc(actual) {
		t.Errorf("Expected keys to not match, but they did")
	}
}
