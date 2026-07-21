package cabin

import "testing"

// Test_expandHome is table-driven and asserts both the expanded output AND
// which injected closure was called. expandHome is private, hence this whitebox
// file; ExpandHome (public) is a thin wrapper not worth re-testing (delegation
// only, per skill:go-test-patterns).
func Test_expandHome(t *testing.T) {
	testData := []struct {
		In   string
		Home string
		User string
		Out  string
	}{
		// No tilde: returned unchanged, no closure called.
		{In: "toto/titi", Out: "toto/titi"},
		// "~user/..." : user lookup, home returned for that user.
		{In: "~u/toto/titi", Home: "/home", User: "u", Out: "/home/toto/titi"},
		// "~/..." : current user's home.
		{In: "~/toto/titi", Home: "/home", Out: "/home/toto/titi"},
		// Bare "~" and "~/": expand to home, trailing slash cleaned by filepath.Join.
		{In: "~", Home: "/home", Out: "/home"},
		{In: "~/", Home: "/home", Out: "/home"},
		// "~user" and "~user/" alone: expand to that user's home.
		{In: "~u", Home: "/home", User: "u", Out: "/home"},
		{In: "~u/", Home: "/home", User: "u", Out: "/home"},
		// "~user" that cannot be looked up: input returned unchanged.
		{In: "~u/toto", User: "u", Out: "~u/toto"},
		// Empty input: returned unchanged.
		{In: "", Out: ""},
	}

	for _, tc := range testData {
		t.Run(tc.In, func(t *testing.T) {
			var homedirCalled, userHomedirCalled bool
			out := expandHome(tc.In,
				func() string {
					homedirCalled = true
					return tc.Home
				},
				func(user string) string {
					userHomedirCalled = true
					if user != tc.User {
						t.Errorf("user lookup got %q want %q", user, tc.User)
					}
					return tc.Home
				},
			)
			// homedir() must be called only for current-user "~" cases (no named user).
			if homedirCalled && tc.User != "" {
				t.Error("homedir called while it should not (~user case)")
			}
			// userhomedir() must be called only for named-user cases.
			if userHomedirCalled && tc.User == "" {
				t.Error("userhomedir called while it should not (no ~user case)")
			}
			if out != tc.Out {
				t.Errorf("expand got %q want %q", out, tc.Out)
			}
		})
	}
}
