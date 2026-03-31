package http

import "os"

// debugMode is evaluated once at startup. Set KEEL_DEBUG=1 to enable
// development warnings (forgotten e.Next(), unwritten responses, unknown params).
// Using a package-level var avoids calling os.Getenv on every request.
var debugMode = os.Getenv("KEEL_DEBUG") != ""

func isDebugMode() bool {
	return debugMode
}
