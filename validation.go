package querybuilder

// DefaultValidator walks the query tree and reports structural problems in
// groups (§6.3).
func DefaultValidator(query AnyNode) ValidationMap {
	m := ValidationMap{}
	validateGroup(query, m)
	return m
}

func validateGroup(n AnyNode, m ValidationMap) {
	id := n.getCommon().ID
	if id != "" {
		m[id] = validateGroupNode(n)
	}
	switch g := n.(type) {
	case *RuleGroup:
		for _, child := range g.Rules {
			if IsRuleGroup(child) {
				validateGroup(child, m)
			}
		}
	case *RuleGroupIC:
		for _, item := range g.Rules {
			if !item.isString && IsRuleGroup(item.Node) {
				validateGroup(item.Node, m)
			}
		}
	}
}

func validateGroupNode(n AnyNode) ValidationResult {
	var reasons []any

	switch g := n.(type) {
	case *RuleGroup:
		if len(g.Rules) == 0 {
			reasons = append(reasons, "empty")
		}
		if len(g.Rules) > 1 {
			known := false
			for _, c := range DefaultCombinators {
				if c.Value == g.Combinator {
					known = true
					break
				}
			}
			if !known {
				reasons = append(reasons, "invalidCombinator")
			}
		}
	case *RuleGroupIC:
		items := g.Rules
		invalid := false
		for i, item := range items {
			expectedNode := i%2 == 0
			if expectedNode && item.isString {
				invalid = true
				break
			}
			if !expectedNode && !item.isString {
				invalid = true
				break
			}
		}
		if len(items) > 0 && len(items)%2 == 0 {
			invalid = true
		}
		if invalid {
			reasons = append(reasons, "invalidIndependentCombinators")
		}
		if len(items) == 0 {
			reasons = append(reasons, "empty")
		}
	}

	if len(reasons) > 0 {
		return ValidationResult{Valid: false, Reasons: reasons}
	}
	return ValidationResult{Valid: true}
}

// MergeValidationMaps combines multiple validation results. A node is invalid
// if any source marks it invalid (§6.2).
func MergeValidationMaps(maps ...any) any {
	merged := ValidationMap{}
	allBool := true
	boolResult := true

	for _, m := range maps {
		switch v := m.(type) {
		case bool:
			if !v {
				boolResult = false
			}
		case ValidationMap:
			allBool = false
			for id, result := range v {
				existing, exists := merged[id]
				if !exists {
					merged[id] = result
					continue
				}
				existingValid := nodeResultValid(existing)
				newValid := nodeResultValid(result)
				if !existingValid || !newValid {
					existingReasons := nodeResultReasons(existing)
					newReasons := nodeResultReasons(result)
					merged[id] = ValidationResult{
						Valid:   false,
						Reasons: append(existingReasons, newReasons...),
					}
				}
			}
		}
	}

	if allBool {
		return boolResult
	}
	return merged
}

func nodeResultValid(v any) bool {
	switch r := v.(type) {
	case bool:
		return r
	case ValidationResult:
		return r.Valid
	}
	return true
}

func nodeResultReasons(v any) []any {
	if r, ok := v.(ValidationResult); ok {
		return r.Reasons
	}
	return nil
}

// IsNodeValid looks up a node's validity in a validation map (§6.4).
func IsNodeValid(id string, m any) bool {
	if m == nil {
		return true
	}
	switch v := m.(type) {
	case bool:
		return v
	case ValidationMap:
		result, ok := v[id]
		if !ok {
			return true
		}
		return nodeResultValid(result)
	}
	return true
}

// NormalizeValidationResult converts a bool or ValidationResult to a
// ValidationResult.
func NormalizeValidationResult(v any) ValidationResult {
	switch r := v.(type) {
	case bool:
		return ValidationResult{Valid: r}
	case ValidationResult:
		return r
	}
	return ValidationResult{Valid: true}
}
