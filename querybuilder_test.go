package querybuilder_test

import (
	"encoding/json"
	"strings"
	"testing"

	qb "github.com/lukethacoder/query-builder-go"
)

// --- helpers ---

func newGroup(comb string, rules ...qb.AnyNode) *qb.RuleGroup {
	return &qb.RuleGroup{Combinator: comb, Rules: rules}
}

func newRule(field, op string, val any) *qb.Rule {
	return &qb.Rule{Field: field, Operator: op, Value: val}
}

// --- types and JSON round-trip ---

func TestJSONRoundTrip(t *testing.T) {
	q := &qb.RuleGroup{
		CommonProperties: qb.CommonProperties{ID: "root"},
		Combinator:       "and",
		Rules: []qb.AnyNode{
			&qb.Rule{Field: "firstName", Operator: "beginsWith", Value: "Stev"},
			&qb.RuleGroup{
				Combinator: "or",
				Not:        true,
				Rules: []qb.AnyNode{
					&qb.Rule{Field: "age", Operator: "between", Value: "26,52"},
					&qb.Rule{Field: "city", Operator: "in", Value: "London,Paris,Tokyo"},
				},
			},
		},
	}

	out, err := qb.FormatJSON(q)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := qb.ParseJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	out2, _ := qb.FormatJSON(parsed)
	if out != out2 {
		t.Errorf("round-trip mismatch:\n%s\n\nvs\n\n%s", out, out2)
	}
}

func TestFormatJSONWithoutIDs(t *testing.T) {
	q := &qb.RuleGroup{
		CommonProperties: qb.CommonProperties{ID: "root"},
		Combinator:       "and",
		Rules: []qb.AnyNode{
			&qb.Rule{CommonProperties: qb.CommonProperties{ID: "r1"}, Field: "name", Operator: "=", Value: "Alice"},
		},
	}
	out, err := qb.FormatJSONWithoutIDs(q)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, `"id"`) {
		t.Errorf("expected no id fields, got: %s", out)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Errorf("invalid JSON: %v", err)
	}
}

// --- paths ---

func TestFindPath(t *testing.T) {
	child := newRule("age", ">", 30)
	q := newGroup("and", newRule("name", "=", "Alice"), child)

	found := qb.FindPath(qb.Path{1}, q)
	if found != child {
		t.Errorf("expected child rule, got %v", found)
	}
	if qb.FindPath(qb.Path{5}, q) != nil {
		t.Error("expected nil for out-of-range path")
	}
}

func TestGetPathOfID(t *testing.T) {
	child := &qb.Rule{CommonProperties: qb.CommonProperties{ID: "target"}, Field: "x", Operator: "=", Value: 1}
	q := &qb.RuleGroup{
		CommonProperties: qb.CommonProperties{ID: "root"},
		Combinator:       "and",
		Rules:            []qb.AnyNode{child},
	}
	p := qb.GetPathOfID("target", q)
	if len(p) != 1 || p[0] != 0 {
		t.Errorf("expected [0], got %v", p)
	}
	if qb.GetPathOfID("missing", q) != nil {
		t.Error("expected nil for missing ID")
	}
}

func TestPathHelpers(t *testing.T) {
	if !qb.PathsAreEqual(qb.Path{1, 2}, qb.Path{1, 2}) {
		t.Error("equal paths not detected")
	}
	if qb.PathsAreEqual(qb.Path{1}, qb.Path{2}) {
		t.Error("unequal paths reported equal")
	}
	if !qb.IsAncestor(qb.Path{1}, qb.Path{1, 2}) {
		t.Error("IsAncestor failed")
	}
	if qb.IsAncestor(qb.Path{1, 2}, qb.Path{1, 2}) {
		t.Error("equal path should not be an ancestor")
	}
	common := qb.GetCommonAncestorPath(qb.Path{1, 2, 3}, qb.Path{1, 2, 4})
	if !qb.PathsAreEqual(common, qb.Path{1, 2}) {
		t.Errorf("expected [1 2], got %v", common)
	}
}

func TestAnnotateAndStripPaths(t *testing.T) {
	q := newGroup("and", newRule("x", "=", 1), newRule("y", "=", 2))
	annotated := qb.AnnotatePaths(q, nil).(*qb.RuleGroup)
	if len(annotated.Path) != 0 {
		t.Errorf("root path should be [], got %v", annotated.Path)
	}
	r0 := annotated.Rules[0].(*qb.Rule)
	if len(r0.Path) != 1 || r0.Path[0] != 0 {
		t.Errorf("expected [0], got %v", r0.Path)
	}
	stripped := qb.StripPaths(annotated).(*qb.RuleGroup)
	if len(stripped.Path) != 0 {
		t.Errorf("stripped root path should be nil/empty, got %v", stripped.Path)
	}
}

// --- IDs ---

func TestGenerateUUID(t *testing.T) {
	id := qb.GenerateUUID()
	if len(id) != 36 {
		t.Errorf("expected UUID length 36, got %d: %s", len(id), id)
	}
	id2 := qb.GenerateUUID()
	if id == id2 {
		t.Error("two UUIDs should not be equal")
	}
}

func TestPrepareNode(t *testing.T) {
	r := &qb.Rule{Field: "x", Operator: "=", Value: 1}
	prepared := qb.PrepareNode(r, nil).(*qb.Rule)
	if prepared.ID == "" {
		t.Error("PrepareNode should assign ID")
	}
	// idempotent
	prepared2 := qb.PrepareNode(prepared, nil).(*qb.Rule)
	if prepared2.ID != prepared.ID {
		t.Error("PrepareNode should not overwrite existing ID")
	}
}

func TestRegenerateIDs(t *testing.T) {
	r := &qb.Rule{CommonProperties: qb.CommonProperties{ID: "old"}, Field: "x", Operator: "=", Value: 1}
	regen := qb.RegenerateIDs(r, nil).(*qb.Rule)
	if regen.ID == "old" {
		t.Error("RegenerateIDs should assign new ID")
	}
}

// --- config ---

func TestParseMultiValue(t *testing.T) {
	vals := qb.ParseMultiValue("London,Paris,Tokyo", "")
	if len(vals) != 3 || vals[1] != "Paris" {
		t.Errorf("unexpected: %v", vals)
	}
	vals2 := qb.ParseMultiValue("a\\,b,c", "")
	if len(vals2) != 2 || vals2[0] != "a,b" {
		t.Errorf("escaped comma: %v", vals2)
	}
}

func TestJoinMultiValue(t *testing.T) {
	joined := qb.JoinMultiValue([]string{"a,b", "c"}, "")
	if joined != `a\,b,c` {
		t.Errorf("unexpected join: %s", joined)
	}
}

func TestFindOption(t *testing.T) {
	list := &qb.OptionList{Flat: qb.DefaultCombinators}
	opt := qb.FindOption(list, "and")
	if opt == nil || opt.Label != "AND" {
		t.Errorf("expected AND combinator, got %v", opt)
	}
	if qb.FindOption(list, "xor") != nil {
		t.Error("xor should not be in default combinators")
	}
}

// --- manipulation ---

func TestAdd(t *testing.T) {
	q := newGroup("and")
	r := newRule("age", ">", 18)
	result := qb.Add(q, r, qb.Path{}, qb.AddOptions{}).(*qb.RuleGroup)
	if len(result.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(result.Rules))
	}
}

func TestRemove(t *testing.T) {
	r1 := newRule("a", "=", 1)
	r2 := newRule("b", "=", 2)
	q := newGroup("and", r1, r2)
	result := qb.Remove(q, qb.Path{0}).(*qb.RuleGroup)
	if len(result.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(result.Rules))
	}
	if result.Rules[0].(*qb.Rule).Field != "b" {
		t.Error("wrong rule retained")
	}
}

func TestUpdate(t *testing.T) {
	r := &qb.Rule{CommonProperties: qb.CommonProperties{ID: "r1"}, Field: "x", Operator: "=", Value: "old"}
	q := &qb.RuleGroup{Combinator: "and", Rules: []qb.AnyNode{r}}
	result := qb.Update(q, "value", "new", qb.Path{0}, qb.UpdateOptions{}).(*qb.RuleGroup)
	if result.Rules[0].(*qb.Rule).Value != "new" {
		t.Error("expected value to be updated")
	}
}

func TestUpdateCombinator(t *testing.T) {
	q := newGroup("and", newRule("x", "=", 1))
	result := qb.Update(q, "combinator", "or", qb.Path{}, qb.UpdateOptions{}).(*qb.RuleGroup)
	if result.Combinator != "or" {
		t.Errorf("expected combinator=or, got %s", result.Combinator)
	}
}

func TestMove(t *testing.T) {
	r1 := newRule("a", "=", 1)
	r2 := newRule("b", "=", 2)
	r3 := newRule("c", "=", 3)
	q := newGroup("and", r1, r2, r3)
	// Move r1 ([0]) to position [2] — r2 and r3 shift left after removal, so adjusted target is [1]
	result := qb.Move(q, qb.Path{0}, qb.Path{2}, qb.MoveOptions{}).(*qb.RuleGroup)
	if len(result.Rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(result.Rules))
	}
	// After move: b, a, c OR b, c, a depending on adjusted index
	if result.Rules[0].(*qb.Rule).Field != "b" {
		t.Errorf("expected b first after move, got %s", result.Rules[0].(*qb.Rule).Field)
	}
}

func TestInsert(t *testing.T) {
	r1 := newRule("a", "=", 1)
	r2 := newRule("b", "=", 2)
	q := newGroup("and", r1)
	result := qb.Insert(q, r2, qb.Path{0}, qb.InsertOptions{}).(*qb.RuleGroup)
	if len(result.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(result.Rules))
	}
	if result.Rules[0].(*qb.Rule).Field != "b" {
		t.Errorf("expected inserted rule first, got %s", result.Rules[0].(*qb.Rule).Field)
	}
}

func TestGroup(t *testing.T) {
	r1 := &qb.Rule{CommonProperties: qb.CommonProperties{ID: "r1"}, Field: "a", Operator: "=", Value: 1}
	r2 := &qb.Rule{CommonProperties: qb.CommonProperties{ID: "r2"}, Field: "b", Operator: "=", Value: 2}
	q := &qb.RuleGroup{Combinator: "and", Rules: []qb.AnyNode{r1, r2}}
	result := qb.Group(q, qb.Path{0}, qb.Path{1}, qb.GroupOptions{}).(*qb.RuleGroup)
	// r2 at [0] should now be a sub-group
	sub, ok := result.Rules[0].(*qb.RuleGroup)
	if !ok {
		t.Fatalf("expected sub-group at [0], got %T", result.Rules[0])
	}
	if len(sub.Rules) != 2 {
		t.Errorf("expected 2 children in sub-group, got %d", len(sub.Rules))
	}
}

func TestConvertToIC(t *testing.T) {
	q := newGroup("and", newRule("a", "=", 1), newRule("b", "=", 2))
	ic := qb.ConvertToIC(q, "")
	if len(ic.Rules) != 3 {
		t.Errorf("IC should have 3 items (node,comb,node), got %d", len(ic.Rules))
	}
	if !ic.Rules[1].IsString() {
		t.Error("item 1 should be combinator string")
	}
}

func TestConvertFromIC(t *testing.T) {
	r1 := newRule("a", "=", 1)
	r2 := newRule("b", "=", 2)
	ic := &qb.RuleGroupIC{Rules: []qb.ICItem{qb.NodeItem(r1), qb.StringItem("or"), qb.NodeItem(r2)}}
	std := qb.ConvertFromIC(ic, "")
	if std.Combinator != "or" {
		t.Errorf("expected combinator=or, got %s", std.Combinator)
	}
	if len(std.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(std.Rules))
	}
}

// --- validation ---

func TestDefaultValidator(t *testing.T) {
	q := &qb.RuleGroup{
		CommonProperties: qb.CommonProperties{ID: "root"},
		Combinator:       "and",
		Rules:            []qb.AnyNode{},
	}
	vm := qb.DefaultValidator(q)
	if qb.IsNodeValid("root", vm) {
		t.Error("empty group should be invalid")
	}
}

func TestDefaultValidatorValid(t *testing.T) {
	q := &qb.RuleGroup{
		CommonProperties: qb.CommonProperties{ID: "root"},
		Combinator:       "and",
		Rules:            []qb.AnyNode{newRule("x", "=", 1)},
	}
	vm := qb.DefaultValidator(q)
	if !qb.IsNodeValid("root", vm) {
		t.Error("non-empty valid group should be valid")
	}
}

func TestIsNodeValid(t *testing.T) {
	if !qb.IsNodeValid("x", nil) {
		t.Error("nil map should be valid")
	}
	vm := qb.ValidationMap{"x": qb.ValidationResult{Valid: false, Reasons: []any{"empty"}}}
	if qb.IsNodeValid("x", vm) {
		t.Error("invalid node should return false")
	}
	if !qb.IsNodeValid("missing", vm) {
		t.Error("missing node should return true")
	}
}

// --- serialization: SQL ---

func TestFormatSQL_Basic(t *testing.T) {
	q := newGroup("and",
		newRule("firstName", "=", "Steve"),
		newRule("age", ">", 30),
	)
	sql := qb.FormatSQL(q, qb.SqlExportOptions{})
	if !strings.Contains(sql, "firstName") || !strings.Contains(sql, "age") {
		t.Errorf("unexpected SQL: %s", sql)
	}
}

func TestFormatSQL_Not(t *testing.T) {
	q := &qb.RuleGroup{
		Combinator: "and",
		Not:        true,
		Rules:      []qb.AnyNode{newRule("x", "=", 1)},
	}
	sql := qb.FormatSQL(q, qb.SqlExportOptions{})
	if !strings.Contains(sql, "NOT") {
		t.Errorf("expected NOT in SQL, got: %s", sql)
	}
}

func TestFormatSQL_Unary(t *testing.T) {
	q := newGroup("and", newRule("deletedAt", "null", nil))
	sql := qb.FormatSQL(q, qb.SqlExportOptions{})
	if !strings.Contains(sql, "is null") {
		t.Errorf("expected IS NULL, got: %s", sql)
	}
}

func TestFormatSQL_Between(t *testing.T) {
	q := newGroup("and", newRule("age", "between", "20,40"))
	sql := qb.FormatSQL(q, qb.SqlExportOptions{})
	if !strings.Contains(sql, "between") {
		t.Errorf("expected BETWEEN, got: %s", sql)
	}
}

func TestFormatSQL_In(t *testing.T) {
	q := newGroup("and", newRule("city", "in", "London,Paris"))
	sql := qb.FormatSQL(q, qb.SqlExportOptions{})
	if !strings.Contains(sql, "in") || !strings.Contains(sql, "London") {
		t.Errorf("unexpected IN SQL: %s", sql)
	}
}

func TestFormatSQL_LIKE(t *testing.T) {
	q := newGroup("and", newRule("name", "contains", "alice"))
	sql := qb.FormatSQL(q, qb.SqlExportOptions{})
	if !strings.Contains(sql, "LIKE") || !strings.Contains(sql, "%alice%") {
		t.Errorf("unexpected LIKE SQL: %s", sql)
	}
}

func TestFormatSQL_PostgreSQLPreset(t *testing.T) {
	q := newGroup("and", newRule("id", "=", 1))
	sql := qb.FormatSQL(q, qb.SqlExportOptions{Preset: "postgresql"})
	if !strings.Contains(sql, `"id"`) {
		t.Errorf("expected quoted field, got: %s", sql)
	}
}

func TestFormatParameterized(t *testing.T) {
	q := newGroup("and",
		newRule("firstName", "=", "Steve"),
		newRule("age", ">", 30),
	)
	result := qb.FormatParameterized(q, qb.ParameterizedExportOptions{})
	if !strings.Contains(result.SQL, "?") {
		t.Errorf("expected ? placeholder, got: %s", result.SQL)
	}
	if len(result.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(result.Params))
	}
}

func TestFormatParameterizedNamed(t *testing.T) {
	q := newGroup("and",
		newRule("firstName", "=", "Steve"),
		newRule("lastName", "=", "Vai"),
	)
	result := qb.FormatParameterizedNamed(q, qb.ParameterizedExportOptions{})
	if !strings.Contains(result.SQL, ":firstName_1") {
		t.Errorf("expected :firstName_1, got: %s", result.SQL)
	}
	if len(result.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(result.Params))
	}
}

// --- serialization: MongoDB ---

func TestFormatMongoDBQuery(t *testing.T) {
	q := newGroup("and",
		newRule("age", ">", 30),
		newRule("city", "in", "London,Paris"),
	)
	doc := qb.FormatMongoDBQuery(q, qb.CommonExportOptions{})
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "$and") || !strings.Contains(s, "$gt") {
		t.Errorf("unexpected mongo doc: %s", s)
	}
}

func TestFormatMongoDBQuery_Or(t *testing.T) {
	q := newGroup("or",
		newRule("x", "=", 1),
		newRule("y", "=", 2),
	)
	doc := qb.FormatMongoDBQuery(q, qb.CommonExportOptions{})
	b, _ := json.Marshal(doc)
	if !strings.Contains(string(b), "$or") {
		t.Errorf("expected $or, got: %s", b)
	}
}

// --- serialization: JsonLogic ---

func TestFormatJSONLogic(t *testing.T) {
	q := newGroup("and",
		newRule("age", ">", 30),
		newRule("name", "=", "Alice"),
	)
	doc := qb.FormatJSONLogic(q, qb.CommonExportOptions{})
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"and"`) {
		t.Errorf("unexpected jsonlogic (missing 'and'): %s", s)
	}
	// Go html-escapes > as >; check for that or the literal >
	if !strings.Contains(s, `u003e`) && !strings.Contains(s, `">"`) {
		t.Errorf("unexpected jsonlogic (missing '>'): %s", s)
	}
}

func TestFormatJSONLogic_Not(t *testing.T) {
	q := &qb.RuleGroup{
		Combinator: "and",
		Not:        true,
		Rules:      []qb.AnyNode{newRule("x", "=", 1)},
	}
	doc := qb.FormatJSONLogic(q, qb.CommonExportOptions{})
	b, _ := json.Marshal(doc)
	if !strings.Contains(string(b), `"!"`) {
		t.Errorf("expected ! negation, got: %s", b)
	}
}

// --- IC group JSON round-trip ---

func TestICGroupJSONRoundTrip(t *testing.T) {
	q := &qb.RuleGroupIC{
		CommonProperties: qb.CommonProperties{ID: "root"},
		Rules: []qb.ICItem{
			qb.NodeItem(&qb.Rule{Field: "a", Operator: "=", Value: 1}),
			qb.StringItem("and"),
			qb.NodeItem(&qb.Rule{Field: "b", Operator: "=", Value: 2}),
		},
	}
	out, err := qb.FormatJSON(q)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := qb.ParseJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	ic, ok := parsed.(*qb.RuleGroupIC)
	if !ok {
		t.Fatalf("expected *RuleGroupIC, got %T", parsed)
	}
	if len(ic.Rules) != 3 {
		t.Errorf("expected 3 items, got %d", len(ic.Rules))
	}
	if !ic.Rules[1].IsString() || ic.Rules[1].Combinator != "and" {
		t.Error("middle item should be combinator 'and'")
	}
}

// --- defaults ---

func TestDefaultOperators(t *testing.T) {
	if len(qb.DefaultOperators) != 18 {
		t.Errorf("expected 18 default operators, got %d", len(qb.DefaultOperators))
	}
}

func TestOperatorNegationMap(t *testing.T) {
	if qb.OperatorNegationMap["="] != "!=" {
		t.Error("negation of = should be !=")
	}
	if qb.OperatorNegationMap["between"] != "notBetween" {
		t.Error("negation of between should be notBetween")
	}
}

func TestGetOperatorArity(t *testing.T) {
	nullOp := qb.Operator{FullOption: qb.FullOption{Name: "null", Value: "null"}, Arity: "unary"}
	if qb.GetOperatorArity(nullOp) != 1 {
		t.Error("null should have arity 1")
	}
	betweenOp := qb.Operator{FullOption: qb.FullOption{Name: "between", Value: "between"}, Arity: "ternary"}
	if qb.GetOperatorArity(betweenOp) != 3 {
		t.Error("between should have arity 3")
	}
	eqOp := qb.Operator{FullOption: qb.FullOption{Name: "=", Value: "="}}
	if qb.GetOperatorArity(eqOp) != 2 {
		t.Error("= should have arity 2")
	}
}

// --- effective disabled ---

func TestIsEffectivelyDisabled(t *testing.T) {
	child := &qb.Rule{CommonProperties: qb.CommonProperties{ID: "r1", Disabled: true}, Field: "x", Operator: "=", Value: 1}
	q := &qb.RuleGroup{CommonProperties: qb.CommonProperties{ID: "root"}, Combinator: "and", Rules: []qb.AnyNode{child}}
	if !qb.IsEffectivelyDisabled(qb.Path{0}, q) {
		t.Error("directly disabled node should report as effectively disabled")
	}
	if qb.IsEffectivelyDisabled(qb.Path{}, q) {
		t.Error("root (not disabled) should not be effectively disabled")
	}
}

func TestIsEffectivelyDisabled_Inherited(t *testing.T) {
	inner := &qb.Rule{Field: "x", Operator: "=", Value: 1}
	outer := &qb.RuleGroup{CommonProperties: qb.CommonProperties{Disabled: true}, Combinator: "and", Rules: []qb.AnyNode{inner}}
	q := &qb.RuleGroup{Combinator: "and", Rules: []qb.AnyNode{outer}}
	if !qb.IsEffectivelyDisabled(qb.Path{0, 0}, q) {
		t.Error("child of disabled group should be effectively disabled")
	}
}
