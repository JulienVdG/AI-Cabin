package main

import "testing"

// TestParseFeatureRefs covers the --features flag: each entry is either a bare
// feature name (`go`) or a feature with inline attrs
// (`port-forward:{port: 5432, host: postgres}`), matched by the header
// FeatureRef YAML forms. Commas inside the braces separate attrs, not entries.
func TestParseFeatureRefs(t *testing.T) {
	t.Run("BareAndAttrsForms", func(t *testing.T) {
		refs, err := parseFeatureRefs("go,port-forward: {port: 5432, host: postgres},git-agent")
		if err != nil {
			t.Fatalf("parseFeatureRefs: %v", err)
		}
		if len(refs) != 3 {
			t.Fatalf("got %d refs, want 3: %+v", len(refs), refs)
		}
		if refs[0].Name != "go" || refs[0].Attrs != nil {
			t.Errorf("ref[0] = %+v, want bare go", refs[0])
		}
		if refs[1].Name != "port-forward" {
			t.Errorf("ref[1].Name = %q, want port-forward", refs[1].Name)
		}
		if refs[1].Attrs == nil || refs[1].Attrs["host"] != "postgres" || refs[1].Attrs["port"] != 5432 {
			t.Errorf("ref[1].Attrs = %+v, want {host: postgres, port: 5432}", refs[1].Attrs)
		}
		if refs[2].Name != "git-agent" || refs[2].Attrs != nil {
			t.Errorf("ref[2] = %+v, want bare git-agent", refs[2])
		}
	})

	t.Run("Empty", func(t *testing.T) {
		refs, err := parseFeatureRefs("")
		if err != nil {
			t.Fatalf("parseFeatureRefs: %v", err)
		}
		if refs != nil {
			t.Errorf("got %+v, want nil for empty input", refs)
		}
	})

	t.Run("WhitespaceTolerated", func(t *testing.T) {
		refs, err := parseFeatureRefs("  go ,  git-agent ")
		if err != nil {
			t.Fatalf("parseFeatureRefs: %v", err)
		}
		if len(refs) != 2 || refs[0].Name != "go" || refs[1].Name != "git-agent" {
			t.Errorf("got %+v, want [go git-agent]", refs)
		}
	})

	t.Run("MalformedRejected", func(t *testing.T) {
		if _, err := parseFeatureRefs("port-forward:{broken"); err == nil {
			t.Error("expected an error for an unterminated attrs map")
		}
	})
}
