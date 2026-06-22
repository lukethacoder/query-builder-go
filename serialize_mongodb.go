package querybuilder

import "fmt"

// FormatMongoDBQuery converts the query to a MongoDB filter document (§7.4.5).
// The returned value is a map[string]any suitable for use with the MongoDB Go
// driver's bson/primitive helpers or json.Marshal.
func FormatMongoDBQuery(q AnyNode, opts CommonExportOptions) map[string]any {
	return groupToMongo(q, opts)
}

func isDroppedMongo(r *Rule, opts CommonExportOptions) bool {
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

func ruleToMongo(r *Rule) map[string]any {
	field := r.Field
	op := r.Operator
	val := r.Value

	if UnaryOperators[op] {
		if op == "null" {
			return map[string]any{field: nil}
		}
		return map[string]any{field: map[string]any{"$exists": true, "$ne": nil}}
	}

	mongoOps := map[string]string{
		"!=": "$ne",
		"<":  "$lt",
		">":  "$gt",
		"<=": "$lte",
		">=": "$gte",
	}

	if op == "=" {
		return map[string]any{field: val}
	}
	if mop, ok := mongoOps[op]; ok {
		return map[string]any{field: map[string]any{mop: val}}
	}

	if op == "in" || op == "notIn" {
		vals := ParseMultiValue(val, "")
		key := "$in"
		if op == "notIn" {
			key = "$nin"
		}
		return map[string]any{field: map[string]any{key: vals}}
	}

	if op == "between" || op == "notBetween" {
		vals := ParseMultiValue(val, "")
		if len(vals) < 2 {
			return nil
		}
		lo, hi := vals[0], vals[1]
		if op == "between" {
			return map[string]any{field: map[string]any{"$gte": lo, "$lte": hi}}
		}
		return map[string]any{"$or": []any{
			map[string]any{field: map[string]any{"$lt": lo}},
			map[string]any{field: map[string]any{"$gt": hi}},
		}}
	}

	sv := fmt.Sprintf("%v", val)
	switch op {
	case "contains":
		return map[string]any{field: map[string]any{"$regex": sv}}
	case "doesNotContain":
		return map[string]any{field: map[string]any{"$not": map[string]any{"$regex": sv}}}
	case "beginsWith":
		return map[string]any{field: map[string]any{"$regex": "^" + sv}}
	case "doesNotBeginWith":
		return map[string]any{field: map[string]any{"$not": map[string]any{"$regex": "^" + sv}}}
	case "endsWith":
		return map[string]any{field: map[string]any{"$regex": sv + "$"}}
	case "doesNotEndWith":
		return map[string]any{field: map[string]any{"$not": map[string]any{"$regex": sv + "$"}}}
	}

	return map[string]any{field: val}
}

func groupToMongo(g AnyNode, opts CommonExportOptions) map[string]any {
	var children []any
	comb := "and"

	switch grp := g.(type) {
	case *RuleGroup:
		comb = grp.Combinator
		for _, child := range grp.Rules {
			if IsRuleGroup(child) {
				children = append(children, groupToMongo(child, opts))
			} else if r, ok := child.(*Rule); ok {
				if isDroppedMongo(r, opts) {
					continue
				}
				doc := ruleToMongo(r)
				if doc != nil {
					children = append(children, doc)
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
				children = append(children, groupToMongo(item.Node, opts))
			} else if r, ok := item.Node.(*Rule); ok {
				if isDroppedMongo(r, opts) {
					continue
				}
				doc := ruleToMongo(r)
				if doc != nil {
					children = append(children, doc)
				}
			}
		}
	}

	if len(children) == 0 {
		return map[string]any{"$expr": true}
	}

	mongoKey := "$and"
	if comb == "or" {
		mongoKey = "$or"
	}
	combined := map[string]any{mongoKey: children}

	not := false
	switch grp := g.(type) {
	case *RuleGroup:
		not = grp.Not
	case *RuleGroupIC:
		not = grp.Not
	}
	if not {
		return map[string]any{"$nor": []any{combined}}
	}
	return combined
}
