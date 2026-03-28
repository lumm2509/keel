package dml

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/lumm2509/keel/dal"
	"github.com/lumm2509/keel/runtime/hook"
)

var ErrNotImplemented = errors.New("dml: not implemented")
var ErrInvalidModel = errors.New("dml: invalid model")
var ErrUnsafeViewQuery = errors.New("dml: unsafe view query")

type Service struct {
	dal    DalProvider
	txMain *sql.Tx
	txAux  *sql.Tx

	onModelValidate                hook.Hook[*ModelEvent]
	onModelCreate                  hook.Hook[*ModelEvent]
	onModelCreateExecute           hook.Hook[*ModelEvent]
	onModelAfterCreateSuccess      hook.Hook[*ModelEvent]
	onModelAfterCreateError        hook.Hook[*ModelErrorEvent]
	onModelUpdate                  hook.Hook[*ModelEvent]
	onModelUpdateExecute           hook.Hook[*ModelEvent]
	onModelAfterUpdateSuccess      hook.Hook[*ModelEvent]
	onModelAfterUpdateError        hook.Hook[*ModelErrorEvent]
	onModelDelete                  hook.Hook[*ModelEvent]
	onModelDeleteExecute           hook.Hook[*ModelEvent]
	onModelAfterDeleteSuccess      hook.Hook[*ModelEvent]
	onModelAfterDeleteError        hook.Hook[*ModelErrorEvent]
	onRecordEnrich                 hook.Hook[*RecordEnrichEvent]
	onRecordValidate               hook.Hook[*RecordEvent]
	onRecordCreate                 hook.Hook[*RecordEvent]
	onRecordCreateExecute          hook.Hook[*RecordEvent]
	onRecordAfterCreateSuccess     hook.Hook[*RecordEvent]
	onRecordAfterCreateError       hook.Hook[*RecordErrorEvent]
	onRecordUpdate                 hook.Hook[*RecordEvent]
	onRecordUpdateExecute          hook.Hook[*RecordEvent]
	onRecordAfterUpdateSuccess     hook.Hook[*RecordEvent]
	onRecordAfterUpdateError       hook.Hook[*RecordErrorEvent]
	onRecordDelete                 hook.Hook[*RecordEvent]
	onRecordDeleteExecute          hook.Hook[*RecordEvent]
	onRecordAfterDeleteSuccess     hook.Hook[*RecordEvent]
	onRecordAfterDeleteError       hook.Hook[*RecordErrorEvent]
	onCollectionValidate           hook.Hook[*CollectionEvent]
	onCollectionCreate             hook.Hook[*CollectionEvent]
	onCollectionCreateExecute      hook.Hook[*CollectionEvent]
	onCollectionAfterCreateSuccess hook.Hook[*CollectionEvent]
	onCollectionAfterCreateError   hook.Hook[*CollectionErrorEvent]
	onCollectionUpdate             hook.Hook[*CollectionEvent]
	onCollectionUpdateExecute      hook.Hook[*CollectionEvent]
	onCollectionAfterUpdateSuccess hook.Hook[*CollectionEvent]
	onCollectionAfterUpdateError   hook.Hook[*CollectionErrorEvent]
	onCollectionDelete             hook.Hook[*CollectionEvent]
	onCollectionDeleteExecute      hook.Hook[*CollectionEvent]
	onCollectionAfterDeleteSuccess hook.Hook[*CollectionEvent]
	onCollectionAfterDeleteError   hook.Hook[*CollectionErrorEvent]
}

type DalProvider interface {
	DAL() dal.Dal
}

type runtimeApp struct {
	dal dal.Dal
	dml Dml
}

func (a *runtimeApp) DAL() dal.Dal { return a.dal }
func (a *runtimeApp) DML() Dml     { return a.dml }

func New(provider DalProvider) *Service {
	return &Service{dal: provider}
}

func NewApp(dao dal.Dal) App {
	mut := New(nil)
	app := &runtimeApp{dal: dao, dml: mut}
	mut.dal = app
	return app
}

func (s *Service) DeleteTable(dangerousTableName string) error {
	_, err := s.db().ExecContext(context.Background(), `DROP TABLE IF EXISTS `+dal.QuoteIdentifier(dangerousTableName)+` CASCADE`)
	return err
}

func (s *Service) DeleteView(dangerousViewName string) error {
	_, err := s.db().ExecContext(context.Background(), `DROP VIEW IF EXISTS `+dal.QuoteIdentifier(dangerousViewName)+` CASCADE`)
	return err
}

func (s *Service) saveView(dangerousViewName string, dangerousSelectQuery string) error {
	if err := validateViewDefinition(dangerousViewName, dangerousSelectQuery); err != nil {
		return err
	}
	_, err := s.db().ExecContext(context.Background(), `CREATE OR REPLACE VIEW `+dal.QuoteIdentifier(dangerousViewName)+` AS `+dangerousSelectQuery)
	return err
}

func (s *Service) Vacuum() error {
	_, err := s.db().ExecContext(context.Background(), `VACUUM`)
	return err
}

func (s *Service) AuxVacuum() error {
	_, err := s.auxDB().ExecContext(context.Background(), `VACUUM`)
	return err
}

func (s *Service) Delete(model Model) error { return s.DeleteWithContext(context.Background(), model) }

func (s *Service) DeleteWithContext(ctx context.Context, model Model) error {
	return s.deleteWithDB(ctx, s.db(), model)
}

func (s *Service) AuxDelete(model Model) error {
	return s.AuxDeleteWithContext(context.Background(), model)
}

func (s *Service) AuxDeleteWithContext(ctx context.Context, model Model) error {
	return s.deleteWithDB(ctx, s.auxDB(), model)
}

func (s *Service) Save(model Model) error { return s.SaveWithContext(context.Background(), model) }

func (s *Service) SaveWithContext(ctx context.Context, model Model) error {
	if err := s.ValidateWithContext(ctx, model); err != nil {
		return err
	}

	return s.saveWithDB(ctx, s.db(), model)
}

func (s *Service) SaveNoValidate(model Model) error {
	return s.SaveNoValidateWithContext(context.Background(), model)
}

func (s *Service) SaveNoValidateWithContext(ctx context.Context, model Model) error {
	return s.saveWithDB(ctx, s.db(), model)
}

func (s *Service) AuxSave(model Model) error {
	return s.AuxSaveWithContext(context.Background(), model)
}

func (s *Service) AuxSaveWithContext(ctx context.Context, model Model) error {
	if err := s.ValidateWithContext(ctx, model); err != nil {
		return err
	}

	return s.saveWithDB(ctx, s.auxDB(), model)
}

func (s *Service) AuxSaveNoValidate(model Model) error {
	return s.AuxSaveNoValidateWithContext(context.Background(), model)
}

func (s *Service) AuxSaveNoValidateWithContext(ctx context.Context, model Model) error {
	return s.saveWithDB(ctx, s.auxDB(), model)
}
func (s *Service) Validate(model Model) error {
	return s.ValidateWithContext(context.Background(), model)
}

func (s *Service) ValidateWithContext(ctx context.Context, model Model) error {
	_ = ctx

	table, values, idPtr, err := modelPersistenceParts(model)
	if err != nil {
		return err
	}
	if strings.TrimSpace(table) == "" {
		return fmt.Errorf("%w: missing table name", ErrInvalidModel)
	}
	if idPtr == nil {
		return fmt.Errorf("%w: missing model id", ErrInvalidModel)
	}
	if strings.TrimSpace(*idPtr) == "" {
		return fmt.Errorf("%w: empty model id", ErrInvalidModel)
	}

	switch m := model.(type) {
	case *dal.Record:
		if m == nil {
			return fmt.Errorf("%w: nil record", ErrInvalidModel)
		}
		if m.Collection == nil || strings.TrimSpace(m.Collection.Name) == "" {
			return fmt.Errorf("%w: record collection is required", ErrInvalidModel)
		}
	case *dal.Collection:
		if m == nil {
			return fmt.Errorf("%w: nil collection", ErrInvalidModel)
		}
		if strings.TrimSpace(m.Name) == "" {
			return fmt.Errorf("%w: collection name is required", ErrInvalidModel)
		}
		if err := validateCollectionFields(m.Fields); err != nil {
			return err
		}
	}

	for key := range values {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%w: empty field name", ErrInvalidModel)
		}
	}

	return nil
}

func (s *Service) RunInTransaction(fn func(txApp App) error) error {
	return s.runInTransaction(s.db(), s.auxDB(), fn)
}

func (s *Service) AuxRunInTransaction(fn func(txApp App) error) error {
	return s.runInTransaction(s.auxDB(), s.auxDB(), fn)
}

func (s *Service) DeleteOldLogs(createdBefore time.Time) error {
	_, err := s.db().ExecContext(context.Background(), `DELETE FROM "logs" WHERE "created" < $1`, createdBefore)
	return err
}

func (s *Service) ReloadCachedCollections() error { return nil }

func (s *Service) TruncateCollection(collection *Collection) error {
	if collection == nil {
		return sql.ErrNoRows
	}

	_, err := s.db().ExecContext(context.Background(), `TRUNCATE TABLE `+dal.QuoteIdentifier(collection.Name)+` RESTART IDENTITY CASCADE`)
	return err
}

func (s *Service) ImportCollections(toImport []map[string]any, deleteMissing bool) error {
	seen := map[string]struct{}{}

	for _, item := range toImport {
		collection, err := collectionFromPayload(item)
		if err != nil {
			return err
		}

		seen[collection.ID] = struct{}{}
		seen[collection.Name] = struct{}{}

		var existing *Collection
		if collection.ID != "" {
			existing, _ = s.dal.DAL().FindCollectionByNameOrId(collection.ID)
		}
		if existing == nil && collection.Name != "" {
			existing, _ = s.dal.DAL().FindCollectionByNameOrId(collection.Name)
		}

		if err := s.Save(collection); err != nil {
			return err
		}
		if err := s.SyncRecordTableSchema(collection, existing); err != nil {
			return err
		}
	}

	if deleteMissing {
		existingCollections, err := s.dal.DAL().FindAllCollections()
		if err != nil {
			return err
		}

		for _, collection := range existingCollections {
			if collection == nil {
				continue
			}
			if _, ok := seen[collection.ID]; ok {
				continue
			}
			if _, ok := seen[collection.Name]; ok {
				continue
			}

			if err := s.Delete(collection); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
	}

	return nil
}

func (s *Service) ImportCollectionsByMarshaledJSON(rawSliceOfMaps []byte, deleteMissing bool) error {
	var payload []map[string]any
	if err := json.Unmarshal(rawSliceOfMaps, &payload); err != nil {
		return err
	}

	return s.ImportCollections(payload, deleteMissing)
}

func (s *Service) SyncRecordTableSchema(newCollection *Collection, oldCollection *Collection) error {
	if newCollection == nil {
		return sql.ErrNoRows
	}

	db := s.db()
	if db == nil {
		return sql.ErrConnDone
	}

	if oldCollection == nil || oldCollection.Name == "" {
		columns := slices.Concat([]string{`"id" text PRIMARY KEY`}, buildColumnDefinitions(newCollection.Fields))
		_, err := db.ExecContext(context.Background(), `CREATE TABLE IF NOT EXISTS `+dal.QuoteIdentifier(newCollection.Name)+` (`+strings.Join(columns, ", ")+`)`)
		if err != nil {
			return err
		}
		return s.syncIndexes(newCollection.Name, nil, newCollection)
	}

	if oldCollection.Name != newCollection.Name {
		if _, err := db.ExecContext(context.Background(), `ALTER TABLE `+dal.QuoteIdentifier(oldCollection.Name)+` RENAME TO `+dal.QuoteIdentifier(newCollection.Name)); err != nil {
			return err
		}
	}

	existingColumns, err := s.dal.DAL().TableColumns(newCollection.Name)
	if err != nil {
		return err
	}

	existing := map[string]struct{}{}
	for _, c := range existingColumns {
		existing[c] = struct{}{}
	}

	oldByID := map[string]dal.Field{}
	oldByName := map[string]dal.Field{}
	for _, field := range oldCollection.Fields {
		if field.ID != "" {
			oldByID[field.ID] = field
		}
		if field.Name != "" {
			oldByName[field.Name] = field
		}
	}

	newNames := map[string]struct{}{"id": {}}
	for _, field := range newCollection.Fields {
		if field.Name == "" || field.Name == "id" {
			continue
		}
		newNames[field.Name] = struct{}{}

		if oldField, ok := oldByID[field.ID]; ok && oldField.Name != "" && oldField.Name != field.Name {
			if _, hasOld := existing[oldField.Name]; hasOld {
				if _, hasNew := existing[field.Name]; !hasNew {
					_, err := db.ExecContext(context.Background(), `ALTER TABLE `+dal.QuoteIdentifier(newCollection.Name)+` RENAME COLUMN `+dal.QuoteIdentifier(oldField.Name)+` TO `+dal.QuoteIdentifier(field.Name))
					if err != nil {
						return err
					}
					delete(existing, oldField.Name)
					existing[field.Name] = struct{}{}
				}
			}
		}

		if _, ok := existing[field.Name]; ok {
			continue
		}
		_, err := db.ExecContext(context.Background(), `ALTER TABLE `+dal.QuoteIdentifier(newCollection.Name)+` ADD COLUMN `+dal.QuoteIdentifier(field.Name)+` `+sqlTypeForField(field.Type))
		if err != nil {
			return err
		}
	}

	for _, column := range existingColumns {
		if _, keep := newNames[column]; keep {
			continue
		}
		if _, keep := oldByName[column]; keep {
			if _, stillPresent := newNames[column]; stillPresent {
				continue
			}
		}
		if column == "id" {
			continue
		}
		if _, err := db.ExecContext(context.Background(), `ALTER TABLE `+dal.QuoteIdentifier(newCollection.Name)+` DROP COLUMN IF EXISTS `+dal.QuoteIdentifier(column)+` CASCADE`); err != nil {
			return err
		}
	}

	return s.syncIndexes(newCollection.Name, oldCollection, newCollection)
}

func (s *Service) DeleteAllMFAsByRecord(authRecord *Record) error {
	return s.deleteByRecordRef("_mfas", authRecord)
}

func (s *Service) DeleteExpiredMFAs() error {
	return s.deleteExpired("_mfas")
}

func (s *Service) DeleteAllOTPsByRecord(authRecord *Record) error {
	return s.deleteByRecordRef("_otps", authRecord)
}

func (s *Service) DeleteExpiredOTPs() error {
	return s.deleteExpired("_otps")
}

func (s *Service) DeleteAllAuthOriginsByRecord(authRecord *Record) error {
	return s.deleteByRecordRef("_authOrigins", authRecord)
}
func (s *Service) ExpandRecord(record *Record, expands []string, optFetchFunc ExpandFetchFunc) map[string]error {
	if record == nil {
		return map[string]error{"*": sql.ErrNoRows}
	}
	return s.expandRecords([]*Record{record}, expands, optFetchFunc)
}
func (s *Service) ExpandRecords(records []*Record, expands []string, optFetchFunc ExpandFetchFunc) map[string]error {
	return s.expandRecords(records, expands, optFetchFunc)
}

func (s *Service) OnModelValidate(tags ...string) *hook.TaggedHook[*ModelEvent] {
	return hook.NewTaggedHook(&s.onModelValidate, tags...)
}
func (s *Service) OnModelCreate(tags ...string) *hook.TaggedHook[*ModelEvent] {
	return hook.NewTaggedHook(&s.onModelCreate, tags...)
}
func (s *Service) OnModelCreateExecute(tags ...string) *hook.TaggedHook[*ModelEvent] {
	return hook.NewTaggedHook(&s.onModelCreateExecute, tags...)
}
func (s *Service) OnModelAfterCreateSuccess(tags ...string) *hook.TaggedHook[*ModelEvent] {
	return hook.NewTaggedHook(&s.onModelAfterCreateSuccess, tags...)
}
func (s *Service) OnModelAfterCreateError(tags ...string) *hook.TaggedHook[*ModelErrorEvent] {
	return hook.NewTaggedHook(&s.onModelAfterCreateError, tags...)
}
func (s *Service) OnModelUpdate(tags ...string) *hook.TaggedHook[*ModelEvent] {
	return hook.NewTaggedHook(&s.onModelUpdate, tags...)
}
func (s *Service) OnModelUpdateExecute(tags ...string) *hook.TaggedHook[*ModelEvent] {
	return hook.NewTaggedHook(&s.onModelUpdateExecute, tags...)
}
func (s *Service) OnModelAfterUpdateSuccess(tags ...string) *hook.TaggedHook[*ModelEvent] {
	return hook.NewTaggedHook(&s.onModelAfterUpdateSuccess, tags...)
}
func (s *Service) OnModelAfterUpdateError(tags ...string) *hook.TaggedHook[*ModelErrorEvent] {
	return hook.NewTaggedHook(&s.onModelAfterUpdateError, tags...)
}
func (s *Service) OnModelDelete(tags ...string) *hook.TaggedHook[*ModelEvent] {
	return hook.NewTaggedHook(&s.onModelDelete, tags...)
}
func (s *Service) OnModelDeleteExecute(tags ...string) *hook.TaggedHook[*ModelEvent] {
	return hook.NewTaggedHook(&s.onModelDeleteExecute, tags...)
}
func (s *Service) OnModelAfterDeleteSuccess(tags ...string) *hook.TaggedHook[*ModelEvent] {
	return hook.NewTaggedHook(&s.onModelAfterDeleteSuccess, tags...)
}
func (s *Service) OnModelAfterDeleteError(tags ...string) *hook.TaggedHook[*ModelErrorEvent] {
	return hook.NewTaggedHook(&s.onModelAfterDeleteError, tags...)
}
func (s *Service) OnRecordEnrich(tags ...string) *hook.TaggedHook[*RecordEnrichEvent] {
	return hook.NewTaggedHook(&s.onRecordEnrich, tags...)
}
func (s *Service) OnRecordValidate(tags ...string) *hook.TaggedHook[*RecordEvent] {
	return hook.NewTaggedHook(&s.onRecordValidate, tags...)
}
func (s *Service) OnRecordCreate(tags ...string) *hook.TaggedHook[*RecordEvent] {
	return hook.NewTaggedHook(&s.onRecordCreate, tags...)
}
func (s *Service) OnRecordCreateExecute(tags ...string) *hook.TaggedHook[*RecordEvent] {
	return hook.NewTaggedHook(&s.onRecordCreateExecute, tags...)
}
func (s *Service) OnRecordAfterCreateSuccess(tags ...string) *hook.TaggedHook[*RecordEvent] {
	return hook.NewTaggedHook(&s.onRecordAfterCreateSuccess, tags...)
}
func (s *Service) OnRecordAfterCreateError(tags ...string) *hook.TaggedHook[*RecordErrorEvent] {
	return hook.NewTaggedHook(&s.onRecordAfterCreateError, tags...)
}
func (s *Service) OnRecordUpdate(tags ...string) *hook.TaggedHook[*RecordEvent] {
	return hook.NewTaggedHook(&s.onRecordUpdate, tags...)
}
func (s *Service) OnRecordUpdateExecute(tags ...string) *hook.TaggedHook[*RecordEvent] {
	return hook.NewTaggedHook(&s.onRecordUpdateExecute, tags...)
}
func (s *Service) OnRecordAfterUpdateSuccess(tags ...string) *hook.TaggedHook[*RecordEvent] {
	return hook.NewTaggedHook(&s.onRecordAfterUpdateSuccess, tags...)
}
func (s *Service) OnRecordAfterUpdateError(tags ...string) *hook.TaggedHook[*RecordErrorEvent] {
	return hook.NewTaggedHook(&s.onRecordAfterUpdateError, tags...)
}
func (s *Service) OnRecordDelete(tags ...string) *hook.TaggedHook[*RecordEvent] {
	return hook.NewTaggedHook(&s.onRecordDelete, tags...)
}
func (s *Service) OnRecordDeleteExecute(tags ...string) *hook.TaggedHook[*RecordEvent] {
	return hook.NewTaggedHook(&s.onRecordDeleteExecute, tags...)
}
func (s *Service) OnRecordAfterDeleteSuccess(tags ...string) *hook.TaggedHook[*RecordEvent] {
	return hook.NewTaggedHook(&s.onRecordAfterDeleteSuccess, tags...)
}
func (s *Service) OnRecordAfterDeleteError(tags ...string) *hook.TaggedHook[*RecordErrorEvent] {
	return hook.NewTaggedHook(&s.onRecordAfterDeleteError, tags...)
}
func (s *Service) OnCollectionValidate(tags ...string) *hook.TaggedHook[*CollectionEvent] {
	return hook.NewTaggedHook(&s.onCollectionValidate, tags...)
}
func (s *Service) OnCollectionCreate(tags ...string) *hook.TaggedHook[*CollectionEvent] {
	return hook.NewTaggedHook(&s.onCollectionCreate, tags...)
}
func (s *Service) OnCollectionCreateExecute(tags ...string) *hook.TaggedHook[*CollectionEvent] {
	return hook.NewTaggedHook(&s.onCollectionCreateExecute, tags...)
}
func (s *Service) OnCollectionAfterCreateSuccess(tags ...string) *hook.TaggedHook[*CollectionEvent] {
	return hook.NewTaggedHook(&s.onCollectionAfterCreateSuccess, tags...)
}
func (s *Service) OnCollectionAfterCreateError(tags ...string) *hook.TaggedHook[*CollectionErrorEvent] {
	return hook.NewTaggedHook(&s.onCollectionAfterCreateError, tags...)
}
func (s *Service) OnCollectionUpdate(tags ...string) *hook.TaggedHook[*CollectionEvent] {
	return hook.NewTaggedHook(&s.onCollectionUpdate, tags...)
}
func (s *Service) OnCollectionUpdateExecute(tags ...string) *hook.TaggedHook[*CollectionEvent] {
	return hook.NewTaggedHook(&s.onCollectionUpdateExecute, tags...)
}
func (s *Service) OnCollectionAfterUpdateSuccess(tags ...string) *hook.TaggedHook[*CollectionEvent] {
	return hook.NewTaggedHook(&s.onCollectionAfterUpdateSuccess, tags...)
}
func (s *Service) OnCollectionAfterUpdateError(tags ...string) *hook.TaggedHook[*CollectionErrorEvent] {
	return hook.NewTaggedHook(&s.onCollectionAfterUpdateError, tags...)
}
func (s *Service) OnCollectionDelete(tags ...string) *hook.TaggedHook[*CollectionEvent] {
	return hook.NewTaggedHook(&s.onCollectionDelete, tags...)
}
func (s *Service) OnCollectionDeleteExecute(tags ...string) *hook.TaggedHook[*CollectionEvent] {
	return hook.NewTaggedHook(&s.onCollectionDeleteExecute, tags...)
}
func (s *Service) OnCollectionAfterDeleteSuccess(tags ...string) *hook.TaggedHook[*CollectionEvent] {
	return hook.NewTaggedHook(&s.onCollectionAfterDeleteSuccess, tags...)
}
func (s *Service) OnCollectionAfterDeleteError(tags ...string) *hook.TaggedHook[*CollectionErrorEvent] {
	return hook.NewTaggedHook(&s.onCollectionAfterDeleteError, tags...)
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *Service) db() *sql.DB {
	if s.dal == nil || s.dal.DAL() == nil {
		return nil
	}
	return s.dal.DAL().ConcurrentDB()
}

func (s *Service) auxDB() *sql.DB {
	if s.dal == nil || s.dal.DAL() == nil {
		return nil
	}
	return s.dal.DAL().AuxConcurrentDB()
}

func (s *Service) runInTransaction(primary *sql.DB, aux *sql.DB, fn func(txApp App) error) error {
	if fn == nil {
		return nil
	}
	if primary == nil {
		return sql.ErrConnDone
	}

	mainTx, err := primary.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}

	auxTx := mainTx
	if aux != nil && aux != primary {
		auxTx, err = aux.BeginTx(context.Background(), nil)
		if err != nil {
			_ = mainTx.Rollback()
			return err
		}
	}

	baseDAL, ok := s.dal.DAL().(*dal.Service)
	if !ok {
		_ = mainTx.Rollback()
		if auxTx != mainTx {
			_ = auxTx.Rollback()
		}
		return fmt.Errorf("dml: dal implementation %T cannot be transaction-cloned", s.dal.DAL())
	}

	txDAL := baseDAL.WithTransactions(mainTx, auxTx)
	txDML := &Service{dal: &runtimeApp{dal: txDAL}, txMain: mainTx, txAux: auxTx}
	s.copyHooks(txDML)

	err = fn(&runtimeApp{dal: txDAL, dml: txDML})
	if err != nil {
		_ = mainTx.Rollback()
		if auxTx != mainTx {
			_ = auxTx.Rollback()
		}
		return err
	}

	if auxTx != mainTx {
		if err := auxTx.Commit(); err != nil {
			_ = mainTx.Rollback()
			return err
		}
	}

	return mainTx.Commit()
}

func (s *Service) copyHooks(dst *Service) {
	dst.onModelValidate = s.onModelValidate
	dst.onModelCreate = s.onModelCreate
	dst.onModelCreateExecute = s.onModelCreateExecute
	dst.onModelAfterCreateSuccess = s.onModelAfterCreateSuccess
	dst.onModelAfterCreateError = s.onModelAfterCreateError
	dst.onModelUpdate = s.onModelUpdate
	dst.onModelUpdateExecute = s.onModelUpdateExecute
	dst.onModelAfterUpdateSuccess = s.onModelAfterUpdateSuccess
	dst.onModelAfterUpdateError = s.onModelAfterUpdateError
	dst.onModelDelete = s.onModelDelete
	dst.onModelDeleteExecute = s.onModelDeleteExecute
	dst.onModelAfterDeleteSuccess = s.onModelAfterDeleteSuccess
	dst.onModelAfterDeleteError = s.onModelAfterDeleteError
	dst.onRecordEnrich = s.onRecordEnrich
	dst.onRecordValidate = s.onRecordValidate
	dst.onRecordCreate = s.onRecordCreate
	dst.onRecordCreateExecute = s.onRecordCreateExecute
	dst.onRecordAfterCreateSuccess = s.onRecordAfterCreateSuccess
	dst.onRecordAfterCreateError = s.onRecordAfterCreateError
	dst.onRecordUpdate = s.onRecordUpdate
	dst.onRecordUpdateExecute = s.onRecordUpdateExecute
	dst.onRecordAfterUpdateSuccess = s.onRecordAfterUpdateSuccess
	dst.onRecordAfterUpdateError = s.onRecordAfterUpdateError
	dst.onRecordDelete = s.onRecordDelete
	dst.onRecordDeleteExecute = s.onRecordDeleteExecute
	dst.onRecordAfterDeleteSuccess = s.onRecordAfterDeleteSuccess
	dst.onRecordAfterDeleteError = s.onRecordAfterDeleteError
	dst.onCollectionValidate = s.onCollectionValidate
	dst.onCollectionCreate = s.onCollectionCreate
	dst.onCollectionCreateExecute = s.onCollectionCreateExecute
	dst.onCollectionAfterCreateSuccess = s.onCollectionAfterCreateSuccess
	dst.onCollectionAfterCreateError = s.onCollectionAfterCreateError
	dst.onCollectionUpdate = s.onCollectionUpdate
	dst.onCollectionUpdateExecute = s.onCollectionUpdateExecute
	dst.onCollectionAfterUpdateSuccess = s.onCollectionAfterUpdateSuccess
	dst.onCollectionAfterUpdateError = s.onCollectionAfterUpdateError
	dst.onCollectionDelete = s.onCollectionDelete
	dst.onCollectionDeleteExecute = s.onCollectionDeleteExecute
	dst.onCollectionAfterDeleteSuccess = s.onCollectionAfterDeleteSuccess
	dst.onCollectionAfterDeleteError = s.onCollectionAfterDeleteError
}

var _ Dml = (*Service)(nil)

func init() {
	_ = strings.TrimSpace("")
}

func (s *Service) saveWithDB(ctx context.Context, db *sql.DB, model Model) error {
	if db == nil {
		return sql.ErrConnDone
	}
	runner := s.runnerForDB(db)

	table, values, idPtr, err := modelPersistenceParts(model)
	if err != nil {
		return err
	}
	if table == "" {
		return fmt.Errorf("%w: missing table name", ErrInvalidModel)
	}

	id := ""
	if idPtr != nil {
		id = strings.TrimSpace(*idPtr)
	}

	isCreate := id == ""
	eventApp := &runtimeApp{dal: s.dal.DAL(), dml: s}
	modelEvent := &ModelEvent{modelEventBase: modelEventBase{App: eventApp}, Model: model}

	if err := s.onModelValidate.Trigger(modelEvent, func(e *ModelEvent) error { return e.Next() }); err != nil {
		return err
	}

	if isCreate {
		if err := s.onModelCreate.Trigger(modelEvent, func(e *ModelEvent) error { return e.Next() }); err != nil {
			return err
		}
	} else {
		if err := s.onModelUpdate.Trigger(modelEvent, func(e *ModelEvent) error { return e.Next() }); err != nil {
			return err
		}
	}

	if id == "" {
		return fmt.Errorf("dml: model id is required")
	}

	if isCreate {
		cols := make([]string, 0, len(values))
		placeholders := make([]string, 0, len(values))
		args := make([]any, 0, len(values))
		index := 1
		for key, value := range values {
			cols = append(cols, dal.QuoteIdentifier(key))
			placeholders = append(placeholders, fmt.Sprintf("$%d", index))
			args = append(args, value)
			index++
		}

		query := `INSERT INTO ` + dal.QuoteIdentifier(table) + ` (` + strings.Join(cols, ", ") + `) VALUES (` + strings.Join(placeholders, ", ") + `)`
		if err := s.triggerModelExec(isCreate, modelEvent, func() error {
			_, err := runner.ExecContext(ctx, query, args...)
			return err
		}); err != nil {
			return err
		}
		return s.triggerModelAfterSuccess(isCreate, modelEvent)
	}

	assignments := make([]string, 0, len(values))
	args := make([]any, 0, len(values))
	index := 1
	for key, value := range values {
		if key == "id" {
			continue
		}
		assignments = append(assignments, dal.QuoteIdentifier(key)+fmt.Sprintf(" = $%d", index))
		args = append(args, value)
		index++
	}
	args = append(args, id)

	query := `UPDATE ` + dal.QuoteIdentifier(table) + ` SET ` + strings.Join(assignments, ", ") + fmt.Sprintf(` WHERE "id" = $%d`, len(args))
	if err := s.triggerModelExec(false, modelEvent, func() error {
		_, err := runner.ExecContext(ctx, query, args...)
		return err
	}); err != nil {
		return err
	}
	return s.triggerModelAfterSuccess(false, modelEvent)
}

func (s *Service) deleteWithDB(ctx context.Context, db *sql.DB, model Model) error {
	if db == nil {
		return sql.ErrConnDone
	}
	runner := s.runnerForDB(db)

	table, _, idPtr, err := modelPersistenceParts(model)
	if err != nil {
		return err
	}
	if table == "" || idPtr == nil || strings.TrimSpace(*idPtr) == "" {
		return fmt.Errorf("dml: model id is required")
	}

	eventApp := &runtimeApp{dal: s.dal.DAL(), dml: s}
	modelEvent := &ModelEvent{modelEventBase: modelEventBase{App: eventApp}, Model: model}
	if err := s.onModelDelete.Trigger(modelEvent, func(e *ModelEvent) error { return e.Next() }); err != nil {
		return err
	}

	err = s.onModelDeleteExecute.Trigger(modelEvent, func(e *ModelEvent) error {
		_, execErr := runner.ExecContext(ctx, `DELETE FROM `+dal.QuoteIdentifier(table)+` WHERE "id" = $1`, *idPtr)
		if execErr != nil {
			return execErr
		}
		return e.Next()
	})
	if err != nil {
		_ = s.onModelAfterDeleteError.Trigger(&ModelErrorEvent{ModelEvent: *modelEvent, Err: err}, func(e *ModelErrorEvent) error { return e.Next() })
		return err
	}

	return s.onModelAfterDeleteSuccess.Trigger(modelEvent, func(e *ModelEvent) error { return e.Next() })
}

func (s *Service) triggerModelExec(isCreate bool, event *ModelEvent, op func() error) error {
	trigger := s.onModelUpdateExecute.Trigger
	errTrigger := s.onModelAfterUpdateError.Trigger
	if isCreate {
		trigger = s.onModelCreateExecute.Trigger
		errTrigger = s.onModelAfterCreateError.Trigger
	}

	err := trigger(event, func(e *ModelEvent) error {
		if err := op(); err != nil {
			return err
		}
		return e.Next()
	})
	if err != nil {
		_ = errTrigger(&ModelErrorEvent{ModelEvent: *event, Err: err}, func(e *ModelErrorEvent) error { return e.Next() })
	}
	return err
}

func (s *Service) triggerModelAfterSuccess(isCreate bool, event *ModelEvent) error {
	trigger := s.onModelAfterUpdateSuccess.Trigger
	if isCreate {
		trigger = s.onModelAfterCreateSuccess.Trigger
	}
	return trigger(event, func(e *ModelEvent) error { return e.Next() })
}

func modelPersistenceParts(model Model) (string, map[string]any, *string, error) {
	switch m := model.(type) {
	case *dal.Record:
		if m == nil {
			return "", nil, nil, sql.ErrNoRows
		}
		if m.Data == nil {
			m.Data = map[string]any{}
		}
		m.Data["id"] = m.ID
		table := ""
		if m.Collection != nil {
			table = m.Collection.Name
		}
		return table, m.Data, &m.ID, nil
	case *dal.Collection:
		if m == nil {
			return "", nil, nil, sql.ErrNoRows
		}
		values := map[string]any{
			"id":     m.ID,
			"name":   m.Name,
			"type":   m.Type,
			"system": m.System,
			"fields": jsonOrValue(m.Fields),
			"meta":   jsonOrValue(m.Meta),
		}
		return "_collections", values, &m.ID, nil
	}

	rv := reflect.ValueOf(model)
	if !rv.IsValid() {
		return "", nil, nil, sql.ErrNoRows
	}
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return "", nil, nil, fmt.Errorf("dml: model must be a non-nil pointer")
	}

	table := detectTableName(model)
	if table == "" {
		return "", nil, nil, nil
	}

	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return "", nil, nil, fmt.Errorf("%w: model must point to a struct", ErrInvalidModel)
	}

	values := map[string]any{}
	var idPtr *string

	rt := elem.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}

		column := field.Tag.Get("db")
		if column == "" {
			column = strings.Split(field.Tag.Get("json"), ",")[0]
		}
		if column == "" {
			column = toSnakeCase(field.Name)
		}
		if column == "-" || column == "" {
			continue
		}

		value := elem.Field(i)
		if field.Name == "ID" && value.Kind() == reflect.String && value.CanAddr() {
			ptr := value.Addr().Interface().(*string)
			idPtr = ptr
		}
		values[column] = serializeModelValue(value.Interface())
	}

	if idPtr == nil {
		return "", nil, nil, fmt.Errorf("dml: model %T must have an exported string ID field", model)
	}

	return table, values, idPtr, nil
}

func detectTableName(model Model) string {
	switch v := model.(type) {
	case interface{ TableName() string }:
		return v.TableName()
	}

	rt := reflect.TypeOf(model)
	for rt != nil && rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if rt == nil || rt.Name() == "" {
		return ""
	}
	return toSnakeCase(rt.Name())
}

func serializeModelValue(v any) any {
	switch value := v.(type) {
	case map[string]any, []any, []string, dal.FieldsList:
		return jsonOrValue(value)
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array, reflect.Struct:
		if _, ok := v.(time.Time); ok {
			return v
		}
		return jsonOrValue(v)
	default:
		return v
	}
}

func jsonOrValue(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	return raw
}

func buildColumnDefinitions(fields dal.FieldsList) []string {
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.Name == "" || field.Name == "id" {
			continue
		}
		result = append(result, dal.QuoteIdentifier(field.Name)+" "+sqlTypeForField(field.Type))
	}
	return result
}

func desiredIndexes(collection *Collection) map[string]string {
	result := map[string]string{}
	if collection == nil {
		return result
	}

	for _, field := range collection.Fields {
		if field.Name == "" {
			continue
		}

		if field.Meta != nil && boolValue(field.Meta["unique"]) {
			name := indexName(collection.Name, field.Name, true)
			result[name] = `CREATE UNIQUE INDEX ` + dal.QuoteIdentifier(name) + ` ON ` + dal.QuoteIdentifier(collection.Name) + ` (` + dal.QuoteIdentifier(field.Name) + `)`
		} else if field.Meta != nil && boolValue(field.Meta["index"]) {
			name := indexName(collection.Name, field.Name, false)
			result[name] = `CREATE INDEX ` + dal.QuoteIdentifier(name) + ` ON ` + dal.QuoteIdentifier(collection.Name) + ` (` + dal.QuoteIdentifier(field.Name) + `)`
		}
	}

	if collection.Meta == nil {
		return result
	}

	rawIndexes, ok := collection.Meta["indexes"]
	if !ok {
		return result
	}

	switch value := rawIndexes.(type) {
	case []any:
		for i, item := range value {
			if sql := explicitIndexSQL(collection.Name, item, i); sql != "" {
				name := explicitIndexName(collection.Name, item, i)
				result[name] = sql
			}
		}
	case []string:
		for i, item := range value {
			if sql := explicitIndexSQL(collection.Name, item, i); sql != "" {
				name := explicitIndexName(collection.Name, item, i)
				result[name] = sql
			}
		}
	}

	return result
}

func explicitIndexSQL(tableName string, item any, i int) string {
	switch value := item.(type) {
	case string:
		value = strings.TrimSpace(value)
		if value == "" {
			return ""
		}
		if strings.HasPrefix(strings.ToUpper(value), "CREATE ") {
			return value
		}
		name := fmt.Sprintf("idx_%s_custom_%d", sanitizeIdentifier(tableName), i+1)
		return `CREATE INDEX ` + dal.QuoteIdentifier(name) + ` ON ` + dal.QuoteIdentifier(tableName) + ` (` + value + `)`
	case map[string]any:
		cols := stringValue(value["columns"])
		if cols == "" {
			return ""
		}
		unique := boolValue(value["unique"])
		name := stringValue(value["name"])
		if name == "" {
			name = indexName(tableName, cols, unique)
		}
		prefix := "CREATE INDEX "
		if unique {
			prefix = "CREATE UNIQUE INDEX "
		}
		return prefix + dal.QuoteIdentifier(name) + ` ON ` + dal.QuoteIdentifier(tableName) + ` (` + cols + `)`
	default:
		return ""
	}
}

func explicitIndexName(tableName string, item any, i int) string {
	switch value := item.(type) {
	case string:
		return fmt.Sprintf("idx_%s_custom_%d", sanitizeIdentifier(tableName), i+1)
	case map[string]any:
		if name := stringValue(value["name"]); name != "" {
			return name
		}
		return indexName(tableName, stringValue(value["columns"]), boolValue(value["unique"]))
	default:
		return fmt.Sprintf("idx_%s_custom_%d", sanitizeIdentifier(tableName), i+1)
	}
}

func indexName(tableName string, column string, unique bool) string {
	prefix := "idx"
	if unique {
		prefix = "ux"
	}
	return fmt.Sprintf("%s_%s_%s", prefix, sanitizeIdentifier(tableName), sanitizeIdentifier(column))
}

func sanitizeIdentifier(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func (s *Service) syncIndexes(tableName string, oldCollection *Collection, newCollection *Collection) error {
	existing, err := s.dal.DAL().TableIndexes(tableName)
	if err != nil {
		return err
	}

	desired := desiredIndexes(newCollection)
	protected := map[string]struct{}{
		tableName + "_pkey": {},
	}

	for name := range existing {
		if _, keep := protected[name]; keep {
			continue
		}
		if _, keep := desired[name]; keep {
			continue
		}
		if strings.HasPrefix(name, "idx_") || strings.HasPrefix(name, "ux_") {
			if _, err := s.db().ExecContext(context.Background(), `DROP INDEX IF EXISTS `+dal.QuoteIdentifier(name)); err != nil {
				return err
			}
		}
	}

	for name, stmt := range desired {
		if existingSQL, ok := existing[name]; ok && strings.TrimSpace(existingSQL) != "" {
			if _, err := s.db().ExecContext(context.Background(), `DROP INDEX IF EXISTS `+dal.QuoteIdentifier(name)); err != nil {
				return err
			}
		}
		if _, err := s.db().ExecContext(context.Background(), stmt); err != nil {
			return err
		}
	}

	return nil
}

func sqlTypeForField(fieldType string) string {
	switch strings.ToLower(strings.TrimSpace(fieldType)) {
	case "bool", "boolean":
		return "boolean"
	case "number", "int", "integer":
		return "bigint"
	case "float", "double", "decimal":
		return "double precision"
	case "json":
		return "jsonb"
	case "date", "datetime", "timestamp":
		return "timestamptz"
	case "bytes":
		return "bytea"
	default:
		return "text"
	}
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

func validateCollectionFields(fields dal.FieldsList) error {
	names := map[string]struct{}{"id": {}}
	for _, field := range fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			return fmt.Errorf("%w: collection field name is required", ErrInvalidModel)
		}
		if _, exists := names[name]; exists {
			return fmt.Errorf("%w: duplicated field name %q", ErrInvalidModel, name)
		}
		names[name] = struct{}{}
	}
	return nil
}

func validateViewDefinition(name string, query string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: view name is required", ErrInvalidModel)
	}
	return dalValidateViewSelectQuery(query)
}

func dalValidateViewSelectQuery(query string) error {
	return dal.ValidateViewSelectQuery(query)
}

func collectionFromPayload(data map[string]any) (*Collection, error) {
	if data == nil {
		return nil, sql.ErrNoRows
	}

	collection := &Collection{
		ID:     stringValue(data["id"]),
		Name:   stringValue(data["name"]),
		Type:   stringValue(data["type"]),
		System: boolValue(data["system"]),
		Meta:   mapValue(data["meta"]),
	}

	if collection.ID == "" {
		collection.ID = collection.Name
	}

	if rawFields, ok := data["fields"]; ok {
		fields, err := fieldsFromValue(rawFields)
		if err != nil {
			return nil, err
		}
		collection.Fields = fields
	}

	return collection, nil
}

func fieldsFromValue(v any) (dal.FieldsList, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var fields dal.FieldsList
	err = json.Unmarshal(raw, &fields)
	return fields, err
}

func mapValue(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	switch value := v.(type) {
	case map[string]any:
		return value
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return map[string]any{}
		}
		out := map[string]any{}
		_ = json.Unmarshal(raw, &out)
		return out
	}
}

func stringValue(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case nil:
		return ""
	default:
		return fmt.Sprint(value)
	}
}

func boolValue(v any) bool {
	switch value := v.(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(value, "true") || value == "1"
	case float64:
		return value != 0
	default:
		return false
	}
}

func (s *Service) deleteByRecordRef(tableName string, authRecord *Record) error {
	if authRecord == nil {
		return sql.ErrNoRows
	}

	_, err := s.db().ExecContext(
		context.Background(),
		`DELETE FROM `+dal.QuoteIdentifier(tableName)+` WHERE "recordRef" = $1`,
		authRecord.ID,
	)
	return err
}

func (s *Service) deleteExpired(tableName string) error {
	db := s.db()
	if db == nil {
		return sql.ErrConnDone
	}

	columns, err := s.dal.DAL().TableColumns(tableName)
	if err != nil {
		return err
	}

	switch {
	case slices.Contains(columns, "expires"):
		_, err = db.ExecContext(context.Background(), `DELETE FROM `+dal.QuoteIdentifier(tableName)+` WHERE "expires" < NOW()`)
	case slices.Contains(columns, "created"):
		_, err = db.ExecContext(context.Background(), `DELETE FROM `+dal.QuoteIdentifier(tableName)+` WHERE "created" < NOW() - INTERVAL '1 day'`)
	default:
		return nil
	}

	return err
}

func (s *Service) runnerForDB(db *sql.DB) sqlExecutor {
	switch {
	case db == nil:
		return nil
	case s.txMain != nil && db == s.db():
		return s.txMain
	case s.txAux != nil && db == s.auxDB():
		return s.txAux
	default:
		return db
	}
}

func (s *Service) expandRecords(records []*Record, expands []string, optFetchFunc ExpandFetchFunc) map[string]error {
	errs := map[string]error{}
	if len(records) == 0 || len(expands) == 0 {
		return errs
	}

	for _, record := range records {
		if record == nil {
			continue
		}
		if record.Expand == nil {
			record.Expand = map[string]any{}
		}

		for _, expand := range expands {
			expand = strings.TrimSpace(expand)
			if expand == "" {
				continue
			}

			err := s.expandPath(record, strings.Split(expand, "."), optFetchFunc)
			if err != nil {
				if err == sql.ErrNoRows {
					continue
				}
				errs[expand] = err
			}
		}
	}

	return errs
}

func (s *Service) expandPath(record *Record, parts []string, optFetchFunc ExpandFetchFunc) error {
	if record == nil || len(parts) == 0 {
		return nil
	}

	current := strings.TrimSpace(parts[0])
	if current == "" {
		return nil
	}

	items, err := s.fetchExpand(record, current, optFetchFunc)
	if err != nil {
		return err
	}

	if record.Expand == nil {
		record.Expand = map[string]any{}
	}
	if len(items) == 1 {
		record.Expand[current] = items[0]
	} else {
		record.Expand[current] = items
	}

	if len(parts) == 1 {
		return nil
	}

	for _, item := range items {
		if err := s.expandPath(item, parts[1:], optFetchFunc); err != nil && err != sql.ErrNoRows {
			return err
		}
	}

	return nil
}

func (s *Service) fetchExpand(record *Record, expand string, optFetchFunc ExpandFetchFunc) ([]*Record, error) {
	if optFetchFunc != nil {
		items, err := optFetchFunc(record, expand)
		if err == nil && items != nil {
			return items, nil
		}
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
	}

	if record.Collection == nil {
		return nil, sql.ErrNoRows
	}

	field, ok := findCollectionField(record.Collection, expand)
	if !ok {
		return nil, sql.ErrNoRows
	}

	target := relatedCollectionIdentifier(field)
	if target == "" {
		return nil, sql.ErrNoRows
	}

	rawValue, ok := record.Data[expand]
	if !ok {
		rawValue, ok = record.Data[field.Name]
	}
	if !ok {
		return nil, sql.ErrNoRows
	}

	ids := expandIDs(rawValue)
	if len(ids) == 0 {
		return nil, sql.ErrNoRows
	}

	return s.dal.DAL().FindRecordsByIds(target, ids)
}

func findCollectionField(collection *dal.Collection, name string) (dal.Field, bool) {
	if collection == nil {
		return dal.Field{}, false
	}

	for _, field := range collection.Fields {
		if field.Name == name || field.ID == name {
			return field, true
		}
	}

	return dal.Field{}, false
}

func relatedCollectionIdentifier(field dal.Field) string {
	if field.Meta == nil {
		return ""
	}

	candidates := []string{
		stringValue(field.Meta["collection"]),
		stringValue(field.Meta["collectionId"]),
		stringValue(field.Meta["collectionName"]),
		stringValue(field.Meta["collectionRef"]),
	}

	for _, item := range candidates {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}

	return ""
}

func expandIDs(v any) []string {
	switch value := v.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		if strings.Contains(value, ",") {
			parts := strings.Split(value, ",")
			out := make([]string, 0, len(parts))
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					out = append(out, part)
				}
			}
			return out
		}
		return []string{value}
	case []string:
		out := make([]string, 0, len(value))
		for _, item := range value {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if s := strings.TrimSpace(stringValue(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		if s := strings.TrimSpace(stringValue(value)); s != "" {
			return []string{s}
		}
		return nil
	}
}
