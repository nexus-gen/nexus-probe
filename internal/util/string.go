package util

func FirstNonEmpty(values ...string) string {
	for _, str := range values {
		if str != "" {
			return str
		}
	}
	return ""
}
