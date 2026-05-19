package cloud

import "os"

// lookupEnv is a tiny indirection so tests can swap env discovery
// without poking at the real process env. Production callers use the
// default implementation that goes straight to os.LookupEnv.
var lookupEnv = func(key string) (string, bool) {
	return os.LookupEnv(key)
}
