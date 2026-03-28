package dal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
)

var namedParamPattern = regexp.MustCompile(`\{:[a-zA-Z_][a-zA-Z0-9_]*\}`)

const (
	collectionsTable   = "_collections"
	externalAuthsTable = "_externalAuths"
	mfasTable          = "_mfas"
	otpsTable          = "_otps"
	authOriginsTable   = "_authOrigins"
)

type Service struct {
	concurrentDB       *sql.DB
	nonconcurrentDB    *sql.DB
	auxConcurrentDB    *sql.DB
	auxNonconcurrentDB *sql.DB
	concurrentTx       *sql.Tx
	nonconcurrentTx    *sql.Tx
	auxConcurrentTx    *sql.Tx
	auxNonconcurrentTx *sql.Tx
}

func New(main *sql.DB, aux ...*sql.DB) *Service {
	var auxDB *sql.DB
	if len(aux) > 0 {
		auxDB = aux[0]
	}

	return NewWithDBs(main, nil, auxDB, nil)
}

func NewWithDBs(mainConcurrent *sql.DB, mainNonconcurrent *sql.DB, auxConcurrent *sql.DB, auxNonconcurrent *sql.DB) *Service {
	if mainNonconcurrent == nil {
		mainNonconcurrent = mainConcurrent
	}
	if auxConcurrent == nil {
		auxConcurrent = mainConcurrent
	}
	if auxNonconcurrent == nil {
		auxNonconcurrent = auxConcurrent
	}

	return &Service{
		concurrentDB:       mainConcurrent,
		nonconcurrentDB:    mainNonconcurrent,
		auxConcurrentDB:    auxConcurrent,
		auxNonconcurrentDB: auxNonconcurrent,
	}
}

func (s *Service) WithTransactions(mainTx *sql.Tx, auxTx *sql.Tx) *Service {
	clone := *s
	if mainTx != nil {
		clone.concurrentTx = mainTx
		clone.nonconcurrentTx = mainTx
	}
	if auxTx != nil {
		clone.auxConcurrentTx = auxTx
		clone.auxNonconcurrentTx = auxTx
	}
	return &clone
}

func (s *Service) ConcurrentDB() *sql.DB       { return s.concurrentDB }
func (s *Service) NonconcurrentDB() *sql.DB    { return s.nonconcurrentDB }
func (s *Service) AuxConcurrentDB() *sql.DB    { return s.auxConcurrentDB }
func (s *Service) AuxNonconcurrentDB() *sql.DB { return s.auxNonconcurrentDB }

func (s *Service) HasTable(tableName string) bool {
	ok, _ := s.hasTable(context.Background(), s.concurrentRunner(), tableName)
	return ok
}

func (s *Service) AuxHasTable(tableName string) bool {
	ok, _ := s.hasTable(context.Background(), s.auxConcurrentRunner(), tableName)
	return ok
}

func (s *Service) TableColumns(tableName string) ([]string, error) {
	rows, err := s.concurrentRunner().QueryContext(context.Background(), `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = $1
		ORDER BY ordinal_position
	`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result = append(result, name)
	}

	return result, rows.Err()
}

func (s *Service) TableInfo(tableName string) ([]*TableInfoRow, error) {
	rows, err := s.concurrentRunner().QueryContext(context.Background(), `
		SELECT
			c.table_schema,
			c.table_name,
			c.column_name,
			c.data_type,
			c.is_nullable = 'YES' AS is_nullable,
			c.column_default,
			EXISTS (
				SELECT 1
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
				  ON tc.constraint_name = kcu.constraint_name
				 AND tc.table_schema = kcu.table_schema
				WHERE tc.constraint_type = 'PRIMARY KEY'
				  AND tc.table_schema = c.table_schema
				  AND tc.table_name = c.table_name
				  AND kcu.column_name = c.column_name
			) AS is_primary_key
		FROM information_schema.columns c
		WHERE c.table_schema = current_schema()
		  AND c.table_name = $1
		ORDER BY c.ordinal_position
	`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*TableInfoRow{}
	for rows.Next() {
		item := new(TableInfoRow)
		if err := rows.Scan(
			&item.Schema,
			&item.TableName,
			&item.ColumnName,
			&item.DataType,
			&item.IsNullable,
			&item.DefaultValue,
			&item.IsPrimaryKey,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, rows.Err()
}

func (s *Service) TableIndexes(tableName string) (map[string]string, error) {
	rows, err := s.concurrentRunner().QueryContext(context.Background(), `
		SELECT indexname, indexdef
		FROM pg_indexes
		WHERE schemaname = current_schema()
		  AND tablename = $1
	`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]string{}
	for rows.Next() {
		var name, definition string
		if err := rows.Scan(&name, &definition); err != nil {
			return nil, err
		}
		result[name] = definition
	}

	return result, rows.Err()
}

func (s *Service) createViewFields(dangerousSelectQuery string) (FieldsList, error) {
	if err := validateViewSelectQuery(dangerousSelectQuery); err != nil {
		return nil, err
	}
	rows, err := s.concurrentRunner().QueryContext(context.Background(), "SELECT * FROM ("+dangerousSelectQuery+") AS keel_view LIMIT 0")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	fields := make(FieldsList, 0, len(columnTypes))
	for _, ct := range columnTypes {
		fields = append(fields, Field{
			Name: ct.Name(),
			Type: strings.ToLower(ct.DatabaseTypeName()),
		})
	}

	return fields, nil
}

func (s *Service) FindRecordByViewFile(viewCollectionModelOrIdentifier any, fileFieldName string, filename string) (*Record, error) {
	tableName := s.recordTableName(viewCollectionModelOrIdentifier)
	if tableName == "" {
		return nil, sql.ErrNoRows
	}

	query := &SelectQuery{
		DB:      s.concurrentDB,
		Table:   QuoteIdentifier(tableName),
		Columns: []string{"*"},
		Where:   []Expression{Expr(QuoteIdentifier(fileFieldName)+" = $1", filename)},
		Limit:   1,
	}

	row, err := query.OneContext(context.Background())
	if err != nil {
		return nil, err
	}

	return s.recordFromMap(tableName, row), nil
}

func (s *Service) ModelQuery(model Model) *SelectQuery {
	return (&SelectQuery{
		DB:      s.concurrentDB,
		Table:   QuoteIdentifier(s.modelTableName(model)),
		Columns: []string{"*"},
	})
}

func (s *Service) AuxModelQuery(model Model) *SelectQuery {
	return (&SelectQuery{
		DB:      s.auxConcurrentDB,
		Table:   QuoteIdentifier(s.modelTableName(model)),
		Columns: []string{"*"},
	})
}

func (s *Service) LogQuery() *SelectQuery {
	return (&SelectQuery{
		DB:      s.concurrentDB,
		Table:   QuoteIdentifier("logs"),
		Columns: []string{"*"},
	})
}

func (s *Service) CollectionQuery() *SelectQuery {
	return (&SelectQuery{
		DB:      s.concurrentDB,
		Table:   QuoteIdentifier(collectionsTable),
		Columns: []string{"*"},
	})
}

func (s *Service) RecordQuery(collectionModelOrIdentifier any) *SelectQuery {
	return (&SelectQuery{
		DB:      s.concurrentDB,
		Table:   QuoteIdentifier(s.recordTableName(collectionModelOrIdentifier)),
		Columns: []string{"*"},
	})
}

func (s *Service) FindLogById(id string) (*Log, error) {
	row, err := s.LogQuery().AddWhere(Expr(`"id" = $1`, id)).OneContext(context.Background())
	if err != nil {
		return nil, err
	}

	return &Log{
		ID:      asString(row["id"]),
		Level:   asString(row["level"]),
		Message: asString(row["message"]),
		Data:    row,
	}, nil
}

func (s *Service) LogsStats(expr Expression) ([]*LogsStatsItem, error) {
	query := `SELECT level, COUNT(*) FROM "logs"`
	args := []any{}
	if expr != nil {
		query += " WHERE " + expr.SQL()
		args = append(args, expr.Arguments()...)
	}
	query += " GROUP BY level ORDER BY level"

	rows, err := s.concurrentRunner().QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*LogsStatsItem
	for rows.Next() {
		item := new(LogsStatsItem)
		if err := rows.Scan(&item.Bucket, &item.Count); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, rows.Err()
}

func (s *Service) FindAllCollections(collectionTypes ...string) ([]*Collection, error) {
	q := s.CollectionQuery()
	if len(collectionTypes) > 0 {
		placeholders := make([]string, 0, len(collectionTypes))
		args := make([]any, 0, len(collectionTypes))
		for i, item := range collectionTypes {
			placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
			args = append(args, item)
		}
		q.AddWhere(Expr(`"type" IN (`+strings.Join(placeholders, ", ")+`)`, args...))
	}
	q.AddOrderBy(`"name" ASC`)

	rows, err := q.AllContext(context.Background())
	if err != nil {
		return nil, err
	}

	result := make([]*Collection, 0, len(rows))
	for _, row := range rows {
		result = append(result, collectionFromMap(row))
	}
	return result, nil
}

func (s *Service) FindCollectionByNameOrId(nameOrId string) (*Collection, error) {
	row, err := s.CollectionQuery().
		AddWhere(Expr(`("id" = $1 OR "name" = $1)`, nameOrId)).
		OneContext(context.Background())
	if err != nil {
		return nil, err
	}

	return collectionFromMap(row), nil
}

func (s *Service) FindCachedCollectionByNameOrId(nameOrId string) (*Collection, error) {
	return s.FindCollectionByNameOrId(nameOrId)
}

func (s *Service) FindCollectionReferences(collection *Collection, excludeIds ...string) (map[*Collection][]Field, error) {
	if collection == nil {
		return nil, sql.ErrNoRows
	}

	collections, err := s.FindAllCollections()
	if err != nil {
		return nil, err
	}

	excluded := map[string]struct{}{}
	for _, id := range excludeIds {
		excluded[id] = struct{}{}
	}

	result := map[*Collection][]Field{}
	for _, item := range collections {
		if item == nil || item.ID == collection.ID {
			continue
		}
		if _, skip := excluded[item.ID]; skip {
			continue
		}

		fields := make([]Field, 0)
		for _, field := range item.Fields {
			if fieldReferencesCollection(field, collection) {
				fields = append(fields, field)
			}
		}
		if len(fields) > 0 {
			result[item] = fields
		}
	}

	return result, nil
}

func (s *Service) FindCachedCollectionReferences(collection *Collection, excludeIds ...string) (map[*Collection][]Field, error) {
	return s.FindCollectionReferences(collection, excludeIds...)
}

func (s *Service) IsCollectionNameUnique(name string, excludeIds ...string) bool {
	collections, err := s.FindAllCollections()
	if err != nil {
		return false
	}

	excluded := map[string]struct{}{}
	for _, id := range excludeIds {
		excluded[id] = struct{}{}
	}

	for _, collection := range collections {
		if collection == nil {
			continue
		}
		if _, skip := excluded[collection.ID]; skip {
			continue
		}
		if strings.EqualFold(collection.Name, name) {
			return false
		}
	}

	return true
}

func (s *Service) FindAllExternalAuthsByRecord(authRecord *Record) ([]*ExternalAuth, error) {
	return s.findExternalAuthsByField("recordRef", authRecord.ID)
}

func (s *Service) FindAllExternalAuthsByCollection(collection *Collection) ([]*ExternalAuth, error) {
	if collection == nil {
		return nil, sql.ErrNoRows
	}
	return s.findExternalAuthsByField("collectionRef", collection.ID)
}

func (s *Service) FindFirstExternalAuthByExpr(expr Expression) (*ExternalAuth, error) {
	query := `SELECT * FROM ` + QuoteIdentifier(externalAuthsTable)
	args := []any{}
	if expr != nil {
		query += ` WHERE ` + expr.SQL()
		args = append(args, expr.Arguments()...)
	}
	query += ` LIMIT 1`

	rows, err := s.concurrentRunner().QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, sql.ErrNoRows
	}

	return externalAuthFromMap(items[0]), nil
}

func (s *Service) FindAllMFAsByRecord(authRecord *Record) ([]*MFA, error) {
	return s.findMFAsByField("recordRef", authRecord.ID)
}

func (s *Service) FindAllMFAsByCollection(collection *Collection) ([]*MFA, error) {
	if collection == nil {
		return nil, sql.ErrNoRows
	}
	return s.findMFAsByField("collectionRef", collection.ID)
}

func (s *Service) FindMFAById(id string) (*MFA, error) {
	row, err := s.findRowByID(mfasTable, id)
	if err != nil {
		return nil, err
	}
	return mfaFromMap(row), nil
}

func (s *Service) FindAllOTPsByRecord(authRecord *Record) ([]*OTP, error) {
	return s.findOTPsByField("recordRef", authRecord.ID)
}

func (s *Service) FindAllOTPsByCollection(collection *Collection) ([]*OTP, error) {
	if collection == nil {
		return nil, sql.ErrNoRows
	}
	return s.findOTPsByField("collectionRef", collection.ID)
}

func (s *Service) FindOTPById(id string) (*OTP, error) {
	row, err := s.findRowByID(otpsTable, id)
	if err != nil {
		return nil, err
	}
	return otpFromMap(row), nil
}

func (s *Service) FindAllAuthOriginsByRecord(authRecord *Record) ([]*AuthOrigin, error) {
	return s.findAuthOriginsByField("recordRef", authRecord.ID)
}

func (s *Service) FindAllAuthOriginsByCollection(collection *Collection) ([]*AuthOrigin, error) {
	if collection == nil {
		return nil, sql.ErrNoRows
	}
	return s.findAuthOriginsByField("collectionRef", collection.ID)
}

func (s *Service) FindAuthOriginById(id string) (*AuthOrigin, error) {
	row, err := s.findRowByID(authOriginsTable, id)
	if err != nil {
		return nil, err
	}
	return authOriginFromMap(row), nil
}

func (s *Service) FindAuthOriginByRecordAndFingerprint(authRecord *Record, fingerprint string) (*AuthOrigin, error) {
	query := `SELECT * FROM ` + QuoteIdentifier(authOriginsTable) + ` WHERE "recordRef" = $1 AND "fingerprint" = $2 LIMIT 1`
	rows, err := s.concurrentRunner().QueryContext(context.Background(), query, authRecord.ID, fingerprint)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, sql.ErrNoRows
	}
	return authOriginFromMap(items[0]), nil
}

func (s *Service) FindRecordById(collectionModelOrIdentifier any, recordId string, optFilters ...QueryFilter) (*Record, error) {
	q := s.RecordQuery(collectionModelOrIdentifier).AddWhere(Expr(`"id" = $1`, recordId))
	for _, filter := range optFilters {
		if filter == nil {
			continue
		}
		if err := filter(q); err != nil {
			return nil, err
		}
	}

	row, err := q.OneContext(context.Background())
	if err != nil {
		return nil, err
	}

	return s.recordFromMap(s.recordTableName(collectionModelOrIdentifier), row), nil
}

func (s *Service) FindRecordsByIds(collectionModelOrIdentifier any, recordIds []string, optFilters ...QueryFilter) ([]*Record, error) {
	if len(recordIds) == 0 {
		return nil, nil
	}

	placeholders := make([]string, 0, len(recordIds))
	args := make([]any, 0, len(recordIds))
	for i, id := range recordIds {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, id)
	}

	q := s.RecordQuery(collectionModelOrIdentifier).AddWhere(Expr(`"id" IN (`+strings.Join(placeholders, ", ")+`)`, args...))
	for _, filter := range optFilters {
		if filter == nil {
			continue
		}
		if err := filter(q); err != nil {
			return nil, err
		}
	}

	items, err := q.AllContext(context.Background())
	if err != nil {
		return nil, err
	}

	result := make([]*Record, 0, len(items))
	tableName := s.recordTableName(collectionModelOrIdentifier)
	for _, item := range items {
		result = append(result, s.recordFromMap(tableName, item))
	}

	return result, nil
}

func (s *Service) FindAllRecords(collectionModelOrIdentifier any, exprs ...Expression) ([]*Record, error) {
	q := s.RecordQuery(collectionModelOrIdentifier)
	q.Where = append(q.Where, exprs...)

	items, err := q.AllContext(context.Background())
	if err != nil {
		return nil, err
	}

	result := make([]*Record, 0, len(items))
	tableName := s.recordTableName(collectionModelOrIdentifier)
	for _, item := range items {
		result = append(result, s.recordFromMap(tableName, item))
	}

	return result, nil
}

func (s *Service) FindFirstRecordByData(collectionModelOrIdentifier any, key string, value any) (*Record, error) {
	q := s.RecordQuery(collectionModelOrIdentifier).AddWhere(Expr(QuoteIdentifier(key)+" = $1", value))
	row, err := q.OneContext(context.Background())
	if err != nil {
		return nil, err
	}
	return s.recordFromMap(s.recordTableName(collectionModelOrIdentifier), row), nil
}

func (s *Service) FindRecordsByFilter(collectionModelOrIdentifier any, filter string, sort string, limit int, offset int, params ...Params) ([]*Record, error) {
	query, args, err := bindNamedParams(filter, params...)
	if err != nil {
		return nil, err
	}

	q := s.RecordQuery(collectionModelOrIdentifier)
	if query != "" {
		q.AddWhere(Expr(query, args...))
	}
	if sort != "" {
		q.AddOrderBy(sort)
	}
	q.Limit = limit
	q.Offset = offset

	items, err := q.AllContext(context.Background())
	if err != nil {
		return nil, err
	}

	result := make([]*Record, 0, len(items))
	tableName := s.recordTableName(collectionModelOrIdentifier)
	for _, item := range items {
		result = append(result, s.recordFromMap(tableName, item))
	}

	return result, nil
}

func (s *Service) FindFirstRecordByFilter(collectionModelOrIdentifier any, filter string, params ...Params) (*Record, error) {
	records, err := s.FindRecordsByFilter(collectionModelOrIdentifier, filter, "", 1, 0, params...)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, sql.ErrNoRows
	}
	return records[0], nil
}

func (s *Service) CountRecords(collectionModelOrIdentifier any, exprs ...Expression) (int64, error) {
	query := "SELECT COUNT(*) FROM " + QuoteIdentifier(s.recordTableName(collectionModelOrIdentifier))
	args := make([]any, 0)
	if len(exprs) > 0 {
		query += " WHERE "
		for i, expr := range exprs {
			if i > 0 {
				query += " AND "
			}
			query += expr.SQL()
			args = append(args, expr.Arguments()...)
		}
	}

	var count int64
	err := s.concurrentRunner().QueryRowContext(context.Background(), query, args...).Scan(&count)
	return count, err
}

func (s *Service) FindAuthRecordByToken(token string, validTypes ...string) (*Record, error) {
	candidates := []string{"token", "tokenKey", "token_key"}
	if len(validTypes) > 0 {
		candidates = append(validTypes, candidates...)
	}

	collections, err := s.FindAllCollections()
	if err != nil {
		return nil, err
	}

	for _, collection := range collections {
		if collection == nil || collection.Name == collectionsTable {
			continue
		}
		columns, err := s.TableColumns(collection.Name)
		if err != nil {
			continue
		}
		for _, candidate := range candidates {
			if !slices.Contains(columns, candidate) {
				continue
			}
			record, err := s.FindFirstRecordByData(collection.Name, candidate, token)
			if err == nil {
				return record, nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		}
	}

	return nil, sql.ErrNoRows
}

func (s *Service) FindAuthRecordByEmail(collectionModelOrIdentifier any, email string) (*Record, error) {
	return s.FindFirstRecordByData(collectionModelOrIdentifier, "email", email)
}

func (s *Service) CanAccessRecord(record *Record, requestInfo *RequestInfo, accessRule *string) (bool, error) {
	if accessRule == nil || strings.TrimSpace(*accessRule) == "" {
		return true, nil
	}

	rule := strings.TrimSpace(*accessRule)
	switch strings.ToLower(rule) {
	case "*", "true", "allow", "public":
		return true, nil
	case "false", "deny", "private":
		return false, nil
	}

	return evalAccessRule(rule, record, requestInfo)
}

func (s *Service) findExternalAuthsByField(field string, value string) ([]*ExternalAuth, error) {
	rows, err := s.findRowsByField(externalAuthsTable, field, value)
	if err != nil {
		return nil, err
	}
	result := make([]*ExternalAuth, 0, len(rows))
	for _, row := range rows {
		result = append(result, externalAuthFromMap(row))
	}
	return result, nil
}

func (s *Service) findMFAsByField(field string, value string) ([]*MFA, error) {
	rows, err := s.findRowsByField(mfasTable, field, value)
	if err != nil {
		return nil, err
	}
	result := make([]*MFA, 0, len(rows))
	for _, row := range rows {
		result = append(result, mfaFromMap(row))
	}
	return result, nil
}

func (s *Service) findOTPsByField(field string, value string) ([]*OTP, error) {
	rows, err := s.findRowsByField(otpsTable, field, value)
	if err != nil {
		return nil, err
	}
	result := make([]*OTP, 0, len(rows))
	for _, row := range rows {
		result = append(result, otpFromMap(row))
	}
	return result, nil
}

func (s *Service) findAuthOriginsByField(field string, value string) ([]*AuthOrigin, error) {
	rows, err := s.findRowsByField(authOriginsTable, field, value)
	if err != nil {
		return nil, err
	}
	result := make([]*AuthOrigin, 0, len(rows))
	for _, row := range rows {
		result = append(result, authOriginFromMap(row))
	}
	return result, nil
}

func (s *Service) findRowsByField(tableName string, field string, value any) ([]map[string]any, error) {
	query := `SELECT * FROM ` + QuoteIdentifier(tableName) + ` WHERE ` + QuoteIdentifier(field) + ` = $1`
	rows, err := s.concurrentRunner().QueryContext(context.Background(), query, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func (s *Service) findRowByID(tableName string, id string) (map[string]any, error) {
	query := `SELECT * FROM ` + QuoteIdentifier(tableName) + ` WHERE "id" = $1 LIMIT 1`
	rows, err := s.concurrentRunner().QueryContext(context.Background(), query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, sql.ErrNoRows
	}
	return items[0], nil
}

func (s *Service) hasTable(ctx context.Context, db sqlRunner, tableName string) (bool, error) {
	if db == nil {
		return false, nil
	}

	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = current_schema()
			  AND table_name = $1
		)
	`, tableName).Scan(&exists)

	return exists, err
}

func (s *Service) concurrentRunner() sqlRunner {
	if s.concurrentTx != nil {
		return s.concurrentTx
	}
	return s.concurrentDB
}

func (s *Service) auxConcurrentRunner() sqlRunner {
	if s.auxConcurrentTx != nil {
		return s.auxConcurrentTx
	}
	return s.auxConcurrentDB
}

func (s *Service) modelTableName(model Model) string {
	switch v := model.(type) {
	case interface{ TableName() string }:
		return v.TableName()
	case string:
		return v
	case *Collection:
		if v != nil {
			return v.Name
		}
	}

	if model == nil {
		return ""
	}

	rt := reflect.TypeOf(model)
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if rt.Name() == "" {
		return ""
	}

	return toSnakeCase(rt.Name())
}

func (s *Service) recordTableName(collectionModelOrIdentifier any) string {
	return s.modelTableName(collectionModelOrIdentifier)
}

func (s *Service) recordFromMap(tableName string, values map[string]any) *Record {
	record := &Record{
		ID:     asString(values["id"]),
		Data:   values,
		Expand: map[string]any{},
	}
	if tableName != "" {
		record.Collection = &Collection{Name: tableName}
	}
	return record
}

func collectionFromMap(values map[string]any) *Collection {
	if values == nil {
		return nil
	}

	result := &Collection{
		ID:     asString(values["id"]),
		Name:   asString(values["name"]),
		Type:   asString(values["type"]),
		System: asBool(values["system"]),
		Meta:   map[string]any{},
	}

	if rawFields, ok := values["fields"]; ok {
		result.Fields = parseFieldsList(rawFields)
	}
	if rawMeta, ok := values["meta"]; ok {
		result.Meta = parseMap(rawMeta)
	}

	return result
}

func externalAuthFromMap(values map[string]any) *ExternalAuth {
	return &ExternalAuth{
		ID:            asString(values["id"]),
		CollectionRef: asString(values["collectionRef"]),
		RecordRef:     asString(values["recordRef"]),
		Provider:      asString(values["provider"]),
		ProviderID:    asString(values["providerId"]),
		Meta:          parseMap(values["meta"]),
	}
}

func mfaFromMap(values map[string]any) *MFA {
	return &MFA{
		ID:            asString(values["id"]),
		CollectionRef: asString(values["collectionRef"]),
		RecordRef:     asString(values["recordRef"]),
		Meta:          parseMap(values["meta"]),
	}
}

func otpFromMap(values map[string]any) *OTP {
	return &OTP{
		ID:            asString(values["id"]),
		CollectionRef: asString(values["collectionRef"]),
		RecordRef:     asString(values["recordRef"]),
		Meta:          parseMap(values["meta"]),
	}
}

func authOriginFromMap(values map[string]any) *AuthOrigin {
	return &AuthOrigin{
		ID:            asString(values["id"]),
		CollectionRef: asString(values["collectionRef"]),
		RecordRef:     asString(values["recordRef"]),
		Fingerprint:   asString(values["fingerprint"]),
		Meta:          parseMap(values["meta"]),
	}
}

func bindNamedParams(filter string, params ...Params) (string, []any, error) {
	if strings.TrimSpace(filter) == "" {
		return "", nil, nil
	}

	merged := Params{}
	for _, paramSet := range params {
		for k, v := range paramSet {
			merged[k] = v
		}
	}

	args := make([]any, 0)
	indexByName := map[string]int{}
	var missing error

	query := namedParamPattern.ReplaceAllStringFunc(filter, func(match string) string {
		name := strings.TrimSuffix(strings.TrimPrefix(match, "{:"), "}")
		if idx, ok := indexByName[name]; ok {
			return fmt.Sprintf("$%d", idx)
		}

		value, ok := merged[name]
		if !ok {
			missing = fmt.Errorf("dal: missing param %q", name)
			return match
		}

		args = append(args, value)
		indexByName[name] = len(args)
		return fmt.Sprintf("$%d", len(args))
	})

	return query, args, missing
}

func asString(v any) string {
	switch value := normalizeScannedValue(v).(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(value)
	}
}

func asBool(v any) bool {
	switch value := normalizeScannedValue(v).(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "t")
	case int64:
		return value != 0
	case int:
		return value != 0
	default:
		return false
	}
}

func parseFieldsList(v any) FieldsList {
	if v == nil {
		return nil
	}

	var out FieldsList
	switch value := normalizeScannedValue(v).(type) {
	case string:
		_ = json.Unmarshal([]byte(value), &out)
	case []byte:
		_ = json.Unmarshal(value, &out)
	}
	return out
}

func parseMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}

	out := map[string]any{}
	switch value := normalizeScannedValue(v).(type) {
	case map[string]any:
		return value
	case string:
		_ = json.Unmarshal([]byte(value), &out)
	case []byte:
		_ = json.Unmarshal(value, &out)
	}
	return out
}

func fieldReferencesCollection(field Field, collection *Collection) bool {
	if collection == nil {
		return false
	}

	if field.Meta == nil {
		return false
	}

	candidates := []string{
		asString(field.Meta["collection"]),
		asString(field.Meta["collectionId"]),
		asString(field.Meta["collectionName"]),
		asString(field.Meta["collectionRef"]),
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if candidate == collection.ID || strings.EqualFold(candidate, collection.Name) {
			return true
		}
	}

	return false
}

func requestHasAuth(info *RequestInfo) bool {
	if info == nil {
		return false
	}

	candidates := []string{"authorization", "auth", "token", "x_auth_token"}
	for _, key := range candidates {
		if v := strings.TrimSpace(info.Headers[key]); v != "" {
			return true
		}
		if v := strings.TrimSpace(info.Query[key]); v != "" {
			return true
		}
		if raw, ok := info.Body[key]; ok && truthy(raw) {
			return true
		}
	}

	return false
}

func truthy(v any) bool {
	switch value := normalizeScannedValue(v).(type) {
	case bool:
		return value
	case string:
		return value != "" && !strings.EqualFold(value, "false") && value != "0"
	case int:
		return value != 0
	case int64:
		return value != 0
	case float64:
		return value != 0
	case nil:
		return false
	default:
		return true
	}
}

func evalAccessRule(rule string, record *Record, info *RequestInfo) (bool, error) {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return true, nil
	}

	if inner, ok := unwrapParens(rule); ok {
		return evalAccessRule(inner, record, info)
	}

	if parts := splitTopLevel(rule, "||"); len(parts) > 1 {
		for _, part := range parts {
			ok, err := evalAccessRule(part, record, info)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}

	if parts := splitTopLevel(rule, "&&"); len(parts) > 1 {
		for _, part := range parts {
			ok, err := evalAccessRule(part, record, info)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	}

	if strings.HasPrefix(rule, "!") {
		ok, err := evalAccessRule(strings.TrimSpace(rule[1:]), record, info)
		if err != nil {
			return false, err
		}
		return !ok, nil
	}

	for _, op := range []string{"==", "!=", ">=", "<=", ">", "<", "="} {
		if idx := topLevelIndex(rule, op); idx >= 0 {
			left := strings.TrimSpace(rule[:idx])
			right := strings.TrimSpace(rule[idx+len(op):])
			return compareAccessRule(left, op, right, record, info), nil
		}
	}

	value, ok := lookupAccessValue(rule, record, info)
	if ok {
		return truthy(value), nil
	}

	switch strings.ToLower(rule) {
	case "auth":
		return requestHasAuth(info), nil
	case "true", "allow", "public":
		return true, nil
	case "false", "deny", "private":
		return false, nil
	default:
		return false, nil
	}
}

func compareAccessRule(left string, op string, right string, record *Record, info *RequestInfo) bool {
	leftValue, leftOK := lookupAccessValue(left, record, info)
	if !leftOK {
		leftValue = left
	}
	rightValue, rightOK := lookupAccessValue(right, record, info)
	if !rightOK {
		rightValue = literalAccessValue(right)
	}

	leftStr := asString(leftValue)
	rightStr := asString(rightValue)

	switch op {
	case "=", "==":
		return leftStr == rightStr
	case "!=":
		return leftStr != rightStr
	case ">":
		return leftStr > rightStr
	case "<":
		return leftStr < rightStr
	case ">=":
		return leftStr >= rightStr
	case "<=":
		return leftStr <= rightStr
	default:
		return false
	}
}

func lookupAccessValue(expr string, record *Record, info *RequestInfo) (any, bool) {
	expr = strings.TrimSpace(expr)
	switch {
	case strings.EqualFold(expr, "auth"):
		return requestHasAuth(info), true
	case strings.HasPrefix(strings.ToLower(expr), "header:"):
		if info == nil {
			return nil, false
		}
		key := strings.TrimSpace(expr[len("header:"):])
		value, ok := info.Headers[key]
		return value, ok
	case strings.HasPrefix(strings.ToLower(expr), "query:"):
		if info == nil {
			return nil, false
		}
		key := strings.TrimSpace(expr[len("query:"):])
		value, ok := info.Query[key]
		return value, ok
	case strings.HasPrefix(strings.ToLower(expr), "body:"):
		if info == nil {
			return nil, false
		}
		key := strings.TrimSpace(expr[len("body:"):])
		value, ok := info.Body[key]
		return value, ok
	case strings.HasPrefix(strings.ToLower(expr), "record."):
		if record == nil {
			return nil, false
		}
		key := strings.TrimSpace(expr[len("record."):])
		value, ok := record.Data[key]
		return value, ok
	default:
		return nil, false
	}
}

func literalAccessValue(raw string) any {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 {
		if (raw[0] == '\'' && raw[len(raw)-1] == '\'') || (raw[0] == '"' && raw[len(raw)-1] == '"') {
			return raw[1 : len(raw)-1]
		}
	}
	switch strings.ToLower(raw) {
	case "true":
		return true
	case "false":
		return false
	case "null", "nil":
		return nil
	default:
		return raw
	}
}

func splitTopLevel(input string, sep string) []string {
	var parts []string
	level := 0
	start := 0
	for i := 0; i <= len(input)-len(sep); i++ {
		switch input[i] {
		case '(':
			level++
		case ')':
			if level > 0 {
				level--
			}
		}

		if level == 0 && strings.HasPrefix(input[i:], sep) {
			parts = append(parts, strings.TrimSpace(input[start:i]))
			start = i + len(sep)
			i += len(sep) - 1
		}
	}

	if len(parts) == 0 {
		return nil
	}

	parts = append(parts, strings.TrimSpace(input[start:]))
	return parts
}

func topLevelIndex(input string, needle string) int {
	level := 0
	for i := 0; i <= len(input)-len(needle); i++ {
		switch input[i] {
		case '(':
			level++
		case ')':
			if level > 0 {
				level--
			}
		}
		if level == 0 && strings.HasPrefix(input[i:], needle) {
			return i
		}
	}
	return -1
}

func unwrapParens(input string) (string, bool) {
	input = strings.TrimSpace(input)
	if len(input) < 2 || input[0] != '(' || input[len(input)-1] != ')' {
		return "", false
	}

	level := 0
	for i, r := range input {
		switch r {
		case '(':
			level++
		case ')':
			level--
			if level == 0 && i != len(input)-1 {
				return "", false
			}
		}
	}

	if level != 0 {
		return "", false
	}

	return strings.TrimSpace(input[1 : len(input)-1]), true
}

func toSnakeCase(in string) string {
	var out strings.Builder
	for i, r := range in {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out.WriteByte('_')
		}
		out.WriteRune(r)
	}
	return strings.ToLower(out.String())
}

var _ Dal = (*Service)(nil)

func init() {
	_ = errors.Is(ErrNotImplemented, ErrNotImplemented)
}

func validateViewSelectQuery(query string) error {
	q := strings.TrimSpace(query)
	if q == "" {
		return fmt.Errorf("%w: empty query", ErrUnsafeViewQuery)
	}
	if strings.Contains(q, ";") {
		return fmt.Errorf("%w: multiple statements are not allowed", ErrUnsafeViewQuery)
	}

	lower := strings.ToLower(q)
	if !(strings.HasPrefix(lower, "select ") || strings.HasPrefix(lower, "with ")) {
		return fmt.Errorf("%w: query must start with SELECT or WITH", ErrUnsafeViewQuery)
	}

	for _, banned := range []string{" drop ", " alter ", " insert ", " update ", " delete ", " truncate ", " create table", " create index", "--", "/*", "*/"} {
		if strings.Contains(" "+lower+" ", banned) {
			return fmt.Errorf("%w: banned token %q", ErrUnsafeViewQuery, strings.TrimSpace(banned))
		}
	}

	return nil
}

func ValidateViewSelectQuery(query string) error {
	return validateViewSelectQuery(query)
}
