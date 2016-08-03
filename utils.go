package mswagger

func IsInStringList(list []string, s string) bool {
	for i, _ := range list {
		if list[i] == s {
			return true
		}
	}
	return false
}
