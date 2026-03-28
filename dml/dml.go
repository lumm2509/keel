package dml

import (
	"context"
	"time"

	"github.com/lumm2509/keel/dal"
	"github.com/lumm2509/keel/runtime/hook"
)

type Dml interface {
	DeleteTable(dangerousTableName string) error
	DeleteView(dangerousViewName string) error
	Vacuum() error
	AuxVacuum() error

	Delete(model Model) error
	DeleteWithContext(ctx context.Context, model Model) error
	AuxDelete(model Model) error
	AuxDeleteWithContext(ctx context.Context, model Model) error
	Save(model Model) error
	SaveWithContext(ctx context.Context, model Model) error
	SaveNoValidate(model Model) error
	SaveNoValidateWithContext(ctx context.Context, model Model) error
	AuxSave(model Model) error
	AuxSaveWithContext(ctx context.Context, model Model) error
	AuxSaveNoValidate(model Model) error
	AuxSaveNoValidateWithContext(ctx context.Context, model Model) error
	Validate(model Model) error
	ValidateWithContext(ctx context.Context, model Model) error
	RunInTransaction(fn func(txApp App) error) error
	AuxRunInTransaction(fn func(txApp App) error) error

	DeleteOldLogs(createdBefore time.Time) error

	ReloadCachedCollections() error
	TruncateCollection(collection *Collection) error
	ImportCollections(toImport []map[string]any, deleteMissing bool) error
	ImportCollectionsByMarshaledJSON(rawSliceOfMaps []byte, deleteMissing bool) error
	SyncRecordTableSchema(newCollection *Collection, oldCollection *Collection) error

	DeleteAllMFAsByRecord(authRecord *Record) error
	DeleteExpiredMFAs() error
	DeleteAllOTPsByRecord(authRecord *Record) error
	DeleteExpiredOTPs() error
	DeleteAllAuthOriginsByRecord(authRecord *Record) error

	ExpandRecord(record *Record, expands []string, optFetchFunc ExpandFetchFunc) map[string]error
	ExpandRecords(records []*Record, expands []string, optFetchFunc ExpandFetchFunc) map[string]error

	OnModelValidate(tags ...string) *hook.TaggedHook[*ModelEvent]
	OnModelCreate(tags ...string) *hook.TaggedHook[*ModelEvent]
	OnModelCreateExecute(tags ...string) *hook.TaggedHook[*ModelEvent]
	OnModelAfterCreateSuccess(tags ...string) *hook.TaggedHook[*ModelEvent]
	OnModelAfterCreateError(tags ...string) *hook.TaggedHook[*ModelErrorEvent]
	OnModelUpdate(tags ...string) *hook.TaggedHook[*ModelEvent]
	OnModelUpdateExecute(tags ...string) *hook.TaggedHook[*ModelEvent]
	OnModelAfterUpdateSuccess(tags ...string) *hook.TaggedHook[*ModelEvent]
	OnModelAfterUpdateError(tags ...string) *hook.TaggedHook[*ModelErrorEvent]
	OnModelDelete(tags ...string) *hook.TaggedHook[*ModelEvent]
	OnModelDeleteExecute(tags ...string) *hook.TaggedHook[*ModelEvent]
	OnModelAfterDeleteSuccess(tags ...string) *hook.TaggedHook[*ModelEvent]
	OnModelAfterDeleteError(tags ...string) *hook.TaggedHook[*ModelErrorEvent]

	OnRecordEnrich(tags ...string) *hook.TaggedHook[*RecordEnrichEvent]
	OnRecordValidate(tags ...string) *hook.TaggedHook[*RecordEvent]
	OnRecordCreate(tags ...string) *hook.TaggedHook[*RecordEvent]
	OnRecordCreateExecute(tags ...string) *hook.TaggedHook[*RecordEvent]
	OnRecordAfterCreateSuccess(tags ...string) *hook.TaggedHook[*RecordEvent]
	OnRecordAfterCreateError(tags ...string) *hook.TaggedHook[*RecordErrorEvent]
	OnRecordUpdate(tags ...string) *hook.TaggedHook[*RecordEvent]
	OnRecordUpdateExecute(tags ...string) *hook.TaggedHook[*RecordEvent]
	OnRecordAfterUpdateSuccess(tags ...string) *hook.TaggedHook[*RecordEvent]
	OnRecordAfterUpdateError(tags ...string) *hook.TaggedHook[*RecordErrorEvent]
	OnRecordDelete(tags ...string) *hook.TaggedHook[*RecordEvent]
	OnRecordDeleteExecute(tags ...string) *hook.TaggedHook[*RecordEvent]
	OnRecordAfterDeleteSuccess(tags ...string) *hook.TaggedHook[*RecordEvent]
	OnRecordAfterDeleteError(tags ...string) *hook.TaggedHook[*RecordErrorEvent]

	OnCollectionValidate(tags ...string) *hook.TaggedHook[*CollectionEvent]
	OnCollectionCreate(tags ...string) *hook.TaggedHook[*CollectionEvent]
	OnCollectionCreateExecute(tags ...string) *hook.TaggedHook[*CollectionEvent]
	OnCollectionAfterCreateSuccess(tags ...string) *hook.TaggedHook[*CollectionEvent]
	OnCollectionAfterCreateError(tags ...string) *hook.TaggedHook[*CollectionErrorEvent]
	OnCollectionUpdate(tags ...string) *hook.TaggedHook[*CollectionEvent]
	OnCollectionUpdateExecute(tags ...string) *hook.TaggedHook[*CollectionEvent]
	OnCollectionAfterUpdateSuccess(tags ...string) *hook.TaggedHook[*CollectionEvent]
	OnCollectionAfterUpdateError(tags ...string) *hook.TaggedHook[*CollectionErrorEvent]
	OnCollectionDelete(tags ...string) *hook.TaggedHook[*CollectionEvent]
	OnCollectionDeleteExecute(tags ...string) *hook.TaggedHook[*CollectionEvent]
	OnCollectionAfterDeleteSuccess(tags ...string) *hook.TaggedHook[*CollectionEvent]
	OnCollectionAfterDeleteError(tags ...string) *hook.TaggedHook[*CollectionErrorEvent]
}

type Model = dal.Model
type Record = dal.Record
type Collection = dal.Collection
