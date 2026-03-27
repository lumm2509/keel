package container

import (
	stdhttp "net/http"

	"context"
	"net"

	"github.com/lumm2509/keel/runtime/hook"
	"github.com/lumm2509/keel/transport/http"
	"golang.org/x/crypto/acme/autocert"
)

type BootstrapEvent[C any] struct {
	hook.Event
	Container Container[C]
}

type TerminateEvent[C any] struct {
	hook.Event
	Container Container[C]
	IsRestart bool
}

type BackupEvent[C any] struct {
	hook.Event
	Container Container[C]
	Context   context.Context
	Name      string   // the name of the backup to create/restore.
	Exclude   []string // list of dir entries to exclude from the backup create/restore.
}

type ServeEvent[C any] struct {
	hook.Event
	Container   Container[C]
	Router      *http.Router[*RequestEvent[C]]
	Server      *stdhttp.Server
	CertManager *autocert.Manager

	// Listener allow specifying a custom network listener.
	//
	// Leave it nil to use the default net.Listen("tcp", e.Server.Addr).
	Listener net.Listener
}
