package config

import "errors"

// ErrCabinNotFound is returned by GetCabin when the requested cabin name is
// not in the registry. This is a normal condition (a fresh install has no
// cabins), not a failure — callers typically branch on errors.Is to decide
// whether to add a new entry.
var ErrCabinNotFound = errors.New("cabin not found in registry")
