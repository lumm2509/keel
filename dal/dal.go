package dal

import "database/sql"

type Dal interface {
	ConcurrentDB() *sql.DB
	NonconcurrentDB() *sql.DB
	AuxConcurrentDB() *sql.DB
	AuxNonconcurrentDB() *sql.DB

	HasTable(tableName string) bool
	AuxHasTable(tableName string) bool
	TableColumns(tableName string) ([]string, error)
	TableInfo(tableName string) ([]*TableInfoRow, error)
	TableIndexes(tableName string) (map[string]string, error)
	FindRecordByViewFile(viewCollectionModelOrIdentifier any, fileFieldName string, filename string) (*Record, error)

	ModelQuery(model Model) *SelectQuery
	AuxModelQuery(model Model) *SelectQuery
	LogQuery() *SelectQuery
	CollectionQuery() *SelectQuery
	RecordQuery(collectionModelOrIdentifier any) *SelectQuery

	FindLogById(id string) (*Log, error)
	LogsStats(expr Expression) ([]*LogsStatsItem, error)

	FindAllCollections(collectionTypes ...string) ([]*Collection, error)
	FindCollectionByNameOrId(nameOrId string) (*Collection, error)
	FindCachedCollectionByNameOrId(nameOrId string) (*Collection, error)
	FindCollectionReferences(collection *Collection, excludeIds ...string) (map[*Collection][]Field, error)
	FindCachedCollectionReferences(collection *Collection, excludeIds ...string) (map[*Collection][]Field, error)
	IsCollectionNameUnique(name string, excludeIds ...string) bool

	FindAllExternalAuthsByRecord(authRecord *Record) ([]*ExternalAuth, error)
	FindAllExternalAuthsByCollection(collection *Collection) ([]*ExternalAuth, error)
	FindFirstExternalAuthByExpr(expr Expression) (*ExternalAuth, error)

	FindAllMFAsByRecord(authRecord *Record) ([]*MFA, error)
	FindAllMFAsByCollection(collection *Collection) ([]*MFA, error)
	FindMFAById(id string) (*MFA, error)

	FindAllOTPsByRecord(authRecord *Record) ([]*OTP, error)
	FindAllOTPsByCollection(collection *Collection) ([]*OTP, error)
	FindOTPById(id string) (*OTP, error)

	FindAllAuthOriginsByRecord(authRecord *Record) ([]*AuthOrigin, error)
	FindAllAuthOriginsByCollection(collection *Collection) ([]*AuthOrigin, error)
	FindAuthOriginById(id string) (*AuthOrigin, error)
	FindAuthOriginByRecordAndFingerprint(authRecord *Record, fingerprint string) (*AuthOrigin, error)

	FindRecordById(collectionModelOrIdentifier any, recordId string, optFilters ...QueryFilter) (*Record, error)
	FindRecordsByIds(collectionModelOrIdentifier any, recordIds []string, optFilters ...QueryFilter) ([]*Record, error)
	FindAllRecords(collectionModelOrIdentifier any, exprs ...Expression) ([]*Record, error)
	FindFirstRecordByData(collectionModelOrIdentifier any, key string, value any) (*Record, error)
	FindRecordsByFilter(collectionModelOrIdentifier any, filter string, sort string, limit int, offset int, params ...Params) ([]*Record, error)
	FindFirstRecordByFilter(collectionModelOrIdentifier any, filter string, params ...Params) (*Record, error)
	CountRecords(collectionModelOrIdentifier any, exprs ...Expression) (int64, error)
	FindAuthRecordByToken(token string, validTypes ...string) (*Record, error)
	FindAuthRecordByEmail(collectionModelOrIdentifier any, email string) (*Record, error)
	CanAccessRecord(record *Record, requestInfo *RequestInfo, accessRule *string) (bool, error)
}
