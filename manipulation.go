package querybuilder

// AddOptions are options for the Add operation (§5.2).
type AddOptions struct {
	Combinators         *OptionList
	CombinatorPreceding string
	IDGenerator         func() string
}

// Add appends ruleOrGroup to the parent identified by parentPath (§5.2).
func Add(query AnyNode, ruleOrGroup AnyNode, parentPath any, opts AddOptions) AnyNode {
	idGen := opts.IDGenerator
	if idGen == nil {
		idGen = GenerateUUID
	}
	p := ResolvePath(parentPath, query)
	if p == nil {
		return query
	}
	parent := FindPath(p, query)
	if parent == nil || !IsRuleGroup(parent) {
		return query
	}
	prepared := PrepareNode(ruleOrGroup, idGen)

	switch g := parent.(type) {
	case *RuleGroup:
		newRules := append(append([]AnyNode{}, g.Rules...), prepared)
		newParent := &RuleGroup{CommonProperties: g.CommonProperties, Combinator: g.Combinator, Rules: newRules, Not: g.Not}
		return setAtPath(query, p, newParent)
	case *RuleGroupIC:
		if len(g.Rules) == 0 {
			newParent := &RuleGroupIC{CommonProperties: g.CommonProperties, Rules: []ICItem{NodeItem(prepared)}, Not: g.Not}
			return setAtPath(query, p, newParent)
		}
		comb := opts.CombinatorPreceding
		if comb == "" {
			// Prefer combinator already preceding last child
			if len(g.Rules) >= 2 {
				secondToLast := g.Rules[len(g.Rules)-2]
				if secondToLast.isString {
					comb = secondToLast.Combinator
				}
			}
		}
		if comb == "" {
			comb = pickCombinator(opts.Combinators)
		}
		newRules := append(append([]ICItem{}, g.Rules...), StringItem(comb), NodeItem(prepared))
		newParent := &RuleGroupIC{CommonProperties: g.CommonProperties, Rules: newRules, Not: g.Not}
		return setAtPath(query, p, newParent)
	}
	return query
}

// Remove deletes the node at path and (for IC groups) one adjacent combinator
// (§5.3).
func Remove(query AnyNode, path any) AnyNode {
	p := ResolvePath(path, query)
	if p == nil || len(p) == 0 {
		return query
	}
	index := p[len(p)-1]
	if IsICQuery(query) && index%2 != 0 {
		return query // combinator slot — not addressable
	}
	parentPath := GetParentPath(p)
	parent := FindPath(parentPath, query)
	if parent == nil || !IsRuleGroup(parent) {
		return query
	}
	switch g := parent.(type) {
	case *RuleGroup:
		newRules := spliceNodes(g.Rules, index, 1)
		newParent := &RuleGroup{CommonProperties: g.CommonProperties, Combinator: g.Combinator, Rules: newRules, Not: g.Not}
		return setAtPath(query, parentPath, newParent)
	case *RuleGroupIC:
		if len(g.Rules) > 1 {
			start := index - 1
			if index == 0 {
				start = 0
			}
			newRules := spliceICItems(g.Rules, start, 2)
			newParent := &RuleGroupIC{CommonProperties: g.CommonProperties, Rules: newRules, Not: g.Not}
			return setAtPath(query, parentPath, newParent)
		}
		newParent := &RuleGroupIC{CommonProperties: g.CommonProperties, Rules: nil, Not: g.Not}
		return setAtPath(query, parentPath, newParent)
	}
	return query
}

// UpdateOptions are options for the Update operation (§5.4).
type UpdateOptions struct {
	ResetOnFieldChange    bool // default true
	ResetOnOperatorChange bool
	GetRuleDefaultOperator func(field string) string
	GetValueSources        func(field, operator string) ValueSources
	GetRuleDefaultValue    func(r *Rule) any
	GetMatchModes          func(field string) *OptionList
	IDGenerator            func() string
}

// Update sets property to value on the node at path (§5.4).
func Update(query AnyNode, property string, value any, path any, opts UpdateOptions) AnyNode {
	if opts.IDGenerator == nil {
		opts.IDGenerator = GenerateUUID
	}
	if !opts.ResetOnFieldChange && property != "field" {
		// zero value for bool is false; spec default for resetOnFieldChange is true
		// Caller must explicitly set this if they want the non-default.
	}

	p := ResolvePath(path, query)
	if p == nil {
		return query
	}

	// IC combinator slot
	if IsICQuery(query) && property == "combinator" {
		if len(p) > 0 && p[len(p)-1]%2 != 0 {
			return setAtPathAny(query, p, value)
		}
		return query
	}

	node := FindPath(p, query)
	if node == nil {
		return query
	}

	// No-op if unchanged
	if getField(node, property) == value {
		return query
	}

	if IsRuleGroup(node) {
		return setAtPath(query, p, applyToGroup(node, property, value))
	}

	rule, ok := node.(*Rule)
	if !ok {
		return query
	}

	resetField := opts.ResetOnFieldChange
	// Default is true per spec
	if property != "field" {
		resetField = false
	}

	updated := ruleWithProperty(rule, property, value)

	if property == "field" {
		newField, _ := value.(string)
		matchModes := func() *OptionList {
			if opts.GetMatchModes != nil {
				return opts.GetMatchModes(newField)
			}
			return nil
		}()
		oldMatchModes := func() *OptionList {
			if opts.GetMatchModes != nil {
				return opts.GetMatchModes(rule.Field)
			}
			return nil
		}()
		forceReset := resetField || matchModes != nil || oldMatchModes != nil
		if matchModes == nil {
			updated.Match = nil
		}
		if forceReset {
			newOp := updated.Operator
			if opts.GetRuleDefaultOperator != nil {
				newOp = opts.GetRuleDefaultOperator(newField)
			}
			newVS := ValueSources{ValueSourceValue}
			if opts.GetValueSources != nil {
				newVS = opts.GetValueSources(newField, newOp)
			}
			newVSrc := ValueSourceValue
			if len(newVS) > 0 {
				newVSrc = newVS[0]
			}
			newVal := any("")
			if opts.GetRuleDefaultValue != nil {
				newVal = opts.GetRuleDefaultValue(updated)
			}
			updated.Operator = newOp
			updated.ValueSource = newVSrc
			updated.Value = newVal
		}
	} else if property == "operator" && opts.ResetOnOperatorChange {
		newOp, _ := value.(string)
		vSources := ValueSources{ValueSourceValue}
		if opts.GetValueSources != nil {
			vSources = opts.GetValueSources(rule.Field, newOp)
		}
		updated.ValueSource = ValueSourceValue
		if len(vSources) > 0 {
			updated.ValueSource = vSources[0]
		}
		updated.Value = ""
	} else if property == "valueSource" {
		newVal := any("")
		if opts.GetRuleDefaultValue != nil {
			newVal = opts.GetRuleDefaultValue(updated)
		}
		updated.Value = newVal
	}

	return setAtPath(query, p, updated)
}

// MoveOptions are options for the Move operation (§5.5).
type MoveOptions struct {
	Clone       bool
	Combinators *OptionList
	IDGenerator func() string
}

// Move relocates (or copies) the node at fromPath to toPath (§5.5).
// toPath may be a Path, an ID string, "up", or "down".
func Move(query AnyNode, fromPath, toPath any, opts MoveOptions) AnyNode {
	idGen := opts.IDGenerator
	if idGen == nil {
		idGen = GenerateUUID
	}
	resolvedFrom := ResolvePath(fromPath, query)
	if resolvedFrom == nil || len(resolvedFrom) == 0 {
		return query
	}
	var resolvedTo Path
	switch v := toPath.(type) {
	case string:
		switch v {
		case "up":
			p := resolveDirection("up", resolvedFrom, query)
			if p == nil {
				return query
			}
			resolvedTo = p
		case "down":
			p := resolveDirection("down", resolvedFrom, query)
			if p == nil {
				return query
			}
			resolvedTo = p
		default:
			p := GetPathOfID(v, query)
			if p == nil {
				return query
			}
			resolvedTo = p
		}
	case Path:
		if FindPath(v, query) == nil {
			return query
		}
		resolvedTo = v
	default:
		return query
	}

	if PathsAreEqual(resolvedFrom, resolvedTo) {
		return query
	}

	sourceNode := FindPath(resolvedFrom, query)
	if sourceNode == nil {
		return query
	}

	var movedNode AnyNode
	if opts.Clone {
		movedNode = RegenerateIDs(sourceNode, idGen)
	} else {
		movedNode = sourceNode
	}

	working := query
	if !opts.Clone {
		working = Remove(working, resolvedFrom)
	}

	adjustedTo := resolvedTo
	if !opts.Clone {
		common := GetCommonAncestorPath(resolvedFrom, resolvedTo)
		fromParent := GetParentPath(resolvedFrom)
		if PathsAreEqual(fromParent, common) {
			fromIdx := resolvedFrom[len(resolvedFrom)-1]
			toIdx := resolvedTo[len(resolvedTo)-1]
			if toIdx > fromIdx {
				stride := 1
				if IsICQuery(query) {
					stride = 2
				}
				adjustedTo = append(append(Path{}, resolvedTo[:len(resolvedTo)-1]...), toIdx-stride)
			}
		}
	}

	return insertAtPath(working, movedNode, adjustedTo, opts.Combinators, idGen)
}

// InsertOptions are options for the Insert operation (§5.6).
type InsertOptions struct {
	Combinators          *OptionList
	CombinatorPreceding  string
	CombinatorSucceeding string
	IDGenerator          func() string
	Replace              bool
}

// Insert places ruleOrGroup at the exact path (§5.6).
func Insert(query AnyNode, ruleOrGroup AnyNode, path any, opts InsertOptions) AnyNode {
	idGen := opts.IDGenerator
	if idGen == nil {
		idGen = GenerateUUID
	}
	p := ResolvePath(path, query)
	if p == nil {
		return query
	}
	parentPath := GetParentPath(p)
	parent := FindPath(parentPath, query)
	if parent == nil || !IsRuleGroup(parent) {
		return query
	}
	prepared := RegenerateIDs(ruleOrGroup, idGen)
	index := p[len(p)-1]

	switch g := parent.(type) {
	case *RuleGroup:
		var newRules []AnyNode
		if opts.Replace {
			newRules = spliceNodesInsert(g.Rules, index, 1, prepared)
		} else {
			newRules = spliceNodesInsert(g.Rules, index, 0, prepared)
		}
		newParent := &RuleGroup{CommonProperties: g.CommonProperties, Combinator: g.Combinator, Rules: newRules, Not: g.Not}
		return setAtPath(query, parentPath, newParent)
	case *RuleGroupIC:
		if opts.Replace {
			snapped := index + (index % 2)
			newRules := spliceICItemsInsert(g.Rules, snapped, 1, NodeItem(prepared))
			newParent := &RuleGroupIC{CommonProperties: g.CommonProperties, Rules: newRules, Not: g.Not}
			return setAtPath(query, parentPath, newParent)
		}
		if index == 0 {
			succ := opts.CombinatorSucceeding
			if succ == "" && len(g.Rules) > 1 && g.Rules[1].isString {
				succ = g.Rules[1].Combinator
			}
			if succ == "" {
				succ = opts.CombinatorPreceding
			}
			if succ == "" {
				succ = pickCombinator(opts.Combinators)
			}
			newRules := spliceICItemsInsert(g.Rules, 0, 0, NodeItem(prepared), StringItem(succ))
			newParent := &RuleGroupIC{CommonProperties: g.CommonProperties, Rules: newRules, Not: g.Not}
			return setAtPath(query, parentPath, newParent)
		}
		adjIndex := index - 1
		if index%2 == 0 {
			adjIndex = index - 1
		}
		prec := opts.CombinatorPreceding
		if prec == "" && adjIndex >= 0 && adjIndex < len(g.Rules) && g.Rules[adjIndex].isString {
			prec = g.Rules[adjIndex].Combinator
		}
		if prec == "" {
			prec = pickCombinator(opts.Combinators)
		}
		newRules := spliceICItemsInsert(g.Rules, adjIndex, 0, StringItem(prec), NodeItem(prepared))
		newParent := &RuleGroupIC{CommonProperties: g.CommonProperties, Rules: newRules, Not: g.Not}
		return setAtPath(query, parentPath, newParent)
	}
	return query
}

// GroupOptions are options for the Group operation (§5.7).
type GroupOptions struct {
	Clone       bool
	Combinators *OptionList
	IDGenerator func() string
}

// Group wraps two nodes into a new sub-group at targetPath (§5.7).
func Group(query AnyNode, sourcePath, targetPath any, opts GroupOptions) AnyNode {
	idGen := opts.IDGenerator
	if idGen == nil {
		idGen = GenerateUUID
	}
	resolvedSource := ResolvePath(sourcePath, query)
	resolvedTarget := ResolvePath(targetPath, query)
	if resolvedSource == nil || len(resolvedSource) == 0 {
		return query
	}
	if resolvedTarget == nil {
		return query
	}
	if PathsAreEqual(resolvedSource, resolvedTarget) {
		return query
	}
	targetNode := FindPath(resolvedTarget, query)
	sourceNode := FindPath(resolvedSource, query)
	if targetNode == nil || sourceNode == nil {
		return query
	}
	var movedSource AnyNode
	if opts.Clone {
		movedSource = RegenerateIDs(sourceNode, idGen)
	} else {
		movedSource = sourceNode
	}
	working := query
	if !opts.Clone {
		working = Remove(working, resolvedSource)
	}
	adjustedTarget := resolvedTarget
	if !opts.Clone {
		common := GetCommonAncestorPath(resolvedSource, resolvedTarget)
		srcParent := GetParentPath(resolvedSource)
		if PathsAreEqual(srcParent, common) {
			fromIdx := resolvedSource[len(resolvedSource)-1]
			toIdx := resolvedTarget[len(resolvedTarget)-1]
			if toIdx > fromIdx {
				stride := 1
				if IsICQuery(query) {
					stride = 2
				}
				adjustedTarget = append(append(Path{}, resolvedTarget[:len(resolvedTarget)-1]...), toIdx-stride)
			}
		}
	}
	comb := pickCombinator(opts.Combinators)
	// Re-fetch target node from working tree (index may have shifted)
	targetNode = FindPath(adjustedTarget, working)
	if targetNode == nil {
		return query
	}
	var newGroup AnyNode
	if IsICQuery(query) {
		newGroup = &RuleGroupIC{
			CommonProperties: CommonProperties{ID: idGen()},
			Rules:            []ICItem{NodeItem(targetNode), StringItem(comb), NodeItem(movedSource)},
		}
	} else {
		newGroup = &RuleGroup{
			CommonProperties: CommonProperties{ID: idGen()},
			Combinator:       comb,
			Rules:            []AnyNode{targetNode, movedSource},
		}
	}
	return setAtPath(working, adjustedTarget, newGroup)
}

// ConvertToIC converts a standard RuleGroup to an IC RuleGroupIC (§7.8).
func ConvertToIC(g *RuleGroup, defaultCombinator string) *RuleGroupIC {
	if defaultCombinator == "" {
		defaultCombinator = g.Combinator
	}
	var rules []ICItem
	for i, child := range g.Rules {
		if i > 0 {
			rules = append(rules, StringItem(defaultCombinator))
		}
		switch c := child.(type) {
		case *RuleGroup:
			rules = append(rules, NodeItem(ConvertToIC(c, "")))
		default:
			rules = append(rules, NodeItem(child))
		}
	}
	return &RuleGroupIC{CommonProperties: g.CommonProperties, Rules: rules, Not: g.Not}
}

// ConvertFromIC converts an IC RuleGroupIC to a standard RuleGroup (§7.8).
func ConvertFromIC(g *RuleGroupIC, defaultCombinator string) *RuleGroup {
	if defaultCombinator == "" {
		defaultCombinator = "and"
	}
	var rules []AnyNode
	comb := defaultCombinator
	for _, item := range g.Rules {
		if item.isString {
			comb = item.Combinator
		} else {
			switch c := item.Node.(type) {
			case *RuleGroupIC:
				rules = append(rules, ConvertFromIC(c, ""))
			default:
				rules = append(rules, c)
			}
		}
	}
	return &RuleGroup{CommonProperties: g.CommonProperties, Combinator: comb, Rules: rules, Not: g.Not}
}

// --- internal helpers ---

func pickCombinator(opts *OptionList) string {
	if opts != nil {
		if f := FirstOption(opts); f != "" {
			return f
		}
	}
	list := &OptionList{Flat: DefaultCombinators}
	if f := FirstOption(list); f != "" {
		return f
	}
	return "and"
}

func setAtPath(query AnyNode, p Path, replacement AnyNode) AnyNode {
	if len(p) == 0 {
		return replacement
	}
	parentPath := GetParentPath(p)
	index := p[len(p)-1]
	parent := FindPath(parentPath, query)
	if parent == nil {
		return query
	}
	switch g := parent.(type) {
	case *RuleGroup:
		if index < 0 || index >= len(g.Rules) {
			return query
		}
		newRules := make([]AnyNode, len(g.Rules))
		copy(newRules, g.Rules)
		newRules[index] = replacement
		newParent := &RuleGroup{CommonProperties: g.CommonProperties, Combinator: g.Combinator, Rules: newRules, Not: g.Not}
		return setAtPath(query, parentPath, newParent)
	case *RuleGroupIC:
		if index < 0 || index >= len(g.Rules) {
			return query
		}
		newRules := make([]ICItem, len(g.Rules))
		copy(newRules, g.Rules)
		newRules[index] = NodeItem(replacement)
		newParent := &RuleGroupIC{CommonProperties: g.CommonProperties, Rules: newRules, Not: g.Not}
		return setAtPath(query, parentPath, newParent)
	}
	return query
}

// setAtPath with a plain any (handles combinator string replacement in IC)
func setAtPathAny(query AnyNode, p Path, value any) AnyNode {
	if s, ok := value.(string); ok {
		// combinator string
		if len(p) == 0 {
			return query
		}
		parentPath := GetParentPath(p)
		index := p[len(p)-1]
		parent := FindPath(parentPath, query)
		if g, ok := parent.(*RuleGroupIC); ok {
			if index >= 0 && index < len(g.Rules) {
				newRules := make([]ICItem, len(g.Rules))
				copy(newRules, g.Rules)
				newRules[index] = StringItem(s)
				newParent := &RuleGroupIC{CommonProperties: g.CommonProperties, Rules: newRules, Not: g.Not}
				return setAtPath(query, parentPath, newParent)
			}
		}
		return query
	}
	if n, ok := value.(AnyNode); ok {
		return setAtPath(query, p, n)
	}
	return query
}

func insertAtPath(query AnyNode, node AnyNode, p Path, combinators *OptionList, idGen func() string) AnyNode {
	parentPath := GetParentPath(p)
	index := p[len(p)-1]
	parent := FindPath(parentPath, query)
	if parent == nil || !IsRuleGroup(parent) {
		return query
	}
	switch g := parent.(type) {
	case *RuleGroup:
		newRules := spliceNodesInsert(g.Rules, index, 0, node)
		newParent := &RuleGroup{CommonProperties: g.CommonProperties, Combinator: g.Combinator, Rules: newRules, Not: g.Not}
		return setAtPath(query, parentPath, newParent)
	case *RuleGroupIC:
		if len(g.Rules) == 0 {
			newParent := &RuleGroupIC{CommonProperties: g.CommonProperties, Rules: []ICItem{NodeItem(node)}, Not: g.Not}
			return setAtPath(query, parentPath, newParent)
		}
		if index == 0 {
			succ := ""
			if len(g.Rules) > 0 && g.Rules[0].isString {
				succ = g.Rules[0].Combinator
			}
			if succ == "" {
				succ = pickCombinator(combinators)
			}
			newRules := spliceICItemsInsert(g.Rules, 0, 0, NodeItem(node), StringItem(succ))
			newParent := &RuleGroupIC{CommonProperties: g.CommonProperties, Rules: newRules, Not: g.Not}
			return setAtPath(query, parentPath, newParent)
		}
		adjIndex := index
		if index%2 == 0 {
			adjIndex = index - 1
		}
		prec := ""
		if adjIndex > 0 && adjIndex-1 < len(g.Rules) && g.Rules[adjIndex-1].isString {
			prec = g.Rules[adjIndex-1].Combinator
		}
		if prec == "" {
			prec = pickCombinator(combinators)
		}
		newRules := spliceICItemsInsert(g.Rules, adjIndex, 0, StringItem(prec), NodeItem(node))
		newParent := &RuleGroupIC{CommonProperties: g.CommonProperties, Rules: newRules, Not: g.Not}
		return setAtPath(query, parentPath, newParent)
	}
	return query
}

func resolveDirection(dir string, p Path, query AnyNode) Path {
	stride := 1
	if IsICQuery(query) {
		stride = 2
	}
	index := p[len(p)-1]
	parentPath := GetParentPath(p)
	parent := FindPath(parentPath, query)
	if parent == nil || !IsRuleGroup(parent) {
		return nil
	}
	var rulesLen int
	switch g := parent.(type) {
	case *RuleGroup:
		rulesLen = len(g.Rules)
	case *RuleGroupIC:
		rulesLen = len(g.Rules)
	}
	if dir == "up" {
		if index-stride >= 0 {
			return append(append(Path{}, parentPath...), index-stride)
		}
		if len(parentPath) == 0 {
			return nil
		}
		return parentPath
	}
	if index+stride <= rulesLen-1 {
		return append(append(Path{}, parentPath...), index+stride)
	}
	return nil
}

func spliceNodes(s []AnyNode, start, deleteCount int) []AnyNode {
	out := make([]AnyNode, 0, len(s)-deleteCount)
	out = append(out, s[:start]...)
	out = append(out, s[start+deleteCount:]...)
	return out
}

func spliceNodesInsert(s []AnyNode, start, deleteCount int, items ...AnyNode) []AnyNode {
	out := make([]AnyNode, 0, len(s)-deleteCount+len(items))
	out = append(out, s[:start]...)
	out = append(out, items...)
	out = append(out, s[start+deleteCount:]...)
	return out
}

func spliceICItems(s []ICItem, start, deleteCount int) []ICItem {
	out := make([]ICItem, 0, len(s)-deleteCount)
	out = append(out, s[:start]...)
	end := start + deleteCount
	if end > len(s) {
		end = len(s)
	}
	out = append(out, s[end:]...)
	return out
}

func spliceICItemsInsert(s []ICItem, start, deleteCount int, items ...ICItem) []ICItem {
	out := make([]ICItem, 0, len(s)-deleteCount+len(items))
	out = append(out, s[:start]...)
	out = append(out, items...)
	end := start + deleteCount
	if end > len(s) {
		end = len(s)
	}
	out = append(out, s[end:]...)
	return out
}

func applyToGroup(n AnyNode, property string, value any) AnyNode {
	switch g := n.(type) {
	case *RuleGroup:
		cp := *g
		switch property {
		case "combinator":
			if s, ok := value.(string); ok {
				cp.Combinator = s
			}
		case "not":
			if b, ok := value.(bool); ok {
				cp.Not = b
			}
		case "disabled":
			if b, ok := value.(bool); ok {
				cp.Disabled = b
			}
		case "id":
			if s, ok := value.(string); ok {
				cp.ID = s
			}
		}
		return &cp
	case *RuleGroupIC:
		cp := *g
		switch property {
		case "not":
			if b, ok := value.(bool); ok {
				cp.Not = b
			}
		case "disabled":
			if b, ok := value.(bool); ok {
				cp.Disabled = b
			}
		case "id":
			if s, ok := value.(string); ok {
				cp.ID = s
			}
		}
		return &cp
	}
	return n
}

func ruleWithProperty(r *Rule, property string, value any) *Rule {
	cp := *r
	switch property {
	case "field":
		if s, ok := value.(string); ok {
			cp.Field = s
		}
	case "operator":
		if s, ok := value.(string); ok {
			cp.Operator = s
		}
	case "value":
		cp.Value = value
	case "valueSource":
		if s, ok := value.(ValueSource); ok {
			cp.ValueSource = s
		} else if s, ok := value.(string); ok {
			cp.ValueSource = ValueSource(s)
		}
	case "disabled":
		if b, ok := value.(bool); ok {
			cp.Disabled = b
		}
	case "id":
		if s, ok := value.(string); ok {
			cp.ID = s
		}
	}
	return &cp
}

func getField(n AnyNode, property string) any {
	switch v := n.(type) {
	case *Rule:
		switch property {
		case "field":
			return v.Field
		case "operator":
			return v.Operator
		case "value":
			return v.Value
		case "valueSource":
			return v.ValueSource
		case "disabled":
			return v.Disabled
		case "id":
			return v.ID
		}
	case *RuleGroup:
		switch property {
		case "combinator":
			return v.Combinator
		case "not":
			return v.Not
		case "disabled":
			return v.Disabled
		case "id":
			return v.ID
		}
	case *RuleGroupIC:
		switch property {
		case "not":
			return v.Not
		case "disabled":
			return v.Disabled
		case "id":
			return v.ID
		}
	}
	return nil
}
