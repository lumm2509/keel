package http

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/lumm2509/keel/runtime/hook"
)

func BenchmarkRouterBuildMux(b *testing.B) {
	b.Run("current", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r := benchmarkRouterFixture()
			mux, err := r.BuildMux()
			if err != nil || mux == nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r := benchmarkRouterFixture()
			mux, err := legacyBuildMux(r)
			if err != nil || mux == nil {
				b.Fatal(err)
			}
		}
	})
}

func benchmarkRouterFixture() *Router[*Event] {
	r := NewRouter(func(w http.ResponseWriter, req *http.Request) (*Event, EventCleanupFunc) {
		return &Event{Request: req, Response: w}, nil
	})

	for g := 0; g < 20; g++ {
		group := r.Group("/api/" + strings.Repeat("g", g%5+1) + "/" + string(rune('a'+g)))
		for m := 0; m < 4; m++ {
			group.Bind(&hook.Handler[*Event]{
				Id:       "g" + string(rune('a'+g)) + string(rune('0'+m)),
				Priority: m - 2,
				Func: func(e *Event) error {
					return e.Next()
				},
			})
		}

		for s := 0; s < 5; s++ {
			sub := group.Group("/sub/" + string(rune('a'+s)))
			for m := 0; m < 2; m++ {
				sub.Bind(&hook.Handler[*Event]{
					Id:       "s" + string(rune('a'+s)) + string(rune('0'+m)),
					Priority: m,
					Func: func(e *Event) error {
						return e.Next()
					},
				})
			}

			for rt := 0; rt < 5; rt++ {
				route := sub.GET("/resource/"+string(rune('a'+rt)), func(e *Event) error { return nil })
				for m := 0; m < 2; m++ {
					route.Bind(&hook.Handler[*Event]{
						Id:       "r" + string(rune('a'+rt)) + string(rune('0'+m)),
						Priority: m,
						Func: func(e *Event) error {
							return e.Next()
						},
					})
				}
				if rt%2 == 0 {
					route.Unbind("g" + string(rune('a'+g)) + "1")
				}
			}
		}
	}

	return r
}

func legacyBuildMux(r *Router[*Event]) (http.Handler, error) {
	if !r.HasRoute("", "/") {
		r.Route("", "/", func(e *Event) error {
			return NewNotFoundError("", nil)
		})
	}

	mux := http.NewServeMux()
	if err := legacyLoadMux(r, mux, r.RouterGroup, nil); err != nil {
		return nil, err
	}

	return mux, nil
}

func legacyLoadMux(r *Router[*Event], mux *http.ServeMux, group *RouterGroup[*Event], parents []*RouterGroup[*Event]) error {
	for _, child := range group.Children {
		switch v := child.(type) {
		case *RouterGroup[*Event]:
			if err := legacyLoadMux(r, mux, v, append(parents, group)); err != nil {
				return err
			}
		case *Route[*Event]:
			routeHook := &hook.Hook[*Event]{}
			var pattern string

			if v.Method != "" {
				pattern = v.Method + " "
			}

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

			routePattern := pattern
			mux.HandleFunc(routePattern, func(resp http.ResponseWriter, req *http.Request) {})
		default:
			return errors.New("invalid Group item type")
		}
	}

	return nil
}
