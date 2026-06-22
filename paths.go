package querybuilder

// GetParentPath returns the path of a node's parent (§4.6).
func GetParentPath(p Path) Path {
	if len(p) == 0 {
		return Path{}
	}
	out := make(Path, len(p)-1)
	copy(out, p)
	return out
}

// PathsAreEqual reports whether two paths are identical.
func PathsAreEqual(a, b Path) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// IsAncestor reports whether maybeAncestor is a strict ancestor of path.
func IsAncestor(maybeAncestor, path Path) bool {
	if len(maybeAncestor) >= len(path) {
		return false
	}
	for i, v := range maybeAncestor {
		if path[i] != v {
			return false
		}
	}
	return true
}

// GetCommonAncestorPath returns the longest common prefix of the parent paths
// of a and b (§4.6).
func GetCommonAncestorPath(a, b Path) Path {
	pa := GetParentPath(a)
	pb := GetParentPath(b)
	n := len(pa)
	if len(pb) < n {
		n = len(pb)
	}
	var common Path
	for i := 0; i < n; i++ {
		if pa[i] == pb[i] {
			common = append(common, pa[i])
		} else {
			break
		}
	}
	return common
}

// FindPath resolves a path against a query and returns the addressed node, or
// nil if the path does not exist (§4.2).
func FindPath(p Path, q AnyNode) AnyNode {
	if len(p) == 0 {
		return q
	}
	current := q
	for _, idx := range p {
		switch g := current.(type) {
		case *RuleGroup:
			if idx < 0 || idx >= len(g.Rules) {
				return nil
			}
			current = g.Rules[idx]
		case *RuleGroupIC:
			if idx < 0 || idx >= len(g.Rules) {
				return nil
			}
			item := g.Rules[idx]
			if item.isString {
				return nil // combinator slot — not an addressable node
			}
			current = item.Node
		default:
			return nil
		}
	}
	return current
}

// FindParent returns the immediate parent group of the node at path.
func FindParent(p Path, q AnyNode) AnyNode {
	if len(p) == 0 {
		return nil
	}
	return FindPath(GetParentPath(p), q)
}

// FindID performs a depth-first search for a node with the given ID (§4.6).
func FindID(id string, q AnyNode) AnyNode {
	if q == nil {
		return nil
	}
	if q.getCommon().ID == id {
		return q
	}
	switch g := q.(type) {
	case *RuleGroup:
		for _, child := range g.Rules {
			if found := FindID(id, child); found != nil {
				return found
			}
		}
	case *RuleGroupIC:
		for _, item := range g.Rules {
			if !item.isString {
				if found := FindID(id, item.Node); found != nil {
					return found
				}
			}
		}
	}
	return nil
}

// GetPathOfID returns the path to the node with the given ID, or nil if not
// found (§4.6).
func GetPathOfID(id string, q AnyNode) Path {
	if q == nil {
		return nil
	}
	if q.getCommon().ID == id {
		return Path{}
	}
	return searchPath(id, q, Path{})
}

func searchPath(id string, n AnyNode, base Path) Path {
	switch g := n.(type) {
	case *RuleGroup:
		for i, child := range g.Rules {
			childPath := append(append(Path{}, base...), i)
			if child.getCommon().ID == id {
				return childPath
			}
			if IsRuleGroup(child) {
				if found := searchPath(id, child, childPath); found != nil {
					return found
				}
			}
		}
	case *RuleGroupIC:
		for i, item := range g.Rules {
			if item.isString {
				continue
			}
			childPath := append(append(Path{}, base...), i)
			if item.Node.getCommon().ID == id {
				return childPath
			}
			if IsRuleGroup(item.Node) {
				if found := searchPath(id, item.Node, childPath); found != nil {
					return found
				}
			}
		}
	}
	return nil
}

// ResolvePath resolves a path-or-ID address to a concrete path (§5.1).
// pathOrID is either a Path ([]int) or a string ID.
func ResolvePath(pathOrID any, q AnyNode) Path {
	switch v := pathOrID.(type) {
	case string:
		return GetPathOfID(v, q)
	case Path:
		if FindPath(v, q) != nil {
			return v
		}
		return nil
	}
	return nil
}

// IsEffectivelyDisabled reports whether a node at path is disabled either
// directly or through inheritance (§4.5).
func IsEffectivelyDisabled(p Path, q AnyNode) bool {
	if q.getCommon().Disabled {
		return true
	}
	current := q
	for _, idx := range p {
		var child AnyNode
		switch g := current.(type) {
		case *RuleGroup:
			if idx < 0 || idx >= len(g.Rules) {
				return false
			}
			child = g.Rules[idx]
		case *RuleGroupIC:
			if idx < 0 || idx >= len(g.Rules) {
				return false
			}
			item := g.Rules[idx]
			if item.isString {
				return false
			}
			child = item.Node
		default:
			return false
		}
		if child.getCommon().Disabled {
			return true
		}
		current = child
	}
	return false
}

// AnnotatePaths recursively writes derived path values to every node (§4.1).
func AnnotatePaths(q AnyNode, base Path) AnyNode {
	if base == nil {
		base = Path{}
	}
	switch g := q.(type) {
	case *RuleGroup:
		cp := *g
		cp.Path = append(Path{}, base...)
		cp.Rules = make([]AnyNode, len(g.Rules))
		for i, child := range g.Rules {
			childPath := append(append(Path{}, base...), i)
			cp.Rules[i] = AnnotatePaths(child, childPath)
		}
		return &cp
	case *RuleGroupIC:
		cp := *g
		cp.Path = append(Path{}, base...)
		cp.Rules = make([]ICItem, len(g.Rules))
		for i, item := range g.Rules {
			if item.isString {
				cp.Rules[i] = item
			} else {
				childPath := append(append(Path{}, base...), i)
				cp.Rules[i] = NodeItem(AnnotatePaths(item.Node, childPath))
			}
		}
		return &cp
	case *Rule:
		cp := *g
		cp.Path = append(Path{}, base...)
		return &cp
	}
	return q
}

// StripPaths removes all path annotations from a query tree.
func StripPaths(q AnyNode) AnyNode {
	switch g := q.(type) {
	case *RuleGroup:
		cp := *g
		cp.Path = nil
		cp.Rules = make([]AnyNode, len(g.Rules))
		for i, child := range g.Rules {
			cp.Rules[i] = StripPaths(child)
		}
		return &cp
	case *RuleGroupIC:
		cp := *g
		cp.Path = nil
		cp.Rules = make([]ICItem, len(g.Rules))
		for i, item := range g.Rules {
			if item.isString {
				cp.Rules[i] = item
			} else {
				cp.Rules[i] = NodeItem(StripPaths(item.Node))
			}
		}
		return &cp
	case *Rule:
		cp := *g
		cp.Path = nil
		return &cp
	}
	return q
}
