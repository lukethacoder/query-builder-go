package querybuilder

import (
	"fmt"
	"strings"
)

type sqlPresetEntry struct {
	quoteFieldNamesWith any    // string or [2]string
	fieldIdentifierSep  string
	concatOperator      string
	numberedParams      bool
	paramPrefix         string
	paramsKeepPrefix    bool
	quoteWith           string
}

var sqlPresets = map[string]sqlPresetEntry{
	"ansi":       {},
	"oracle":     {},
	"sqlite":     {paramsKeepPrefix: true},
	"mysql":      {concatOperator: "CONCAT"},
	"mssql":      {quoteFieldNamesWith: [2]string{"[", "]"}, fieldIdentifierSep: ".", concatOperator: "+", paramPrefix: "@"},
	"postgresql": {quoteFieldNamesWith: `"`, numberedParams: true, paramPrefix: "$"},
}

func applyPresetToSQL(opts SqlExportOptions) SqlExportOptions {
	if opts.Preset == "" {
		return opts
	}
	p, ok := sqlPresets[opts.Preset]
	if !ok {
		return opts
	}
	if opts.QuoteFieldNamesWith == nil && p.quoteFieldNamesWith != nil {
		opts.QuoteFieldNamesWith = p.quoteFieldNamesWith
	}
	if opts.FieldIdentifierSep == "" && p.fieldIdentifierSep != "" {
		opts.FieldIdentifierSep = p.fieldIdentifierSep
	}
	if opts.ConcatOperator == "" && p.concatOperator != "" {
		opts.ConcatOperator = p.concatOperator
	}
	return opts
}

func applyPresetToParam(opts ParameterizedExportOptions) ParameterizedExportOptions {
	if opts.Preset == "" {
		return opts
	}
	p, ok := sqlPresets[opts.Preset]
	if !ok {
		return opts
	}
	if opts.SqlExportOptions.QuoteFieldNamesWith == nil && p.quoteFieldNamesWith != nil {
		opts.SqlExportOptions.QuoteFieldNamesWith = p.quoteFieldNamesWith
	}
	if opts.SqlExportOptions.FieldIdentifierSep == "" && p.fieldIdentifierSep != "" {
		opts.SqlExportOptions.FieldIdentifierSep = p.fieldIdentifierSep
	}
	if opts.SqlExportOptions.ConcatOperator == "" && p.concatOperator != "" {
		opts.SqlExportOptions.ConcatOperator = p.concatOperator
	}
	if !opts.NumberedParams && p.numberedParams {
		opts.NumberedParams = true
	}
	if opts.ParamPrefix == "" && p.paramPrefix != "" {
		opts.ParamPrefix = p.paramPrefix
	}
	if !opts.ParamsKeepPrefix && p.paramsKeepPrefix {
		opts.ParamsKeepPrefix = true
	}
	return opts
}

func quoteFieldSQL(field string, opts SqlExportOptions) string {
	q := opts.QuoteFieldNamesWith
	sep := opts.FieldIdentifierSep
	if q == nil {
		return field
	}
	var pre, suf string
	switch v := q.(type) {
	case string:
		pre, suf = v, v
	case [2]string:
		pre, suf = v[0], v[1]
	default:
		return field
	}
	if sep != "" && strings.Contains(field, sep) {
		parts := strings.Split(field, sep)
		for i, p := range parts {
			parts[i] = pre + p + suf
		}
		return strings.Join(parts, sep)
	}
	return pre + field + suf
}

func quoteValueSQL(val string, opts SqlExportOptions) string {
	q := opts.QuoteValuesWith
	if q == "" {
		q = "'"
	}
	return q + strings.ReplaceAll(val, q, q+q) + q
}

func emitValueSQL(val any, opts SqlExportOptions) string {
	s := fmt.Sprintf("%v", val)
	parse := opts.CommonExportOptions.ParseNumbers != ""
	if parse && s != "" {
		var f float64
		if _, err := fmt.Sscanf(s, "%g", &f); err == nil {
			return s
		}
	}
	return quoteValueSQL(s, opts)
}

func isDroppedSQL(r *Rule, opts SqlExportOptions) bool {
	pf := opts.CommonExportOptions.PlaceholderFieldName
	po := opts.CommonExportOptions.PlaceholderOperatorName
	pv := opts.CommonExportOptions.PlaceholderValueName
	if pf == "" {
		pf = PlaceholderName
	}
	if po == "" {
		po = PlaceholderName
	}
	if pv == "" {
		pv = PlaceholderName
	}
	if r.Field == pf || r.Operator == po {
		return true
	}
	if !UnaryOperators[r.Operator] && fmt.Sprintf("%v", r.Value) == pv {
		return true
	}
	return false
}

func ruleToSQL(r *Rule, opts SqlExportOptions) string {
	qf := quoteFieldSQL(r.Field, opts)
	op := r.Operator
	val := r.Value

	if UnaryOperators[op] {
		if op == "null" {
			return qf + " is null"
		}
		return qf + " is not null"
	}

	if MultiValueOperators[op] {
		vals := ParseMultiValue(val, "")
		if op == "between" || op == "notBetween" {
			if len(vals) < 2 {
				return ""
			}
			lo, hi := vals[0], vals[1]
			if !opts.CommonExportOptions.PreserveValueOrder && opts.CommonExportOptions.ParseNumbers != "" {
				var a, b float64
				if _, err1 := fmt.Sscanf(lo, "%g", &a); err1 == nil {
					if _, err2 := fmt.Sscanf(hi, "%g", &b); err2 == nil {
						if a > b {
							lo, hi = hi, lo
						}
					}
				}
			}
			kw := "between"
			if op == "notBetween" {
				kw = "not between"
			}
			return fmt.Sprintf("%s %s %s and %s", qf, kw, emitValueSQL(lo, opts), emitValueSQL(hi, opts))
		}
		if len(vals) == 0 {
			return ""
		}
		parts := make([]string, len(vals))
		for i, v := range vals {
			parts[i] = emitValueSQL(v, opts)
		}
		kw := "in"
		if op == "notIn" {
			kw = "not in"
		}
		return fmt.Sprintf("%s %s (%s)", qf, kw, strings.Join(parts, ", "))
	}

	likeOps := map[string]string{
		"contains":         "LIKE",
		"doesNotContain":   "NOT LIKE",
		"beginsWith":       "LIKE",
		"doesNotBeginWith": "NOT LIKE",
		"endsWith":         "LIKE",
		"doesNotEndWith":   "NOT LIKE",
	}
	if sqlOp, ok := likeOps[op]; ok {
		rawVal := fmt.Sprintf("%v", val)
		var pattern string
		switch op {
		case "contains", "doesNotContain":
			pattern = "%" + rawVal + "%"
		case "beginsWith", "doesNotBeginWith":
			pattern = rawVal + "%"
		case "endsWith", "doesNotEndWith":
			pattern = "%" + rawVal
		}
		return fmt.Sprintf("%s %s %s", qf, sqlOp, quoteValueSQL(pattern, opts))
	}

	return fmt.Sprintf("%s %s %s", qf, op, emitValueSQL(val, opts))
}

func groupToSQL(g AnyNode, opts SqlExportOptions, vm ValidationMap) string {
	id := g.getCommon().ID
	if id != "" && vm != nil && !IsNodeValid(id, vm) {
		return "(1 = 1)"
	}

	var parts []string
	lastComb := "and"

	switch grp := g.(type) {
	case *RuleGroup:
		lastComb = grp.Combinator
		for _, child := range grp.Rules {
			clause := processSQLChild(child, opts, vm)
			if clause != "" {
				parts = append(parts, clause)
			}
		}
	case *RuleGroupIC:
		segments := joinICSQL(grp.Rules, opts, vm, &lastComb)
		if len(segments) == 0 {
			return "(1 = 1)"
		}
		combined := strings.Join(segments, " ")
		if grp.Not {
			return "NOT (" + combined + ")"
		}
		return combined
	}

	if len(parts) == 0 {
		return "(1 = 1)"
	}
	sep := " " + lastComb + " "
	joined := strings.Join(parts, sep)
	not := false
	if grp, ok := g.(*RuleGroup); ok {
		not = grp.Not
	}
	if not {
		return "NOT (" + joined + ")"
	}
	if len(parts) > 1 {
		return "(" + joined + ")"
	}
	return joined
}

func joinICSQL(rules []ICItem, opts SqlExportOptions, vm ValidationMap, lastComb *string) []string {
	var segments []string
	pending := ""
	for _, item := range rules {
		if item.isString {
			pending = item.Combinator
			continue
		}
		clause := processSQLChild(item.Node, opts, vm)
		if clause == "" {
			continue
		}
		if len(segments) > 0 && pending != "" {
			segments = append(segments, pending)
		}
		segments = append(segments, clause)
		if pending != "" {
			*lastComb = pending
		}
		pending = ""
	}
	return segments
}

func processSQLChild(n AnyNode, opts SqlExportOptions, vm ValidationMap) string {
	if IsRuleGroup(n) {
		return groupToSQL(n, opts, vm)
	}
	r, ok := n.(*Rule)
	if !ok {
		return ""
	}
	if isDroppedSQL(r, opts) {
		return ""
	}
	return ruleToSQL(r, opts)
}

// FormatSQL serializes the query as a SQL WHERE clause fragment (§7.4.1).
func FormatSQL(q AnyNode, opts SqlExportOptions) string {
	opts = applyPresetToSQL(opts)
	var vm ValidationMap
	if opts.CommonExportOptions.Validator != nil {
		_, m := opts.CommonExportOptions.Validator(q)
		vm = m
	}
	return groupToSQL(q, opts, vm)
}

// FormatParameterized serializes the query as positional-parameter SQL
// (§7.4.2).
func FormatParameterized(q AnyNode, opts ParameterizedExportOptions) ParameterizedResult {
	opts = applyPresetToParam(opts)
	params := make([]any, 0)
	var vm ValidationMap
	if opts.CommonExportOptions.Validator != nil {
		_, m := opts.CommonExportOptions.Validator(q)
		vm = m
	}
	sql := groupToSQLParam(q, opts, vm, &params, nil, nil)
	return ParameterizedResult{SQL: sql, Params: params}
}

// FormatParameterizedNamed serializes the query as named-parameter SQL
// (§7.4.2).
func FormatParameterizedNamed(q AnyNode, opts ParameterizedExportOptions) ParameterizedNamedResult {
	opts = applyPresetToParam(opts)
	params := make(map[string]any)
	counts := make(map[string]int)
	var vm ValidationMap
	if opts.CommonExportOptions.Validator != nil {
		_, m := opts.CommonExportOptions.Validator(q)
		vm = m
	}
	sql := groupToSQLParam(q, opts, vm, nil, params, counts)
	return ParameterizedNamedResult{SQL: sql, Params: params}
}

func addParam(value any, field string, opts ParameterizedExportOptions, arrParams *[]any, namedParams map[string]any, counts map[string]int) string {
	named := namedParams != nil
	prefix := opts.ParamPrefix
	if prefix == "" {
		if named {
			prefix = ":"
		} else {
			prefix = "?"
		}
	}
	if named && counts != nil {
		key := sanitizeParamKey(field)
		counts[key]++
		paramKey := fmt.Sprintf("%s_%d", key, counts[key])
		fullKey := paramKey
		if opts.ParamsKeepPrefix {
			fullKey = prefix + paramKey
		}
		namedParams[fullKey] = value
		return prefix + paramKey
	}
	*arrParams = append(*arrParams, value)
	if opts.NumberedParams {
		return fmt.Sprintf("%s%d", prefix, len(*arrParams))
	}
	return "?"
}

func sanitizeParamKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

func ruleToSQLParam(r *Rule, opts ParameterizedExportOptions, arrParams *[]any, namedParams map[string]any, counts map[string]int) string {
	sqlOpts := opts.SqlExportOptions
	qf := quoteFieldSQL(r.Field, sqlOpts)
	op := r.Operator
	val := r.Value

	if UnaryOperators[op] {
		if op == "null" {
			return qf + " is null"
		}
		return qf + " is not null"
	}

	if MultiValueOperators[op] {
		vals := ParseMultiValue(val, "")
		if op == "between" || op == "notBetween" {
			if len(vals) < 2 {
				return ""
			}
			ph1 := addParam(vals[0], r.Field, opts, arrParams, namedParams, counts)
			ph2 := addParam(vals[1], r.Field, opts, arrParams, namedParams, counts)
			kw := "between"
			if op == "notBetween" {
				kw = "not between"
			}
			return fmt.Sprintf("%s %s %s and %s", qf, kw, ph1, ph2)
		}
		if len(vals) == 0 {
			return ""
		}
		placeholders := make([]string, len(vals))
		for i, v := range vals {
			placeholders[i] = addParam(v, r.Field, opts, arrParams, namedParams, counts)
		}
		kw := "in"
		if op == "notIn" {
			kw = "not in"
		}
		return fmt.Sprintf("%s %s (%s)", qf, kw, strings.Join(placeholders, ", "))
	}

	likeOps := map[string]string{
		"contains":         "LIKE",
		"doesNotContain":   "NOT LIKE",
		"beginsWith":       "LIKE",
		"doesNotBeginWith": "NOT LIKE",
		"endsWith":         "LIKE",
		"doesNotEndWith":   "NOT LIKE",
	}
	if sqlOp, ok2 := likeOps[op]; ok2 {
		rawVal := fmt.Sprintf("%v", val)
		var pattern string
		switch op {
		case "contains", "doesNotContain":
			pattern = "%" + rawVal + "%"
		case "beginsWith", "doesNotBeginWith":
			pattern = rawVal + "%"
		case "endsWith", "doesNotEndWith":
			pattern = "%" + rawVal
		}
		ph := addParam(pattern, r.Field, opts, arrParams, namedParams, counts)
		return fmt.Sprintf("%s %s %s", qf, sqlOp, ph)
	}

	ph := addParam(val, r.Field, opts, arrParams, namedParams, counts)
	return fmt.Sprintf("%s %s %s", qf, op, ph)
}

func groupToSQLParam(g AnyNode, opts ParameterizedExportOptions, vm ValidationMap, arrParams *[]any, namedParams map[string]any, counts map[string]int) string {
	sqlOpts := opts.SqlExportOptions
	id := g.getCommon().ID
	if id != "" && vm != nil && !IsNodeValid(id, vm) {
		return "(1 = 1)"
	}

	var parts []string
	comb := "and"

	switch grp := g.(type) {
	case *RuleGroup:
		comb = grp.Combinator
		for _, child := range grp.Rules {
			if IsRuleGroup(child) {
				sub := groupToSQLParam(child, opts, vm, arrParams, namedParams, counts)
				if sub != "" {
					parts = append(parts, sub)
				}
			} else if r, ok := child.(*Rule); ok {
				if isDroppedSQL(r, sqlOpts) {
					continue
				}
				clause := ruleToSQLParam(r, opts, arrParams, namedParams, counts)
				if clause != "" {
					parts = append(parts, clause)
				}
			}
		}
	case *RuleGroupIC:
		for _, item := range grp.Rules {
			if item.isString {
				comb = item.Combinator
				continue
			}
			child := item.Node
			if IsRuleGroup(child) {
				sub := groupToSQLParam(child, opts, vm, arrParams, namedParams, counts)
				if sub != "" {
					parts = append(parts, sub)
				}
			} else if r, ok := child.(*Rule); ok {
				if isDroppedSQL(r, sqlOpts) {
					continue
				}
				clause := ruleToSQLParam(r, opts, arrParams, namedParams, counts)
				if clause != "" {
					parts = append(parts, clause)
				}
			}
		}
	}

	if len(parts) == 0 {
		return "(1 = 1)"
	}
	sep := " " + comb + " "
	joined := strings.Join(parts, sep)
	not := false
	switch grp := g.(type) {
	case *RuleGroup:
		not = grp.Not
	case *RuleGroupIC:
		not = grp.Not
	}
	if not {
		return "NOT (" + joined + ")"
	}
	if len(parts) > 1 {
		return "(" + joined + ")"
	}
	return joined
}
