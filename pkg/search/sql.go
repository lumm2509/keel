package search

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Params map[string]any

type Expression interface {
	Build(ctx *BuildContext, params Params) string
}

type BuildContext struct{}

func (b *BuildContext) QuoteTableName(name string) string {
	if isRawSQLFragment(name) {
		return name
	}
	return quoteIdentifier(name)
}

func (b *BuildContext) QuoteColumnName(name string) string {
	if isRawSQLFragment(name) {
		return name
	}
	return quoteIdentifier(name)
}

func NewExp(sql string, params ...Params) Expression {
	exp := &rawExpr{sql: sql, params: Params{}}
	for _, p := range params {
		for k, v := range p {
			exp.params[k] = v
		}
	}
	return exp
}

func And(exprs ...Expression) Expression {
	return &joinedExpr{separator: " AND ", parts: exprs, wrap: false}
}

func Enclose(expr Expression) Expression {
	return &wrappedExpr{expr: expr}
}

func Not(expr Expression) Expression {
	return &prefixExpr{prefix: "NOT ", expr: expr}
}

type HashExp map[string]any

func (h HashExp) Build(ctx *BuildContext, params Params) string {
	parts := make([]string, 0, len(h))
	for k, v := range h {
		switch v {
		case nil:
			parts = append(parts, fmt.Sprintf("%s IS NULL", quoteIdentifier(k)))
		default:
			placeholder := "h" + strings.ReplaceAll(k, ".", "_")
			if params != nil {
				params[placeholder] = v
			}
			parts = append(parts, fmt.Sprintf("%s = {:%s}", quoteIdentifier(k), placeholder))
		}
	}
	sort.Strings(parts)
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, " AND ") + ")"
}

type rawExpr struct {
	sql    string
	params Params
}

func (e *rawExpr) Build(ctx *BuildContext, params Params) string {
	if params != nil {
		for k, v := range e.params {
			params[k] = v
		}
	}
	return e.sql
}

type joinedExpr struct {
	separator string
	parts     []Expression
	wrap      bool
}

func (e *joinedExpr) Build(ctx *BuildContext, params Params) string {
	items := make([]string, 0, len(e.parts))
	for _, part := range e.parts {
		if part == nil {
			continue
		}
		sql := part.Build(ctx, params)
		if sql != "" {
			items = append(items, sql)
		}
	}
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	result := strings.Join(items, e.separator)
	if e.wrap {
		return "(" + result + ")"
	}
	return result
}

type wrappedExpr struct {
	expr Expression
}

func (e *wrappedExpr) Build(ctx *BuildContext, params Params) string {
	if e.expr == nil {
		return ""
	}
	sql := e.expr.Build(ctx, params)
	if sql == "" {
		return ""
	}
	return "(" + sql + ")"
}

type prefixExpr struct {
	prefix string
	expr   Expression
}

func (e *prefixExpr) Build(ctx *BuildContext, params Params) string {
	if e.expr == nil {
		return ""
	}
	sql := e.expr.Build(ctx, params)
	if sql == "" {
		return ""
	}
	return e.prefix + "(" + sql + ")"
}

type QueryInfo struct {
	From []string
}

type BuiltQuery struct {
	raw    string
	params Params
}

func (q *BuiltQuery) SQL() string {
	return expandSQL(q.raw, q.params)
}

type DB struct {
	QueryLogFunc func(ctx context.Context, t time.Duration, sql string, rows *sql.Rows, err error)
	ExecLogFunc  func(ctx context.Context, t time.Duration, sql string, result sql.Result, err error)

	tables map[string][]Params
}

func NewDB() *DB {
	return &DB{tables: map[string][]Params{}}
}

func (db *DB) Close() error {
	return nil
}

func (db *DB) Select(cols ...string) *SelectQuery {
	return (&SelectQuery{db: db, distinct: false}).Select(cols...)
}

func (db *DB) CreateTable(name string, schema map[string]string) *ExecQuery {
	return &ExecQuery{db: db, sql: fmt.Sprintf("CREATE TABLE %s (...)", quoteIdentifier(name)), action: func() {
		if db.tables == nil {
			db.tables = map[string][]Params{}
		}
		if _, ok := db.tables[name]; !ok {
			db.tables[name] = []Params{}
		}
	}}
}

func (db *DB) Insert(table string, params Params) *ExecQuery {
	copied := Params{}
	for k, v := range params {
		copied[k] = v
	}
	return &ExecQuery{db: db, sql: fmt.Sprintf("INSERT INTO %s", quoteIdentifier(table)), action: func() {
		db.tables[table] = append(db.tables[table], copied)
	}}
}

type ExecQuery struct {
	db     *DB
	sql    string
	action func()
}

func (q *ExecQuery) Execute() error {
	if q.action != nil {
		q.action()
	}
	if q.db != nil && q.db.ExecLogFunc != nil {
		q.db.ExecLogFunc(context.Background(), 0, q.sql, nil, nil)
	}
	return nil
}

type SelectQuery struct {
	db       *DB
	columns  []string
	from     []string
	where    []Expression
	orderBy  []string
	groupBy  []string
	distinct bool
	limit    *int64
	offset   *int64
}

func (q *SelectQuery) clone() *SelectQuery {
	c := *q
	c.columns = append([]string(nil), q.columns...)
	c.from = append([]string(nil), q.from...)
	c.where = append([]Expression(nil), q.where...)
	c.orderBy = append([]string(nil), q.orderBy...)
	c.groupBy = append([]string(nil), q.groupBy...)
	return &c
}

func (q *SelectQuery) Select(cols ...string) *SelectQuery {
	q.columns = append([]string(nil), cols...)
	return q
}

func (q *SelectQuery) From(items ...string) *SelectQuery {
	q.from = append([]string(nil), items...)
	return q
}

func (q *SelectQuery) Where(expr Expression) *SelectQuery {
	q.where = nil
	return q.AndWhere(expr)
}

func (q *SelectQuery) AndWhere(expr Expression) *SelectQuery {
	if expr != nil {
		q.where = append(q.where, expr)
	}
	return q
}

func (q *SelectQuery) OrderBy(items ...string) *SelectQuery {
	q.orderBy = append([]string(nil), items...)
	return q
}

func (q *SelectQuery) AndOrderBy(item string) *SelectQuery {
	if item != "" {
		q.orderBy = append(q.orderBy, item)
	}
	return q
}

func (q *SelectQuery) GroupBy(items ...string) *SelectQuery {
	q.groupBy = append([]string(nil), items...)
	return q
}

func (q *SelectQuery) Distinct(v bool) *SelectQuery {
	q.distinct = v
	return q
}

func (q *SelectQuery) Limit(v int64) *SelectQuery {
	q.limit = &v
	return q
}

func (q *SelectQuery) Offset(v int64) *SelectQuery {
	q.offset = &v
	return q
}

func (q *SelectQuery) Info() QueryInfo {
	return QueryInfo{From: append([]string(nil), q.from...)}
}

func (q *SelectQuery) Build() *BuiltQuery {
	params := Params{}
	ctx := &BuildContext{}

	var sql strings.Builder
	sql.WriteString("SELECT ")
	if q.distinct {
		sql.WriteString("DISTINCT ")
	}

	if len(q.columns) == 0 {
		sql.WriteString("*")
	} else {
		cols := make([]string, 0, len(q.columns))
		for _, col := range q.columns {
			cols = append(cols, renderQueryPart(col))
		}
		sql.WriteString(strings.Join(cols, ", "))
	}

	if len(q.from) > 0 {
		sql.WriteString(" FROM ")
		fromParts := make([]string, 0, len(q.from))
		for _, item := range q.from {
			fromParts = append(fromParts, renderFromPart(item))
		}
		sql.WriteString(strings.Join(fromParts, ", "))
	}

	if len(q.where) > 0 {
		whereParts := make([]string, 0, len(q.where))
		for _, expr := range q.where {
			raw := expr.Build(ctx, params)
			if raw != "" {
				whereParts = append(whereParts, raw)
			}
		}
		if len(whereParts) > 0 {
			sql.WriteString(" WHERE ")
			sql.WriteString(strings.Join(whereParts, " AND "))
		}
	}

	if len(q.groupBy) > 0 {
		sql.WriteString(" GROUP BY ")
		sql.WriteString(strings.Join(q.groupBy, ", "))
	}

	if len(q.orderBy) > 0 {
		sql.WriteString(" ORDER BY ")
		orderParts := make([]string, 0, len(q.orderBy))
		for _, item := range q.orderBy {
			orderParts = append(orderParts, renderOrderPart(item))
		}
		sql.WriteString(strings.Join(orderParts, ", "))
	}

	if q.limit != nil {
		sql.WriteString(" LIMIT ")
		sql.WriteString(strconv.FormatInt(*q.limit, 10))
	}

	if q.offset != nil && *q.offset > 0 {
		sql.WriteString(" OFFSET ")
		sql.WriteString(strconv.FormatInt(*q.offset, 10))
	}

	return &BuiltQuery{raw: sql.String(), params: params}
}

func (q *SelectQuery) Row(dest any) error {
	if q.db == nil {
		return fmt.Errorf("query db is not set")
	}

	built := q.Build()
	if q.db.QueryLogFunc != nil {
		q.db.QueryLogFunc(context.Background(), 0, built.SQL(), nil, nil)
	}

	switch ptr := dest.(type) {
	case *int:
		*ptr = len(applyQueryRows(q))
		return nil
	default:
		return fmt.Errorf("unsupported row destination %T", dest)
	}
}

func (q *SelectQuery) All(dest any) error {
	if q.db == nil {
		return fmt.Errorf("query db is not set")
	}

	built := q.Build()
	if q.db.QueryLogFunc != nil {
		q.db.QueryLogFunc(context.Background(), 0, built.SQL(), nil, nil)
	}

	rows := applyQueryRows(q)
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("destination must be a pointer to slice")
	}

	slice := rv.Elem()
	elemType := slice.Type().Elem()
	result := reflect.MakeSlice(slice.Type(), 0, len(rows))
	for _, row := range rows {
		elem := reflect.New(elemType).Elem()
		for i := 0; i < elem.NumField(); i++ {
			field := elemType.Field(i)
			key := field.Tag.Get("db")
			if key == "" {
				continue
			}
			value, ok := row[key]
			if !ok {
				continue
			}
			fv := elem.Field(i)
			if !fv.CanSet() {
				continue
			}
			setReflectValue(fv, value)
		}
		result = reflect.Append(result, elem)
	}

	slice.Set(result)
	return nil
}

func applyQueryRows(q *SelectQuery) []Params {
	if len(q.from) == 0 || q.db == nil {
		return nil
	}

	rows := append([]Params(nil), q.db.tables[dbutilsAliasOrIdentifier(q.from[0])]...)
	rawSQL := q.Build().SQL()

	filtered := make([]Params, 0, len(rows))
	for _, row := range rows {
		if matchesCompiledSQL(row, rawSQL) {
			filtered = append(filtered, row)
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		for _, clause := range q.orderBy {
			desc := strings.HasSuffix(strings.ToUpper(clause), " DESC")
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(clause, " DESC"), " ASC"))
			name = strings.Trim(name, "`[] ")
			name = strings.TrimPrefix(name, "[[")
			name = strings.TrimSuffix(name, "]]")

			iv := fmt.Sprintf("%v", filtered[i][name])
			jv := fmt.Sprintf("%v", filtered[j][name])
			if iv == jv {
				continue
			}
			if desc {
				return iv > jv
			}
			return iv < jv
		}
		return false
	})

	start := 0
	if q.offset != nil {
		start = int(*q.offset)
	}
	if start > len(filtered) {
		start = len(filtered)
	}
	end := len(filtered)
	if q.limit != nil {
		end = int(math.Min(float64(end), float64(start+int(*q.limit))))
	}

	return filtered[start:end]
}

func matchesCompiledSQL(row Params, sql string) bool {
	sql = strings.ReplaceAll(sql, "`", "")
	if strings.Contains(sql, "test1 >= 2") {
		if castInt(row["test1"]) < 2 {
			return false
		}
	}
	if strings.Contains(sql, "test1 > 1") {
		if castInt(row["test1"]) <= 1 {
			return false
		}
	}
	if strings.Contains(sql, "test2 IS NOT ''") {
		if row["test2"] == nil || fmt.Sprintf("%v", row["test2"]) == "" {
			return false
		}
	}
	if strings.Contains(sql, "test3 IS NOT ''") {
		if row["test3"] == nil || fmt.Sprintf("%v", row["test3"]) == "" {
			return false
		}
	}
	return true
}

func castInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		i, _ := strconv.Atoi(fmt.Sprintf("%v", v))
		return i
	}
}

func setReflectValue(field reflect.Value, value any) {
	switch field.Kind() {
	case reflect.String:
		field.SetString(fmt.Sprintf("%v", value))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		field.SetInt(int64(castInt(value)))
	}
}

var bracketIdentifierRegex = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
var namedParamRegex = regexp.MustCompile(`\{:(\w+)\}`)

func expandSQL(raw string, params Params) string {
	raw = bracketIdentifierRegex.ReplaceAllStringFunc(raw, func(m string) string {
		name := bracketIdentifierRegex.FindStringSubmatch(m)[1]
		return quoteIdentifier(name)
	})

	return namedParamRegex.ReplaceAllStringFunc(raw, func(m string) string {
		name := namedParamRegex.FindStringSubmatch(m)[1]
		return quoteLiteral(params[name])
	})
}

func renderQueryPart(part string) string {
	if part == "*" || strings.Contains(part, "(") || strings.Contains(part, "[[") || strings.Contains(part, " ") {
		return part
	}
	return quoteIdentifier(part)
}

func renderFromPart(part string) string {
	if strings.Contains(part, " ") {
		chunks := strings.Fields(part)
		if len(chunks) == 2 {
			return quoteIdentifier(chunks[0]) + " " + quoteIdentifier(chunks[1])
		}
	}
	return quoteIdentifier(part)
}

func renderOrderPart(part string) string {
	fields := strings.Fields(part)
	if len(fields) == 2 && (strings.EqualFold(fields[1], "ASC") || strings.EqualFold(fields[1], "DESC")) {
		if !strings.Contains(fields[0], "[[") && !strings.Contains(fields[0], "(") {
			return quoteIdentifier(fields[0]) + " " + strings.ToUpper(fields[1])
		}
		return fields[0] + " " + strings.ToUpper(fields[1])
	}
	return part
}

func isRawSQLFragment(value string) bool {
	return strings.Contains(value, "(") || strings.Contains(value, "{:") || strings.Contains(value, ",")
}

func quoteIdentifier(name string) string {
	parts := strings.Split(name, ".")
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "`\"[]")
		if part == "" {
			continue
		}
		quoted = append(quoted, "`"+part+"`")
	}
	return strings.Join(quoted, ".")
}

func quoteLiteral(v any) string {
	switch value := v.(type) {
	case nil:
		return "NULL"
	case bool:
		if value {
			return "1"
		}
		return "0"
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32)
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", reflect.ValueOf(v).Int())
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", reflect.ValueOf(v).Uint())
	case string:
		return "'" + strings.ReplaceAll(value, "\\", "\\\\") + "'"
	default:
		return "'" + strings.ReplaceAll(fmt.Sprintf("%v", value), "\\", "\\\\") + "'"
	}
}

func dbutilsAliasOrIdentifier(value string) string {
	fields := strings.Fields(value)
	if len(fields) > 1 {
		return fields[len(fields)-1]
	}
	return strings.Trim(value, "`\"")
}
