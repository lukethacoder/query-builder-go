package querybuilder

import (
	"fmt"
	"strings"
)

// NormalizeOption converts a flexible option (name-only, value-only, or string)
// into a fully-resolved FullOption (§2.6).
func NormalizeOption(name, value, label string) FullOption {
	if name == "" {
		name = value
	}
	if value == "" {
		value = name
	}
	if label == "" {
		label = name
	}
	return FullOption{Name: name, Value: value, Label: label}
}

// NormalizeOptionFromString converts a bare string identifier into a FullOption.
func NormalizeOptionFromString(s string) FullOption {
	return FullOption{Name: s, Value: s, Label: s}
}

// FlattenOptionList returns all options from a list, expanding groups (§2.2).
func FlattenOptionList(list *OptionList) []FullOption {
	if list == nil {
		return nil
	}
	if len(list.Groups) > 0 {
		var result []FullOption
		for _, g := range list.Groups {
			result = append(result, g.Options...)
		}
		return result
	}
	return list.Flat
}

// FindOption looks up an option by name or value (§2.7).
func FindOption(list *OptionList, identifier string) *FullOption {
	for _, opt := range FlattenOptionList(list) {
		o := opt
		if o.Name == identifier || o.Value == identifier {
			return &o
		}
	}
	return nil
}

// FirstOption returns the identifier (Value) of the first option in the list.
func FirstOption(list *OptionList) string {
	flat := FlattenOptionList(list)
	if len(flat) == 0 {
		return ""
	}
	return flat[0].Value
}

// GetOperatorArity returns the numeric arity for an operator (§3.4).
func GetOperatorArity(op Operator) int {
	if op.Arity != nil {
		switch v := op.Arity.(type) {
		case string:
			switch v {
			case "unary":
				return 1
			case "binary":
				return 2
			case "ternary":
				return 3
			}
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	switch op.Value {
	case "null", "notNull":
		return 1
	case "between", "notBetween":
		return 3
	}
	return 2
}

// IsUnaryOperator reports whether the operator takes no value.
func IsUnaryOperator(op Operator) bool {
	return GetOperatorArity(op) < 2
}

// ResolveValueSources returns the applicable value sources for a field +
// operator combination (§2.7).
func ResolveValueSources(f *Field, operator string) ValueSources {
	if f != nil && len(f.ValueSources) > 0 {
		return f.ValueSources
	}
	return ValueSources{ValueSourceValue}
}

// ParseMultiValue splits a delimited or array value into a string slice (§3.3).
// joinChar defaults to DefaultJoinChar if empty.
func ParseMultiValue(value any, joinChar string) []string {
	if joinChar == "" {
		joinChar = DefaultJoinChar
	}
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, len(v))
		for i, x := range v {
			out[i] = fmt.Sprintf("%v", x)
		}
		return out
	case string:
		return splitEscaped(v, joinChar)
	case nil:
		return nil
	default:
		return []string{fmt.Sprintf("%v", v)}
	}
}

func splitEscaped(s, sep string) []string {
	var result []string
	var current strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+len(sep) < len(s) && s[i+1:i+1+len(sep)] == sep {
			current.WriteString(sep)
			i += 1 + len(sep)
			continue
		}
		if strings.HasPrefix(s[i:], sep) {
			trimmed := strings.TrimSpace(current.String())
			if trimmed != "" {
				result = append(result, trimmed)
			}
			current.Reset()
			i += len(sep)
			continue
		}
		current.WriteByte(s[i])
		i++
	}
	if trimmed := strings.TrimSpace(current.String()); trimmed != "" {
		result = append(result, trimmed)
	}
	return result
}

// JoinMultiValue encodes a slice of values into a delimited string (§3.3).
func JoinMultiValue(values []string, joinChar string) string {
	if joinChar == "" {
		joinChar = DefaultJoinChar
	}
	escaped := make([]string, len(values))
	for i, v := range values {
		escaped[i] = strings.ReplaceAll(v, joinChar, "\\"+joinChar)
	}
	return strings.Join(escaped, joinChar)
}
