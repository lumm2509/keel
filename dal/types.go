package dal

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"

	transporthttp "github.com/lumm2509/keel/transport/http"
)

var ErrNotImplemented = errors.New("dal: not implemented")
var ErrUnsafeViewQuery = errors.New("dal: unsafe view query")

type Model interface{}

type RequestInfo = transporthttp.RequestInfo

type sqlRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

type Params map[string]any

type Expression interface {
	SQL() string
	Arguments() []any
}

type RawExpression struct {
	Query string
	Args  []any
}

func (e RawExpression) SQL() string      { return e.Query }
func (e RawExpression) Arguments() []any { return append([]any(nil), e.Args...) }
func Expr(query string, args ...any) RawExpression {
	return RawExpression{Query: query, Args: append([]any(nil), args...)}
}

type QueryFilter func(q *SelectQuery) error

type SelectQuery struct {
	DB      *sql.DB
	Table   string
	Columns []string
	Joins   []string
	Where   []Expression
	OrderBy []string
	GroupBy []string
	Having  []Expression
	Limit   int
	Offset  int
}

func (q *SelectQuery) AddWhere(expr ...Expression) *SelectQuery {
	q.Where = append(q.Where, expr...)
	return q
}

func (q *SelectQuery) AddOrderBy(orderBy ...string) *SelectQuery {
	q.OrderBy = append(q.OrderBy, orderBy...)
	return q
}

func (q *SelectQuery) WithDB(db *sql.DB) *SelectQuery {
	q.DB = db
	return q
}

func (q *SelectQuery) Build() (string, []any) {
	var b strings.Builder
	args := make([]any, 0)

	columns := "*"
	if len(q.Columns) > 0 {
		columns = strings.Join(q.Columns, ", ")
	}

	b.WriteString("SELECT ")
	b.WriteString(columns)

	if q.Table != "" {
		b.WriteString(" FROM ")
		b.WriteString(q.Table)
	}

	if len(q.Joins) > 0 {
		b.WriteByte(' ')
		b.WriteString(strings.Join(q.Joins, " "))
	}

	if len(q.Where) > 0 {
		b.WriteString(" WHERE ")
		for i, expr := range q.Where {
			if i > 0 {
				b.WriteString(" AND ")
			}
			b.WriteString(expr.SQL())
			args = append(args, expr.Arguments()...)
		}
	}

	if len(q.GroupBy) > 0 {
		b.WriteString(" GROUP BY ")
		b.WriteString(strings.Join(q.GroupBy, ", "))
	}

	if len(q.Having) > 0 {
		b.WriteString(" HAVING ")
		for i, expr := range q.Having {
			if i > 0 {
				b.WriteString(" AND ")
			}
			b.WriteString(expr.SQL())
			args = append(args, expr.Arguments()...)
		}
	}

	if len(q.OrderBy) > 0 {
		b.WriteString(" ORDER BY ")
		b.WriteString(strings.Join(q.OrderBy, ", "))
	}

	if q.Limit > 0 {
		b.WriteString(" LIMIT ")
		b.WriteString(intString(q.Limit))
	}

	if q.Offset > 0 {
		b.WriteString(" OFFSET ")
		b.WriteString(intString(q.Offset))
	}

	return b.String(), args
}

func (q *SelectQuery) QueryContext(ctx context.Context, db ...*sql.DB) (*sql.Rows, error) {
	var conn sqlRunner = q.DB
	if len(db) > 0 && db[0] != nil {
		conn = db[0]
	}
	if conn == nil {
		return nil, sql.ErrConnDone
	}

	query, args := q.Build()
	return conn.QueryContext(ctx, query, args...)
}

func (q *SelectQuery) allWith(ctx context.Context, conn sqlRunner) ([]map[string]any, error) {
	if conn == nil {
		return nil, sql.ErrConnDone
	}

	query, args := q.Build()
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows)
}

func (q *SelectQuery) AllContext(ctx context.Context, db ...*sql.DB) ([]map[string]any, error) {
	if len(db) > 0 && db[0] != nil {
		return q.allWith(ctx, db[0])
	}
	return q.allWith(ctx, q.DB)
}

func (q *SelectQuery) OneContext(ctx context.Context, db ...*sql.DB) (map[string]any, error) {
	q.Limit = 1

	var conn sqlRunner = q.DB
	if len(db) > 0 && db[0] != nil {
		conn = db[0]
	}
	items, err := q.allWith(ctx, conn)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, sql.ErrNoRows
	}

	return items[0], nil
}

type TableInfoRow struct {
	Schema       string
	TableName    string
	ColumnName   string
	DataType     string
	IsNullable   bool
	DefaultValue sql.NullString
	IsPrimaryKey bool
}

type Field struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Type   string         `json:"type"`
	System bool           `json:"system"`
	Meta   map[string]any `json:"meta,omitempty"`
}

type FieldsList []Field

type Collection struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Type   string         `json:"type"`
	System bool           `json:"system"`
	Fields FieldsList     `json:"fields,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

type Record struct {
	ID         string         `json:"id"`
	Collection *Collection    `json:"collection,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	Expand     map[string]any `json:"expand,omitempty"`
}

type Log struct {
	ID      string         `json:"id"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}

type LogsStatsItem struct {
	Bucket string `json:"bucket"`
	Count  int64  `json:"count"`
}

type ExternalAuth struct {
	ID            string         `json:"id"`
	CollectionRef string         `json:"collectionRef"`
	RecordRef     string         `json:"recordRef"`
	Provider      string         `json:"provider"`
	ProviderID    string         `json:"providerId"`
	Meta          map[string]any `json:"meta,omitempty"`
}

type MFA struct {
	ID            string         `json:"id"`
	CollectionRef string         `json:"collectionRef"`
	RecordRef     string         `json:"recordRef"`
	Meta          map[string]any `json:"meta,omitempty"`
}

type OTP struct {
	ID            string         `json:"id"`
	CollectionRef string         `json:"collectionRef"`
	RecordRef     string         `json:"recordRef"`
	Meta          map[string]any `json:"meta,omitempty"`
}

type AuthOrigin struct {
	ID            string         `json:"id"`
	CollectionRef string         `json:"collectionRef"`
	RecordRef     string         `json:"recordRef"`
	Fingerprint   string         `json:"fingerprint"`
	Meta          map[string]any `json:"meta,omitempty"`
}

func intString(v int) string {
	if v == 0 {
		return "0"
	}

	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}

	return string(buf[i:])
}

func scanRows(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		scanTargets := make([]any, len(columns))
		for i := range values {
			scanTargets[i] = &values[i]
		}

		if err := rows.Scan(scanTargets...); err != nil {
			return nil, err
		}

		item := make(map[string]any, len(columns))
		for i, column := range columns {
			item[column] = normalizeScannedValue(values[i])
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func normalizeScannedValue(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

var identifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func QuoteIdentifier(name string) string {
	parts := strings.Split(name, ".")
	quoted := make([]string, 0, len(parts))

	for _, part := range parts {
		if part == "*" {
			quoted = append(quoted, part)
			continue
		}

		if !identifierPattern.MatchString(part) {
			return name
		}

		quoted = append(quoted, `"`+part+`"`)
	}

	return strings.Join(quoted, ".")
}
