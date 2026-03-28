package dml

import (
	"github.com/lumm2509/keel/dal"
	"github.com/lumm2509/keel/runtime/hook"
)

type App interface {
	DAL() dal.Dal
	DML() Dml
}

type ExpandFetchFunc func(record *dal.Record, expandPath string) ([]*dal.Record, error)

type modelEventBase struct {
	hook.Event

	App      App
	TagsList []string
}

func (e *modelEventBase) Tags() []string {
	return append([]string(nil), e.TagsList...)
}

type ModelEvent struct {
	modelEventBase
	Model dal.Model
}

type ModelErrorEvent struct {
	ModelEvent
	Err error
}

type RecordEnrichEvent struct {
	modelEventBase
	Record  *dal.Record
	Records []*dal.Record
}

type RecordEvent struct {
	modelEventBase
	Record *dal.Record
}

type RecordErrorEvent struct {
	RecordEvent
	Err error
}

type CollectionEvent struct {
	modelEventBase
	Collection *dal.Collection
}

type CollectionErrorEvent struct {
	CollectionEvent
	Err error
}
