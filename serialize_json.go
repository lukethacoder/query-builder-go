package querybuilder

import (
	"encoding/json"
)

// FormatJSON serializes the query as pretty-printed JSON with IDs and paths
// preserved (§7.2).
func FormatJSON(q AnyNode) (string, error) {
	b, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// FormatJSONWithoutIDs serializes the query as compact JSON with all id and
// path fields stripped (§7.2).
func FormatJSONWithoutIDs(q AnyNode) (string, error) {
	stripped := stripIDs(StripPaths(q))
	b, err := json.Marshal(stripped)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func stripIDs(n AnyNode) AnyNode {
	switch g := n.(type) {
	case *RuleGroup:
		cp := *g
		cp.ID = ""
		cp.Rules = make([]AnyNode, len(g.Rules))
		for i, child := range g.Rules {
			cp.Rules[i] = stripIDs(child)
		}
		return &cp
	case *RuleGroupIC:
		cp := *g
		cp.ID = ""
		cp.Rules = make([]ICItem, len(g.Rules))
		for i, item := range g.Rules {
			if item.isString {
				cp.Rules[i] = item
			} else {
				cp.Rules[i] = NodeItem(stripIDs(item.Node))
			}
		}
		return &cp
	case *Rule:
		cp := *g
		cp.ID = ""
		return &cp
	}
	return n
}

// ParseJSON deserializes a query from its JSON representation. The returned
// node is either *RuleGroup or *RuleGroupIC.
func ParseJSON(data string) (AnyNode, error) {
	return unmarshalNode([]byte(data))
}
