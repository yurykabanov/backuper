package util

import (
	"regexp"
	"strings"
)

var camelRegex = regexp.MustCompile("[A-Z]?[a-z0-9]+")

func CamelToSnakeCase(str string) string {
	matches := camelRegex.FindAllString(str, -1)
	lowers := make([]string, len(matches))

	for i, match := range matches {
		lowers[i] = strings.ToLower(match)
	}

	return strings.Join(lowers, "_")
}
