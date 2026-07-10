package types

// CompareValues compares two values using the specified operator
// Handles numeric, string, and boolean comparisons
// Supports: =, <, >, <=, >=, !=, <>
func CompareValues(left interface{}, op string, right interface{}) bool {
	// Try numeric comparison first
	if n1, ok := NormalizeToFloat(left); ok {
		if n2, ok := NormalizeToFloat(right); ok {
			switch op {
			case "=":
				return n1 == n2
			case "!=", "<>":
				return n1 != n2
			case "<":
				return n1 < n2
			case ">":
				return n1 > n2
			case "<=":
				return n1 <= n2
			case ">=":
				return n1 >= n2
			}
		}
	}
	
	// Try string comparison
	if s1, ok := left.(string); ok {
		if s2, ok := right.(string); ok {
			switch op {
			case "=":
				return s1 == s2
			case "!=", "<>":
				return s1 != s2
			case "<":
				return s1 < s2
			case ">":
				return s1 > s2
			case "<=":
				return s1 <= s2
			case ">=":
				return s1 >= s2
			}
		}
	}
	
	// Fallback: direct equality/inequality comparison for booleans and other types
	switch op {
	case "=":
		return left == right
	case "!=", "<>":
		return left != right
	default:
		// For non-comparable types with ordering operators, return false
		return false
	}
}

// NormalizeToFloat converts various numeric types to float64
func NormalizeToFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case float64:
		return val, true
	}
	return 0, false
}

// NormalizeToInt64 converts various numeric types to int64
func NormalizeToInt64(val interface{}) (int64, bool) {
	switch v := val.(type) {
	case float64:
		if v == float64(int64(v)) {
			return int64(v), true
		}
	case int64:
		return v, true
	case int:
		return int64(v), true
	}
	return 0, false
}
