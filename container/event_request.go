package container

import transporthttp "github.com/lumm2509/keel/transport/http"

const (
	RequestEventKeyInfoContext = transporthttp.RequestEventKeyInfoContext
)

const (
	RequestInfoContextDefault       = transporthttp.RequestInfoContextDefault
	RequestInfoContextExpand        = transporthttp.RequestInfoContextExpand
	RequestInfoContextRealtime      = transporthttp.RequestInfoContextRealtime
	RequestInfoContextProtectedFile = transporthttp.RequestInfoContextProtectedFile
	RequestInfoContextBatch         = transporthttp.RequestInfoContextBatch
	RequestInfoContextOAuth2        = transporthttp.RequestInfoContextOAuth2
	RequestInfoContextOTP           = transporthttp.RequestInfoContextOTP
	RequestInfoContextPasswordAuth  = transporthttp.RequestInfoContextPasswordAuth
)

type RequestEvent[Cradle any] = transporthttp.RequestEvent[Cradle]
type RequestInfo = transporthttp.RequestInfo
