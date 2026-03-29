package grpc

import (
	"context"
	"encoding/json"

	"github.com/lumm2509/keel/runtime/hook"
)

type Resolver interface {
	hook.Resolver
	grpcResponse() []byte
}

type MethodInfo struct {
	FullMethod string
	Method     string
	Service    string
}

type Event struct {
	Context context.Context
	Method  MethodInfo

	hook.Event
	hook.EventData

	request  []byte
	response []byte
}

func (e *Event) RequestBytes() []byte {
	return append([]byte(nil), e.request...)
}

func (e *Event) BindJSON(dst any) error {
	return json.Unmarshal(e.request, dst)
}

func (e *Event) SetResponseBytes(data []byte) {
	e.response = append(e.response[:0], data...)
}

func (e *Event) JSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	e.SetResponseBytes(data)
	return nil
}

func (e *Event) grpcResponse() []byte {
	if len(e.response) == 0 {
		return []byte("null")
	}

	return append([]byte(nil), e.response...)
}

