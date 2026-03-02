package controllergen

import (
	"strings"
	"unicode"
)

var pluralExceptions = map[string]string{
	"Endpoints": "Endpoints",
}

func pluralize(name string) string {
	if p, ok := pluralExceptions[name]; ok {
		return p
	}
	if strings.HasSuffix(name, "s") {
		return name
	}
	if strings.HasSuffix(name, "y") {
		return name[:len(name)-1] + "ies"
	}
	return name + "s"
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func upperFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
