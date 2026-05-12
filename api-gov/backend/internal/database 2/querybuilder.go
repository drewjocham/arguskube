package database

import (
	"fmt"
	"strings"
)

type Operator string

const (
	OpEq        Operator = "="
	OpNeq       Operator = "!="
	OpGt        Operator = ">"
	OpGte       Operator = ">="
	OpLt        Operator = "<"
	OpLte       Operator = "<="
	OpLike      Operator = "ILIKE"
	OpIn        Operator = "= ANY"
	OpIsNull    Operator = "IS NULL"
	OpNotNull   Operator = "IS NOT NULL"
	OpJSONPath  Operator = "@>"
	OpVectorL2  Operator = "<->"
	OpVectorCos Operator = "<=>"
)

type whereClause struct {
	field    string
	operator Operator
	value    any
	raw      bool
}

type orderByClause struct {
	column string
	asc    bool
}

type joinClause struct {
	table     string
	alias     string
	condition string
	joinType  string
}

// QueryBuilder constructs parameterized SQL queries safely.
// All values use $N placeholders to prevent injection.
type QueryBuilder struct {
	action         string
	table          string
	columns        []string
	whereClauses   []whereClause
	orderByClauses []orderByClause
	joinClauses    []joinClause
	groupByCols    []string
	limitVal       int
	offsetVal      int
	returningCols  []string
	onConflictCol  string
	upsertCols     []string
}

func Select(table string, columns ...string) *QueryBuilder {
	return &QueryBuilder{action: "SELECT", table: table, columns: columns, limitVal: -1, offsetVal: -1}
}

func InsertInto(table string, columns ...string) *QueryBuilder {
	return &QueryBuilder{action: "INSERT", table: table, columns: columns}
}

func Update(table string, columns ...string) *QueryBuilder {
	return &QueryBuilder{action: "UPDATE", table: table, columns: columns}
}

func DeleteFrom(table string) *QueryBuilder {
	return &QueryBuilder{action: "DELETE", table: table}
}

func (q *QueryBuilder) Where(field string, op Operator, value any) *QueryBuilder {
	q.whereClauses = append(q.whereClauses, whereClause{field: field, operator: op, value: value})
	return q
}

func (q *QueryBuilder) WhereRaw(clause string) *QueryBuilder {
	q.whereClauses = append(q.whereClauses, whereClause{raw: true, field: clause})
	return q
}

func (q *QueryBuilder) OrderBy(column string, asc bool) *QueryBuilder {
	q.orderByClauses = append(q.orderByClauses, orderByClause{column: column, asc: asc})
	return q
}

func (q *QueryBuilder) Limit(n int) *QueryBuilder {
	q.limitVal = n
	return q
}

func (q *QueryBuilder) Offset(n int) *QueryBuilder {
	q.offsetVal = n
	return q
}

func (q *QueryBuilder) GroupBy(columns ...string) *QueryBuilder {
	q.groupByCols = append(q.groupByCols, columns...)
	return q
}

func (q *QueryBuilder) Returning(columns ...string) *QueryBuilder {
	q.returningCols = columns
	return q
}

func (q *QueryBuilder) OnConflict(column string) *QueryBuilder {
	q.onConflictCol = column
	return q
}

func (q *QueryBuilder) DoUpdate(cols ...string) *QueryBuilder {
	q.upsertCols = cols
	return q
}

func (q *QueryBuilder) Build() (string, []any) {
	switch q.action {
	case "SELECT":
		return q.buildSelect()
	case "INSERT":
		return q.buildInsert()
	case "UPDATE":
		return q.buildUpdate()
	case "DELETE":
		return q.buildDelete()
	}
	return "", nil
}

func (q *QueryBuilder) buildSelect() (string, []any) {
	if len(q.columns) == 0 {
		q.columns = []string{"*"}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("SELECT %s FROM %s", strings.Join(q.columns, ", "), q.table))

	for _, j := range q.joinClauses {
		b.WriteString(fmt.Sprintf(" %s %s AS %s ON %s", j.joinType, j.table, j.alias, j.condition))
	}

	args := q.buildWhereClause(&b)

	if len(q.groupByCols) > 0 {
		b.WriteString(fmt.Sprintf(" GROUP BY %s", strings.Join(q.groupByCols, ", ")))
	}

	b.WriteString(q.buildOrderByString())
	b.WriteString(q.buildLimitOffsetString())

	return b.String(), args
}

func (q *QueryBuilder) buildInsert() (string, []any) {
	if len(q.columns) == 0 {
		return "", nil
	}

	refs := make([]string, len(q.columns))
	for i := range q.columns {
		refs[i] = fmt.Sprintf("$%d", i+1)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		q.table, strings.Join(q.columns, ", "), strings.Join(refs, ", ")))

	if q.onConflictCol != "" {
		b.WriteString(fmt.Sprintf(" ON CONFLICT (%s)", q.onConflictCol))
		if len(q.upsertCols) > 0 {
			sets := make([]string, len(q.upsertCols))
			for i, col := range q.upsertCols {
				sets[i] = fmt.Sprintf("%s = EXCLUDED.%s", col, col)
			}
			b.WriteString(fmt.Sprintf(" DO UPDATE SET %s", strings.Join(sets, ", ")))
		} else {
			b.WriteString(" DO NOTHING")
		}
	}

	if len(q.returningCols) > 0 {
		b.WriteString(fmt.Sprintf(" RETURNING %s", strings.Join(q.returningCols, ", ")))
	}

	return b.String(), nil
}

func (q *QueryBuilder) buildUpdate() (string, []any) {
	if len(q.columns) == 0 {
		return "", nil
	}

	sets := make([]string, len(q.columns))
	for i, col := range q.columns {
		sets[i] = fmt.Sprintf("%s = $%d", col, i+1)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("UPDATE %s SET %s", q.table, strings.Join(sets, ", ")))

	whereArgs := q.buildWhereClause(&b)

	if len(q.returningCols) > 0 {
		b.WriteString(fmt.Sprintf(" RETURNING %s", strings.Join(q.returningCols, ", ")))
	}

	return b.String(), whereArgs
}

func (q *QueryBuilder) buildDelete() (string, []any) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("DELETE FROM %s", q.table))
	args := q.buildWhereClause(&b)
	return b.String(), args
}

func (q *QueryBuilder) whereStartIdx() int {
	switch q.action {
	case "INSERT":
		return len(q.columns) + 1
	case "UPDATE":
		return len(q.columns) + 1
	default:
		return 1
	}
}

func (q *QueryBuilder) buildWhereClause(b *strings.Builder) []any {
	if len(q.whereClauses) == 0 {
		return nil
	}

	clauses := make([]string, 0, len(q.whereClauses))
	var args []any

	startIdx := q.whereStartIdx()
	for _, w := range q.whereClauses {
		if w.raw {
			clauses = append(clauses, w.field)
			continue
		}

		switch w.operator {
		case OpIsNull, OpNotNull:
			clauses = append(clauses, fmt.Sprintf("%s %s", w.field, w.operator))
		default:
			clauses = append(clauses, fmt.Sprintf("%s %s $%d", w.field, w.operator, len(args)+startIdx))
			args = append(args, w.value)
		}
	}

	b.WriteString(fmt.Sprintf(" WHERE %s", strings.Join(clauses, " AND ")))
	return args
}

func (q *QueryBuilder) buildOrderByString() string {
	if len(q.orderByClauses) == 0 {
		return ""
	}
	clauses := make([]string, len(q.orderByClauses))
	for i, o := range q.orderByClauses {
		if o.asc {
			clauses[i] = fmt.Sprintf("%s ASC", o.column)
		} else {
			clauses[i] = fmt.Sprintf("%s DESC", o.column)
		}
	}
	return fmt.Sprintf(" ORDER BY %s", strings.Join(clauses, ", "))
}

func (q *QueryBuilder) buildLimitOffsetString() string {
	var b strings.Builder
	if q.limitVal >= 0 {
		b.WriteString(fmt.Sprintf(" LIMIT %d", q.limitVal))
	}
	if q.offsetVal >= 0 {
		b.WriteString(fmt.Sprintf(" OFFSET %d", q.offsetVal))
	}
	return b.String()
}
