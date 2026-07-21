package cabin

import (
	"os"
	osuser "os/user"
	"path/filepath"
	"strings"
)

// homedir returns the current user's home directory. $HOME takes precedence
// (matches shell behavior: bash/zsh expand "~" via $HOME first, /etc/passwd only
// as fallback), so an explicit HOME override is respected. This also makes
// the function testable via t.Setenv("HOME", ...). Falls back to os/user
// lookup when $HOME is unset (Windows, minimal containers).
func homedir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if user, err := osuser.Current(); err == nil {
		return user.HomeDir
	}
	return ""
}

// userhomedir returns the home directory of the named user, or "" if the user
// cannot be looked up. Used for "~user" expansion.
func userhomedir(user string) string {
	u, err := osuser.Lookup(user)
	if err != nil {
		return ""
	}
	return u.HomeDir
}

// expandHome expands a leading "~" or "~user" into the corresponding home
// directory, mirroring OpenSSH's path expansion. Paths not starting with "~"
// are returned unchanged. If "~user" cannot be resolved, the input is returned
// as-is (consistent with shells, which leave an unresolved user untouched).
//
// homedir and userhomedir are injected so the function is unit-testable
// without touching os/user.
func expandHome(inputPath string, homedir func() string, userhomedir func(string) string) string {
	if len(inputPath) == 0 {
		return inputPath
	}
	if inputPath[0] != '~' {
		return inputPath
	}
	path := filepath.ToSlash(inputPath[1:])
	var user string
	slashIdx := strings.IndexByte(path, '/')
	if slashIdx == -1 {
		user = path
		path = ""
	} else {
		user = path[:slashIdx]
		path = path[slashIdx+1:]
	}

	var home string
	if user == "" {
		home = homedir()
	} else {
		home = userhomedir(user)
		if home == "" {
			return inputPath
		}
	}

	return filepath.Join(home, filepath.FromSlash(path))
}

// ExpandHome expands "~" and "~user" in the input path using the current
// user's home directory (and other users' homes via os/user lookup).
func ExpandHome(inputPath string) string {
	return expandHome(inputPath, homedir, userhomedir)
}
