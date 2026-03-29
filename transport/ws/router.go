package ws

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lumm2509/keel/runtime/hook"
	"golang.org/x/net/websocket"
)

type EventCleanupFunc func()

type EventFactoryFunc[T hook.Resolver] func(conn *websocket.Conn, req *http.Request) (T, EventCleanupFunc)

type ErrorHandlerFunc func(conn *websocket.Conn, req *http.Request, err error)

type Router[T hook.Resolver] struct {
	*RouterGroup[T]

	errorHandler ErrorHandlerFunc
	eventFactory EventFactoryFunc[T]
}

func NewRouter[T hook.Resolver](eventFactory EventFactoryFunc[T]) *Router[T] {
	return &Router[T]{
		RouterGroup:  &RouterGroup[T]{},
		eventFactory: eventFactory,
		errorHandler: DefaultErrorHandler,
	}
}

func (r *Router[T]) SetErrorHandler(handler ErrorHandlerFunc) *Router[T] {
	if handler != nil {
		r.errorHandler = handler
	}

	return r
}

func (r *Router[T]) BuildHandler() (http.Handler, error) {
	mux := http.NewServeMux()

	if err := r.loadMux(mux, r.RouterGroup, nil); err != nil {
		return nil, err
	}

	return mux, nil
}

func (r *Router[T]) loadMux(mux *http.ServeMux, group *RouterGroup[T], parents []*RouterGroup[T]) error {
	for _, child := range group.Children {
		switch v := child.(type) {
		case *RouterGroup[T]:
			if err := r.loadMux(mux, v, append(parents, group)); err != nil {
				return err
			}
		case *Route[T]:
			routeHook := &hook.Hook[T]{}

			var pattern string

			for _, p := range parents {
				pattern += p.Prefix
				for _, h := range p.Middlewares {
					if _, ok := p.ExcludedMiddlewares[h.Id]; !ok {
						if _, ok = group.ExcludedMiddlewares[h.Id]; !ok {
							if _, ok = v.ExcludedMiddlewares[h.Id]; !ok {
								routeHook.Bind(h)
							}
						}
					}
				}
			}

			pattern += group.Prefix
			for _, h := range group.Middlewares {
				if _, ok := group.ExcludedMiddlewares[h.Id]; !ok {
					if _, ok = v.ExcludedMiddlewares[h.Id]; !ok {
						routeHook.Bind(h)
					}
				}
			}

			pattern += v.Path
			for _, h := range v.Middlewares {
				if _, ok := v.ExcludedMiddlewares[h.Id]; !ok {
					routeHook.Bind(h)
				}
			}

			mux.Handle(pattern, websocket.Handler(func(conn *websocket.Conn) {
				req := conn.Request()
				event, cleanupFunc := r.eventFactory(conn, req)

				err := routeHook.TriggerWithOneOff(event, v.Action)
				if err != nil {
					r.errorHandler(conn, req, err)
				}

				if cleanupFunc != nil {
					cleanupFunc()
				}
			}))
		default:
			return errors.New("invalid Group item type")
		}
	}

	return nil
}

func DefaultErrorHandler(conn *websocket.Conn, _ *http.Request, err error) {
	if err == nil {
		return
	}

	_ = websocket.JSON.Send(conn, map[string]any{
		"message": err.Error(),
	})

	_ = conn.Close()
}

func MarshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}
