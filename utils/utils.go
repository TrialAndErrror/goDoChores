package utils

func ReverseMap(data map[string]string) map[string]string {
	newMap := make(map[string]string)
	for key, value := range data {
		newMap[value] = key
	}

	return newMap
}
