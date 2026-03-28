package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/lumm2509/keel/runtime/hook"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

type EventCleanupFunc func()

type EventFactoryFunc[T Resolver] func(ctx context.Context, req []byte, info MethodInfo) (T, EventCleanupFunc)

type serviceHandler interface{}

type serviceImpl struct{}

type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (jsonCodec) Name() string {
	return "json"
}

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

type Router[T Resolver] struct {
	*RouterGroup[T]

	eventFactory EventFactoryFunc[T]
}

func NewRouter[T Resolver](eventFactory EventFactoryFunc[T]) *Router[T] {
	return &Router[T]{
		RouterGroup:  &RouterGroup[T]{},
		eventFactory: eventFactory,
	}
}

func (r *Router[T]) BuildServer(opts ...ggrpc.ServerOption) (*ggrpc.Server, error) {
	server := ggrpc.NewServer(append([]ggrpc.ServerOption{ggrpc.ForceServerCodec(jsonCodec{})}, opts...)...)

	serviceRoutes := map[string][]ggrpc.MethodDesc{}

	if err := r.collectRoutes(serviceRoutes, r.RouterGroup, nil); err != nil {
		return nil, err
	}

	for serviceName, methods := range serviceRoutes {
		desc := &ggrpc.ServiceDesc{
			ServiceName: serviceName,
			HandlerType: (*serviceHandler)(nil),
			Methods:     methods,
			Streams:     nil,
			Metadata:    "manual",
		}

		server.RegisterService(desc, serviceImpl{})
	}

	return server, nil
}

func (r *Router[T]) collectRoutes(serviceRoutes map[string][]ggrpc.MethodDesc, group *RouterGroup[T], parents []*RouterGroup[T]) error {
	for _, child := range group.children {
		switch v := child.(type) {
		case *RouterGroup[T]:
			if err := r.collectRoutes(serviceRoutes, v, append(parents, group)); err != nil {
				return err
			}
		case *Route[T]:
			fullMethod := joinFullMethod(parents, group, v.Method)
			service, method, err := splitFullMethod(fullMethod)
			if err != nil {
				return err
			}

			routeHook := &hook.Hook[T]{}

			for _, p := range parents {
				for _, h := range p.Middlewares {
					if _, ok := p.excludedMiddlewares[h.Id]; !ok {
						if _, ok = group.excludedMiddlewares[h.Id]; !ok {
							if _, ok = v.excludedMiddlewares[h.Id]; !ok {
								routeHook.Bind(h)
							}
						}
					}
				}
			}

			for _, h := range group.Middlewares {
				if _, ok := group.excludedMiddlewares[h.Id]; !ok {
					if _, ok = v.excludedMiddlewares[h.Id]; !ok {
						routeHook.Bind(h)
					}
				}
			}

			for _, h := range v.Middlewares {
				if _, ok := v.excludedMiddlewares[h.Id]; !ok {
					routeHook.Bind(h)
				}
			}

			serviceRoutes[service] = append(serviceRoutes[service], ggrpc.MethodDesc{
				MethodName: method,
				Handler:    r.unaryHandler(routeHook, v.Action, fullMethod),
			})
		default:
			return errors.New("invalid Group item type")
		}
	}

	return nil
}

func (r *Router[T]) unaryHandler(routeHook *hook.Hook[T], action func(e T) error, fullMethod string) func(any, context.Context, func(any) error, ggrpc.UnaryServerInterceptor) (any, error) {
	service, method, _ := splitFullMethod(fullMethod)
	info := MethodInfo{
		FullMethod: fullMethod,
		Method:     method,
		Service:    service,
	}

	return func(_ any, ctx context.Context, dec func(any) error, interceptor ggrpc.UnaryServerInterceptor) (any, error) {
		req := json.RawMessage{}
		if err := dec(&req); err != nil {
			return nil, ToStatusError(err)
		}

		handler := func(ctx context.Context, rawReq any) (any, error) {
			reqBytes := normalizeRequestBytes(rawReq)

			event, cleanupFunc := r.eventFactory(ctx, reqBytes, info)
			if cleanupFunc != nil {
				defer cleanupFunc()
			}

			if err := routeHook.TriggerWithOneOff(event, action); err != nil {
				return nil, ToStatusError(err)
			}

			return json.RawMessage(event.grpcResponse()), nil
		}

		if interceptor == nil {
			return handler(ctx, req)
		}

		return interceptor(ctx, req, &ggrpc.UnaryServerInfo{FullMethod: fullMethod}, handler)
	}
}

func ToStatusError(err error) error {
	if err == nil {
		return nil
	}

	if _, ok := status.FromError(err); ok {
		return err
	}

	return status.Error(codes.Internal, err.Error())
}

func normalizeRequestBytes(rawReq any) []byte {
	switch v := rawReq.(type) {
	case nil:
		return []byte("null")
	case json.RawMessage:
		return append([]byte(nil), v...)
	case *json.RawMessage:
		if v == nil {
			return []byte("null")
		}
		return append([]byte(nil), (*v)...)
	case []byte:
		return append([]byte(nil), v...)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return []byte("null")
		}
		return data
	}
}

func cleanFullMethod(fullMethod string) string {
	trimmed := strings.TrimSpace(fullMethod)
	if trimmed == "" {
		return ""
	}

	parts := strings.Split(trimmed, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			clean = append(clean, part)
		}
	}

	if len(clean) == 0 {
		return ""
	}

	return "/" + strings.Join(clean, "/")
}

func joinFullMethod[T Resolver](parents []*RouterGroup[T], group *RouterGroup[T], method string) string {
	parts := make([]string, 0, len(parents)+2)

	for _, p := range parents {
		if part := strings.Trim(p.Prefix, "/"); part != "" {
			parts = append(parts, part)
		}
	}

	if part := strings.Trim(group.Prefix, "/"); part != "" {
		parts = append(parts, part)
	}

	if part := strings.Trim(method, "/"); part != "" {
		parts = append(parts, part)
	}

	return "/" + strings.Join(parts, "/")
}

func splitFullMethod(fullMethod string) (string, string, error) {
	cleaned := cleanFullMethod(fullMethod)
	parts := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", status.Errorf(codes.InvalidArgument, "invalid grpc method %q", fullMethod)
	}

	return parts[0], parts[1], nil
}
