// Package querybuilder implements the Query Builder specification — a
// language-neutral standard for representing, manipulating, validating, and
// serializing structured filter queries.
package querybuilder

// Path is an ordered list of indices that uniquely identifies a node in the
// query tree (§4.1).
type Path []int

// ValueSource controls how a rule's Value field is interpreted (§1.2).
type ValueSource string

const (
	ValueSourceValue ValueSource = "value"
	ValueSourceField ValueSource = "field"
)

// MatchMode names the counting mode for array/nested field sub-queries (§1.4).
type MatchMode string

const (
	MatchModeAll     MatchMode = "all"
	MatchModeSome    MatchMode = "some"
	MatchModeNone    MatchMode = "none"
	MatchModeAtLeast MatchMode = "atLeast"
	MatchModeAtMost  MatchMode = "atMost"
	MatchModeExactly MatchMode = "exactly"
)

// MatchConfig accompanies a Rule when filtering over array or nested fields
// (§1.4).
type MatchConfig struct {
	Mode      MatchMode `json:"mode"`
	Threshold *int      `json:"threshold,omitempty"`
}

// CommonProperties are the fields shared by every node type (§1.1).
type CommonProperties struct {
	ID       string `json:"id,omitempty"`
	Path     Path   `json:"path,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Rule is a single filter predicate (§1.3).
type Rule struct {
	CommonProperties
	Field       string      `json:"field"`
	Operator    string      `json:"operator"`
	Value       any `json:"value"`
	ValueSource ValueSource `json:"valueSource,omitempty"`
	Match       *MatchConfig `json:"match,omitempty"`
}

// RuleGroup is a standard group whose children share one combinator (§1.5).
type RuleGroup struct {
	CommonProperties
	Combinator string      `json:"combinator"`
	Rules      []AnyNode   `json:"rules"`
	Not        bool        `json:"not,omitempty"`
}

// RuleGroupIC is an independent-combinator group where combinators are
// interleaved between child nodes (§1.6).
type RuleGroupIC struct {
	CommonProperties
	// Combinator is intentionally absent; the field is never written to JSON.
	Rules []ICItem `json:"rules"`
	Not   bool     `json:"not,omitempty"`
}

// ICItem is an element of an IC group's rules array: either a child node or a
// combinator string.
type ICItem struct {
	// Exactly one of Node or Combinator is set.
	Node       AnyNode
	Combinator string
	isString   bool
}

// IsString reports whether this item holds a combinator string.
func (i ICItem) IsString() bool { return i.isString }

// StringItem constructs a combinator ICItem.
func StringItem(s string) ICItem { return ICItem{Combinator: s, isString: true} }

// NodeItem constructs a node ICItem.
func NodeItem(n AnyNode) ICItem { return ICItem{Node: n} }

// AnyNode is satisfied by *Rule, *RuleGroup, and *RuleGroupIC.
type AnyNode interface {
	nodeKind() string
	getCommon() *CommonProperties
}

func (r *Rule) nodeKind() string      { return "rule" }
func (r *RuleGroup) nodeKind() string  { return "group" }
func (r *RuleGroupIC) nodeKind() string { return "groupIC" }

func (r *Rule) getCommon() *CommonProperties      { return &r.CommonProperties }
func (r *RuleGroup) getCommon() *CommonProperties  { return &r.CommonProperties }
func (r *RuleGroupIC) getCommon() *CommonProperties { return &r.CommonProperties }

// IsRule reports whether n is a *Rule.
func IsRule(n AnyNode) bool { _, ok := n.(*Rule); return ok }

// IsRuleGroup reports whether n is a *RuleGroup or *RuleGroupIC.
func IsRuleGroup(n AnyNode) bool {
	switch n.(type) {
	case *RuleGroup, *RuleGroupIC:
		return true
	}
	return false
}

// IsStandardGroup reports whether n is a standard *RuleGroup.
func IsStandardGroup(n AnyNode) bool {
	_, ok := n.(*RuleGroup)
	return ok
}

// IsICGroup reports whether n is an independent-combinator *RuleGroupIC.
func IsICGroup(n AnyNode) bool {
	_, ok := n.(*RuleGroupIC)
	return ok
}

// IsICQuery reports whether the root group is an IC group.
func IsICQuery(q AnyNode) bool { return IsICGroup(q) }

// AnyRuleGroup is satisfied by *RuleGroup and *RuleGroupIC.
type AnyRuleGroup interface {
	AnyNode
	getRules() []AnyNode
	isIC() bool
}

func (g *RuleGroup) isIC() bool  { return false }
func (g *RuleGroupIC) isIC() bool { return true }

func (g *RuleGroup) getRules() []AnyNode {
	return g.Rules
}

func (g *RuleGroupIC) getRules() []AnyNode {
	nodes := make([]AnyNode, 0, len(g.Rules))
	for _, item := range g.Rules {
		if !item.isString {
			nodes = append(nodes, item.Node)
		}
	}
	return nodes
}

// --- Option types (§2.1) ---

// BaseOption is the minimal option shape with a display label.
type BaseOption struct {
	Name     string `json:"name,omitempty"`
	Value    string `json:"value,omitempty"`
	Label    string `json:"label"`
	Disabled bool   `json:"disabled,omitempty"`
}

// FullOption is a fully-resolved option with both Name and Value set.
type FullOption struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Label    string `json:"label"`
	Disabled bool   `json:"disabled,omitempty"`
}

// OptionGroup wraps a labeled group of options (§2.2).
type OptionGroup struct {
	Label   string       `json:"label"`
	Options []FullOption `json:"options"`
}

// OptionList holds either a flat slice of FullOption or a slice of OptionGroup.
// Only one of Flat or Groups should be non-nil.
type OptionList struct {
	Flat   []FullOption
	Groups []OptionGroup
}

// Operator extends FullOption with an optional arity hint (§2.5).
type Operator struct {
	FullOption
	Arity any `json:"arity,omitempty"` // int, "unary", "binary", "ternary", or nil
}

// Combinator is a plain FullOption used as a logical join keyword (§2.5).
type Combinator = FullOption

// ValueEditorType names the UI widget for editing rule values (§2.4).
type ValueEditorType string

const (
	ValueEditorText        ValueEditorType = "text"
	ValueEditorSelect      ValueEditorType = "select"
	ValueEditorMultiSelect ValueEditorType = "multiselect"
	ValueEditorCheckbox    ValueEditorType = "checkbox"
	ValueEditorRadio       ValueEditorType = "radio"
	ValueEditorTextarea    ValueEditorType = "textarea"
	ValueEditorSwitch      ValueEditorType = "switch"
)

// ValueSources lists the allowed value sources for a rule (§2.4).
type ValueSources []ValueSource

// Field describes a filterable attribute (§2.3).
type Field struct {
	FullOption
	Operators       *OptionList
	ValueEditorType ValueEditorType
	ValueSources    ValueSources
	InputType       string
	Values          *OptionList
	DefaultOperator string
	DefaultValue    any
	Placeholder     string
	Validator       RuleValidatorFunc
	MatchModes      *OptionList
	Subproperties   *OptionList
}

// --- Validation types (§6.1) ---

// ValidationResult holds the outcome of validating a single node.
type ValidationResult struct {
	Valid   bool
	Reasons []any
}

// ValidationMap maps node IDs to their validation outcomes.
type ValidationMap map[string]any // bool or ValidationResult

// QueryValidatorFunc validates an entire query tree.
type QueryValidatorFunc func(q AnyNode) (bool, ValidationMap)

// RuleValidatorFunc validates a single rule.
type RuleValidatorFunc func(r *Rule) (bool, *ValidationResult)

// --- Export option types (§7.1) ---

// ParseNumbers controls numeric coercion during export.
type ParseNumbers string

const (
	ParseNumbersBool     ParseNumbers = "bool"
	ParseNumbersStrict   ParseNumbers = "strict"
	ParseNumbersNative   ParseNumbers = "native"
	ParseNumbersEnhanced ParseNumbers = "enhanced"
)

// CommonExportOptions are options shared by all non-JSON export formats.
type CommonExportOptions struct {
	Fields                  *OptionList
	Validator               QueryValidatorFunc
	ParseNumbers            ParseNumbers
	PlaceholderFieldName    string
	PlaceholderOperatorName string
	PlaceholderValueName    string
	PreserveValueOrder      bool
	RuleProcessor           func(r *Rule, opts *CommonExportOptions) string
	OperatorProcessor       func(r *Rule, opts *CommonExportOptions) string
	ValueProcessor          func(r *Rule, opts *CommonExportOptions) string
	RuleGroupProcessor      func(g AnyRuleGroup, opts *CommonExportOptions) string
}

// SqlExportOptions are options for SQL export formats.
type SqlExportOptions struct {
	CommonExportOptions
	QuoteFieldNamesWith    any // string or [2]string
	FieldIdentifierSep     string
	QuoteValuesWith        string
	ConcatOperator         string
	Preset                 string // "ansi", "oracle", "sqlite", "mysql", "mssql", "postgresql"
}

// ParameterizedExportOptions extends SqlExportOptions for parameterized SQL.
type ParameterizedExportOptions struct {
	SqlExportOptions
	NumberedParams  bool
	ParamPrefix     string
	ParamsKeepPrefix bool
}

// ParameterizedResult is the output of FormatParameterized.
type ParameterizedResult struct {
	SQL    string
	Params []any
}

// ParameterizedNamedResult is the output of FormatParameterizedNamed.
type ParameterizedNamedResult struct {
	SQL    string
	Params map[string]any
}
