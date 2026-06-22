# query-builder-go

Go implementation of the [Query Builder specification](https://github.com/lukethacoder/query-builder-spec) — a language-neutral standard for representing, addressing, manipulating, validating, and serializing structured filter queries.

[![CI](https://github.com/lukethacoder/query-builder-go/actions/workflows/ci.yml/badge.svg)](https://github.com/lukethacoder/query-builder-go/actions/workflows/ci.yml)

## Installation

```sh
go get github.com/lukethacoder/query-builder-go
```

Requires Go 1.21 or later.

## Usage

```go
import qb "github.com/lukethacoder/query-builder-go"

// Build a query
query := &qb.RuleGroup{
    Combinator: "and",
    Rules: []qb.AnyNode{
        &qb.Rule{Field: "firstName", Operator: "beginsWith", Value: "Stev"},
        &qb.RuleGroup{
            Combinator: "or",
            Not: true,
            Rules: []qb.AnyNode{
                &qb.Rule{Field: "age", Operator: "between", Value: "26,52"},
                &qb.Rule{Field: "city", Operator: "in", Value: "London,Paris,Tokyo"},
            },
        },
    },
}

// Serialize to SQL
sql := qb.FormatSQL(query, qb.SqlExportOptions{})
// => (firstName LIKE 'Stev%' and NOT (age between '26' and '52' or city in ('London', 'Paris', 'Tokyo')))

// Parameterized SQL
result := qb.FormatParameterized(query, qb.ParameterizedExportOptions{})
// result.SQL  => "(firstName LIKE ? and NOT (age between ? and ? or city in (?, ?, ?)))"
// result.Params => ["Stev%", "26", "52", "London", "Paris", "Tokyo"]

// Named parameters
named := qb.FormatParameterizedNamed(query, qb.ParameterizedExportOptions{})

// MongoDB filter document
doc := qb.FormatMongoDBQuery(query, qb.CommonExportOptions{})

// JsonLogic rule
logic := qb.FormatJSONLogic(query, qb.CommonExportOptions{})

// Canonical JSON
json, _ := qb.FormatJSON(query)
jsonNoIDs, _ := qb.FormatJSONWithoutIDs(query)

// Parse JSON back to a query
parsed, _ := qb.ParseJSON(json)
```

## Manipulation

All manipulation functions are immutable — they return a new query tree and never modify the input.

```go
// Add a rule
query = qb.Add(query, &qb.Rule{Field: "lastName", Operator: "=", Value: "Vai"}, qb.Path{}, qb.AddOptions{}).(qb.AnyNode)

// Remove a rule by path
query = qb.Remove(query, qb.Path{0})

// Update a property
query = qb.Update(query, "combinator", "or", qb.Path{}, qb.UpdateOptions{})

// Move a node
query = qb.Move(query, qb.Path{0}, qb.Path{1}, qb.MoveOptions{})

// Insert at an exact position
query = qb.Insert(query, newRule, qb.Path{0}, qb.InsertOptions{})

// Wrap two nodes into a sub-group
query = qb.Group(query, qb.Path{0}, qb.Path{1}, qb.GroupOptions{})

// Convert between standard and independent-combinator groups
icQuery := qb.ConvertToIC(stdGroup, "")
stdGroup = qb.ConvertFromIC(icGroup, "and")
```

## Validation

```go
// Default structural validator
vm := qb.DefaultValidator(query)
if !qb.IsNodeValid("root", vm) {
    // group is empty or has an invalid combinator
}
```

## SQL presets

Preset configurations for common databases:

| Preset       | Field quoting | Param style  |
|--------------|---------------|--------------|
| `ansi`       | none          | `?`          |
| `oracle`     | none          | `?`          |
| `sqlite`     | none          | `?` (prefix in key) |
| `mysql`      | none          | `?`          |
| `mssql`      | `[field]`     | `@param`     |
| `postgresql` | `"field"`     | `$1`, `$2`   |

```go
sql := qb.FormatSQL(query, qb.SqlExportOptions{Preset: "postgresql"})
```

## Conformance

This library targets **baseline export conformance** per the spec:

- Core (data model, paths, manipulation, validation, canonical JSON) — chapters 01-06 + §7.2
- Baseline exports: `sql`, `parameterized`, `parameterized_named`, `mongodb_query`, `jsonlogic`

## Versioning

This project uses [Conventional Commits](https://www.conventionalcommits.org/) and [semantic-release](https://semantic-release.gitbook.io/) for automated versioning. Go module versions are published as git tags (`v1.2.3`).
