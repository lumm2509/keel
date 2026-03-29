package container

// DataDirProvider is implemented by containers that expose a data directory path.
// Used by the HTTP serve layer to locate TLS certificate cache directories.
type DataDirProvider interface{ DataDir() string }
