package cabin

import "errors"

// ErrNoHeader is returned by ValidateCabin when the Taskfile at the given path
// has no top-level "ai-cabin:" block (i.e. the directory is not a cabin).
// Callers that want to surface a helpful message should errors.Is this and
// print guidance (the path is preserved in the returned resolvedPath for
// context).
var ErrNoHeader = errors.New("Taskfile has no ai-cabin: block (not a cabin)")
