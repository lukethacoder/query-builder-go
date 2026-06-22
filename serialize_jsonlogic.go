package querybuilder

import "fmt"

// FormatJSONLogic converts the query to a JsonLogic rule object (§7.4.6).
func FormatJSONLogic(q AnyNode, opts CommonExportOptions) map[string]any {
	return groupToJSONLogic(q, opts)
}

func isDroppedJL(r *Rule, opts CommonExportOptions) bool {
	pf := opts.PlaceholderFieldName
	po := opts.PlaceholderOperatorName
	pv := opts.PlaceholderValueName
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

func jlVar(name string) map[string]any {
	return map[string]any{"var": name}
}

func ruleToJSONLogic(r *Rule) map[string]any {
	f := jlVar(r.Field)
	op := r.Operator
	val := r.Value

	opMap := map[string]string{
		"=":  "==",
		"!=": "!=",
		"<":  "<",
		">":  ">",
		"<=": "<=",
		">=": ">=",
	}
	if jlOp, ok := opMap[op]; ok {
		return map[string]any{jlOp: []any{f, val}}
	}

	if UnaryOperators[op] {
		if op == "null" {
			return map[string]any{"==": []any{f, nil}}
		}
		return map[string]any{"!=": []any{f, nil}}
	}

	if op == "in" || op == "notIn" {
		vals := ParseMultiValue(val, "")
		logic := map[string]any{"in": []any{f, vals}}
		if op == "notIn" {
			return map[string]any{"!": logic}
		}
		return logic
	}

	if op == "between" || op == "notBetween" {
		vals := ParseMultiValue(val, "")
		if len(vals) < 2 {
			return nil
		}
		logic := map[string]any{"<=": []any{vals[0], f, vals[1]}}
		if op == "notBetween" {
			return map[string]any{"!": logic}
		}
		return logic
	}

	sv := fmt.Sprintf("%v", val)
	switch op {
	case "contains":
		return map[string]any{"in": []any{sv, jlVar(r.Field)}}
	case "doesNotContain":
		return map[string]any{"!": map[string]any{"in": []any{sv, f}}}
	case "beginsWith":
		return map[string]any{"startsWith": []any{f, sv}}
	case "doesNotBeginWith":
		return map[string]any{"!": map[string]any{"startsWith": []any{f, sv}}}
	case "endsWith":
		return map[string]any{"endsWith": []any{f, sv}}
	case "doesNotEndWith":
		return map[string]any{"!": map[string]any{"endsWith": []any{f, sv}}}
	case "matchesRegex":
		return map[string]any{"matchesRegex": []any{f, sv}}
	}

	return map[string]any{"==": []any{f, val}}
}

func groupToJSONLogic(g AnyNode, opts CommonExportOptions) map[string]any {
	var children []any
	comb := "and"

	switch grp := g.(type) {
	case *RuleGroup:
		comb = grp.Combinator
		for _, child := range grp.Rules {
			if IsRuleGroup(child) {
				children = append(children, groupToJSONLogic(child, opts))
			} else if r, ok := child.(*Rule); ok {
				if isDroppedJL(r, opts) {
					continue
				}
				node := ruleToJSONLogic(r)
				if node != nil {
					children = append(children, node)
				}
			}
		}
	case *RuleGroupIC:
		for _, item := range grp.Rules {
			if item.isString {
				comb = item.Combinator
				continue
			}
			if IsRuleGroup(item.Node) {
				children = append(children, groupToJSONLogic(item.Node, opts))
			} else if r, ok := item.Node.(*Rule); ok {
				if isDroppedJL(r, opts) {
					continue
				}
				node := ruleToJSONLogic(r)
				if node != nil {
					children = append(children, node)
				}
			}
		}
	}

	if len(children) == 0 {
		return map[string]any{"==": []any{1, 1}}
	}

	jlKey := "and"
	if comb == "or" {
		jlKey = "or"
	}
	combined := map[string]any{jlKey: children}

	not := false
	switch grp := g.(type) {
	case *RuleGroup:
		not = grp.Not
	case *RuleGroupIC:
		not = grp.Not
	}
	if not {
		return map[string]any{"!": combined}
	}
	return combined
}
