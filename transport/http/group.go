package http

import "github.com/lumm2509/keel/runtime/hook"

// RouterGroup is a type alias for hook.RouterGroup.
// All routing methods (GET, POST, Group, Bind, etc.) are promoted from hook.RouterGroup.
type RouterGroup[T hook.Resolver] = hook.RouterGroup[T]
