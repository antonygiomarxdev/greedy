package config

import "time"

func ParseFloatParam(params map[string]interface{}, key string) (float64, bool) {
	v, ok := params[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}

func ParseIntParam(params map[string]interface{}, key string) (int64, bool) {
	v, ok := params[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return int64(val), true
	case int:
		return int64(val), true
	case int64:
		return val, true
	default:
		return 0, false
	}
}

func ParseDurationParam(params map[string]interface{}, key string) (time.Duration, bool) {
	v, ok := params[key]
	if !ok {
		return 0, false
	}
	s, ok := v.(string)
	if !ok {
		return 0, false
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, false
	}
	return d, true
}
