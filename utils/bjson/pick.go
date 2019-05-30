package bjson

func PickString(payload map[string]interface{}, target string) string {
	if x, found := payload[target]; found {
		if val, ok := x.(string); ok {
			return val
		}
	}

	return ""
}

func PickInt64(payload map[string]interface{}, target string) int64 {
	if x, found := payload[target]; found {
		if val, ok := x.(int64); ok {
			return val
		}
	}

	return 0
}
