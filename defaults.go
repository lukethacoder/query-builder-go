package querybuilder

// DefaultOperators is the canonical set of 18 operators defined in §3.1.
var DefaultOperators = []Operator{
	{FullOption: FullOption{Name: "=", Value: "=", Label: "="}},
	{FullOption: FullOption{Name: "!=", Value: "!=", Label: "!="}},
	{FullOption: FullOption{Name: "<", Value: "<", Label: "<"}},
	{FullOption: FullOption{Name: ">", Value: ">", Label: ">"}},
	{FullOption: FullOption{Name: "<=", Value: "<=", Label: "<="}},
	{FullOption: FullOption{Name: ">=", Value: ">=", Label: ">="}},
	{FullOption: FullOption{Name: "contains", Value: "contains", Label: "contains"}},
	{FullOption: FullOption{Name: "beginsWith", Value: "beginsWith", Label: "begins with"}},
	{FullOption: FullOption{Name: "endsWith", Value: "endsWith", Label: "ends with"}},
	{FullOption: FullOption{Name: "doesNotContain", Value: "doesNotContain", Label: "does not contain"}},
	{FullOption: FullOption{Name: "doesNotBeginWith", Value: "doesNotBeginWith", Label: "does not begin with"}},
	{FullOption: FullOption{Name: "doesNotEndWith", Value: "doesNotEndWith", Label: "does not end with"}},
	{FullOption: FullOption{Name: "null", Value: "null", Label: "is null"}, Arity: "unary"},
	{FullOption: FullOption{Name: "notNull", Value: "notNull", Label: "is not null"}, Arity: "unary"},
	{FullOption: FullOption{Name: "in", Value: "in", Label: "in"}},
	{FullOption: FullOption{Name: "notIn", Value: "notIn", Label: "not in"}},
	{FullOption: FullOption{Name: "between", Value: "between", Label: "between"}, Arity: "ternary"},
	{FullOption: FullOption{Name: "notBetween", Value: "notBetween", Label: "not between"}, Arity: "ternary"},
}

// DefaultCombinators is the standard pair of combinators (§3.2).
var DefaultCombinators = []Combinator{
	{Name: "and", Value: "and", Label: "AND"},
	{Name: "or", Value: "or", Label: "OR"},
}

// ExtendedCombinators adds XOR to the default set.
var ExtendedCombinators = append(append([]Combinator{}, DefaultCombinators...), Combinator{
	Name: "xor", Value: "xor", Label: "XOR",
})

// OperatorNegationMap maps each operator to its logical opposite (§3.1).
var OperatorNegationMap = map[string]string{
	"=":                "!=",
	"!=":               "=",
	"<":                ">=",
	">=":               "<",
	">":                "<=",
	"<=":               ">",
	"contains":         "doesNotContain",
	"doesNotContain":   "contains",
	"beginsWith":       "doesNotBeginWith",
	"doesNotBeginWith": "beginsWith",
	"endsWith":         "doesNotEndWith",
	"doesNotEndWith":   "endsWith",
	"null":             "notNull",
	"notNull":          "null",
	"in":               "notIn",
	"notIn":            "in",
	"between":          "notBetween",
	"notBetween":       "between",
}

// MultiValueOperators is the set of operators that accept comma-separated
// or array values (§3.3).
var MultiValueOperators = map[string]bool{
	"in": true, "notIn": true, "between": true, "notBetween": true,
}

// UnaryOperators is the set of operators that take no value (§3.4).
var UnaryOperators = map[string]bool{
	"null": true, "notNull": true,
}

// TernaryOperators is the set of operators that require exactly two values.
var TernaryOperators = map[string]bool{
	"between": true, "notBetween": true,
}

// PlaceholderName is the sentinel value used for incomplete field/operator/value
// slots (§2.8).
const PlaceholderName = "~"

// PlaceholderLabel is the display label for the field placeholder.
const PlaceholderLabel = "------"

// MatchThresholdPlaceholder is the sentinel value used in match threshold
// fields (§2.8).
const MatchThresholdPlaceholder = "#"

// DefaultJoinChar is the separator used when encoding multi-values as strings
// (§3.3).
const DefaultJoinChar = ","
