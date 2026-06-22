package querybuilder

import (
	"crypto/rand"
	"fmt"
)

// GenerateUUID produces a version-4 UUID (§5.8).
func GenerateUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// PrepareNode assigns IDs to any node (or descendant) that lacks one
// ("fill-if-missing", §5.8).
func PrepareNode(n AnyNode, idGen func() string) AnyNode {
	if idGen == nil {
		idGen = GenerateUUID
	}
	return prepareNode(n, idGen)
}

func prepareNode(n AnyNode, idGen func() string) AnyNode {
	switch v := n.(type) {
	case *Rule:
		if v.ID == "" {
			cp := *v
			cp.ID = idGen()
			return &cp
		}
		return v
	case *RuleGroup:
		cp := *v
		if cp.ID == "" {
			cp.ID = idGen()
		}
		cp.Rules = prepareNodes(v.Rules, idGen)
		return &cp
	case *RuleGroupIC:
		cp := *v
		if cp.ID == "" {
			cp.ID = idGen()
		}
		cp.Rules = prepareICItems(v.Rules, idGen)
		return &cp
	}
	return n
}

func prepareNodes(nodes []AnyNode, idGen func() string) []AnyNode {
	out := make([]AnyNode, len(nodes))
	for i, n := range nodes {
		out[i] = prepareNode(n, idGen)
	}
	return out
}

func prepareICItems(items []ICItem, idGen func() string) []ICItem {
	out := make([]ICItem, len(items))
	for i, it := range items {
		if it.isString {
			out[i] = it
		} else {
			out[i] = NodeItem(prepareNode(it.Node, idGen))
		}
	}
	return out
}

// RegenerateIDs assigns fresh IDs to a node and all its descendants
// ("regenerate", §5.8).
func RegenerateIDs(n AnyNode, idGen func() string) AnyNode {
	if idGen == nil {
		idGen = GenerateUUID
	}
	return regenerateIDs(n, idGen)
}

func regenerateIDs(n AnyNode, idGen func() string) AnyNode {
	switch v := n.(type) {
	case *Rule:
		cp := *v
		cp.ID = idGen()
		return &cp
	case *RuleGroup:
		cp := *v
		cp.ID = idGen()
		cp.Rules = make([]AnyNode, len(v.Rules))
		for i, child := range v.Rules {
			cp.Rules[i] = regenerateIDs(child, idGen)
		}
		return &cp
	case *RuleGroupIC:
		cp := *v
		cp.ID = idGen()
		cp.Rules = make([]ICItem, len(v.Rules))
		for i, it := range v.Rules {
			if it.isString {
				cp.Rules[i] = it
			} else {
				cp.Rules[i] = NodeItem(regenerateIDs(it.Node, idGen))
			}
		}
		return &cp
	}
	return n
}
