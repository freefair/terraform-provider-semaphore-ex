package provider

import (
	"fmt"
	"strconv"
	"strings"
)

// parseImportFields validates the entire identity before it can select an API
// object. Alternative shapes are explicit so extra scope labels cannot be ignored.
func parseImportFields(input string, requiredFields []string, alternatives ...[]string) (map[string]int64, error) {
	parts := strings.Split(input, "/")
	if len(requiredFields) == 1 && len(parts) == 1 {
		parts = []string{requiredFields[0], input}
	}
	shapes := append([][]string{requiredFields}, alternatives...)
	for _, fields := range shapes {
		if len(parts) != 2*len(fields) {
			continue
		}
		result := make(map[string]int64, len(fields))
		matches := true
		for i, field := range fields {
			if parts[2*i] != field {
				matches = false
				break
			}
			value := parts[2*i+1]
			if value == "" || strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
				matches = false
				break
			}
			id, err := strconv.ParseInt(value, 10, 64)
			if err != nil || id <= 0 {
				matches = false
				break
			}
			result[field] = id
		}
		if matches {
			return result, nil
		}
	}
	return nil, fmt.Errorf("expected positive numeric IDs with labels %s in the documented order", strings.Join(requiredFields, ", "))
}
