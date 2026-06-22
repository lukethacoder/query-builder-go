package querybuilder

import (
	"encoding/json"
	"fmt"
)

// --- JSON marshaling for ICItem ---

func (i ICItem) MarshalJSON() ([]byte, error) {
	if i.isString {
		return json.Marshal(i.Combinator)
	}
	return marshalNode(i.Node)
}

func (i *ICItem) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		i.Combinator = s
		i.isString = true
		return nil
	}
	node, err := unmarshalNode(data)
	if err != nil {
		return err
	}
	i.Node = node
	return nil
}

// --- JSON marshaling for AnyNode ---

func marshalNode(n AnyNode) ([]byte, error) {
	if n == nil {
		return []byte("null"), nil
	}
	return json.Marshal(n)
}

// wireGroup is used to detect whether a JSON object is a group (has "rules").
type wireGroup struct {
	Rules      json.RawMessage `json:"rules"`
	Combinator *string         `json:"combinator"`
}

func unmarshalNode(data []byte) (AnyNode, error) {
	// Peek for "rules" key to distinguish group from rule
	var probe wireGroup
	if err := json.Unmarshal(data, &probe); err == nil && probe.Rules != nil {
		if probe.Combinator != nil {
			// Standard group
			var g RuleGroup
			if err := unmarshalRuleGroup(data, &g); err != nil {
				return nil, err
			}
			return &g, nil
		}
		// IC group
		var g RuleGroupIC
		if err := unmarshalRuleGroupIC(data, &g); err != nil {
			return nil, err
		}
		return &g, nil
	}
	// Rule
	var r Rule
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("querybuilder: cannot unmarshal node: %w", err)
	}
	return &r, nil
}

// wireRuleGroup is the raw JSON shape for RuleGroup, with rules as raw messages.
type wireRuleGroup struct {
	CommonProperties
	Combinator string            `json:"combinator"`
	Rules      []json.RawMessage `json:"rules"`
	Not        bool              `json:"not,omitempty"`
}

func unmarshalRuleGroup(data []byte, g *RuleGroup) error {
	var w wireRuleGroup
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	g.CommonProperties = w.CommonProperties
	g.Combinator = w.Combinator
	g.Not = w.Not
	g.Rules = make([]AnyNode, 0, len(w.Rules))
	for _, raw := range w.Rules {
		node, err := unmarshalNode(raw)
		if err != nil {
			return err
		}
		g.Rules = append(g.Rules, node)
	}
	return nil
}

// wireRuleGroupIC is the raw JSON shape for RuleGroupIC.
type wireRuleGroupIC struct {
	CommonProperties
	Rules []json.RawMessage `json:"rules"`
	Not   bool              `json:"not,omitempty"`
}

func unmarshalRuleGroupIC(data []byte, g *RuleGroupIC) error {
	var w wireRuleGroupIC
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	g.CommonProperties = w.CommonProperties
	g.Not = w.Not
	g.Rules = make([]ICItem, 0, len(w.Rules))
	for _, raw := range w.Rules {
		var item ICItem
		if err := item.UnmarshalJSON(raw); err != nil {
			return err
		}
		g.Rules = append(g.Rules, item)
	}
	return nil
}

// MarshalJSON renders RuleGroup to JSON.
func (g RuleGroup) MarshalJSON() ([]byte, error) {
	type alias RuleGroup
	// We need to custom-marshal Rules so that AnyNode elements serialize correctly.
	type wire struct {
		ID         string            `json:"id,omitempty"`
		Path       Path              `json:"path,omitempty"`
		Disabled   bool              `json:"disabled,omitempty"`
		Combinator string            `json:"combinator"`
		Rules      []json.RawMessage `json:"rules"`
		Not        bool              `json:"not,omitempty"`
	}
	_ = alias{}
	rules := make([]json.RawMessage, len(g.Rules))
	for i, n := range g.Rules {
		b, err := marshalNode(n)
		if err != nil {
			return nil, err
		}
		rules[i] = b
	}
	return json.Marshal(wire{
		ID:         g.ID,
		Path:       g.Path,
		Disabled:   g.Disabled,
		Combinator: g.Combinator,
		Rules:      rules,
		Not:        g.Not,
	})
}

// UnmarshalJSON deserializes RuleGroup from JSON.
func (g *RuleGroup) UnmarshalJSON(data []byte) error {
	return unmarshalRuleGroup(data, g)
}

// UnmarshalJSON deserializes RuleGroupIC from JSON.
func (g *RuleGroupIC) UnmarshalJSON(data []byte) error {
	return unmarshalRuleGroupIC(data, g)
}

// MarshalJSON renders RuleGroupIC to JSON.
func (g RuleGroupIC) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID       string            `json:"id,omitempty"`
		Path     Path              `json:"path,omitempty"`
		Disabled bool              `json:"disabled,omitempty"`
		Rules    []json.RawMessage `json:"rules"`
		Not      bool              `json:"not,omitempty"`
	}
	items := make([]json.RawMessage, len(g.Rules))
	for i, it := range g.Rules {
		b, err := it.MarshalJSON()
		if err != nil {
			return nil, err
		}
		items[i] = b
	}
	return json.Marshal(wire{
		ID:       g.ID,
		Path:     g.Path,
		Disabled: g.Disabled,
		Rules:    items,
		Not:      g.Not,
	})
}
