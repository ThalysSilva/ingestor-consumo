package utils

func MatchKeysIgnoreOrder(expected []string) func([]string) bool {
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
}
