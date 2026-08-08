package cli

import (
	"fmt"
	"strconv"
	"strings"
)

type arguments struct {
	values map[string]string
	items  []string
}

var booleanFlags = map[string]bool{
	"execute": true, "validate": true, "partial-failure": true, "stream": true,
	"return-total": true, "force": true, "help": true, "json": true,
	"granular": true,
}

func parseArguments(input []string) (arguments, error) {
	result := arguments{values: map[string]string{}}
	for index := 0; index < len(input); index++ {
		arg := input[index]
		if arg == "--" {
			result.items = append(result.items, input[index+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "--") {
			result.items = append(result.items, arg)
			continue
		}
		keyValue := strings.TrimPrefix(arg, "--")
		if key, value, ok := strings.Cut(keyValue, "="); ok {
			result.values[key] = value
			continue
		}
		if booleanFlags[keyValue] {
			result.values[keyValue] = "true"
			continue
		}
		if index+1 >= len(input) || strings.HasPrefix(input[index+1], "--") {
			return result, fmt.Errorf("--%s requires a value", keyValue)
		}
		index++
		result.values[keyValue] = input[index]
	}
	return result, nil
}

func (a arguments) value(key string) string { return a.values[key] }
func (a arguments) bool(key string) bool    { value, _ := strconv.ParseBool(a.values[key]); return value }
func (a arguments) int(key string, fallback int) (int, error) {
	value := a.values[key]
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("--%s must be an integer", key)
	}
	return parsed, nil
}
