package container

import (
	"context"

	"github.com/lumm2509/keel/runtime/hook"
)

type BackupEvent[C any] struct {
	hook.Event
	Container Container[C]
	Context   context.Context
	Name      string   // the name of the backup to create/restore.
	Exclude   []string // list of dir entries to exclude from the backup create/restore.
}
