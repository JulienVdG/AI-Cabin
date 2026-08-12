// Package render renders text/template fragments for cabin feature bundles.
//
// Parse reads a .tmpl file from an fs.FS (typically a unionfs union of the
// fragment fallback chain). Execute renders a parsed template with profile
// vars (from config.ResolveVars) and feature attributes (from the cabin
// header's features: block).
//
// At execution, attributes are top-level ({{.port}}, {{range .forwards}}) and
// profile vars are namespaced ({{.Vars.SCW_PROJECT_ID}}). "Vars" is a reserved
// attribute name.
//
// Undefined keys use missingkey=invalid: they render as the literal "<no
// value>" (visible, falsy in {{if}}), so template authors handle optionality
// with {{if .X}} or {{default "y" .X}} (the "default" func is registered). If
// the rendered output contains "<no value>", Execute returns ErrUndefinedVar
// (the output is still written to dst, so the user can open the file and
// locate the offending var — atomicity is traded for debuggability).
package render

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"text/template"
)

var (
	// ErrUndefinedVar is returned when the rendered output contains the
	// "<no value>" marker, indicating a template references an undefined
	// variable without guarding it with {{if .X}} or {{default "y" .X}}.
	// The output is still written to dst for inspection.
	ErrUndefinedVar = errors.New("template references an undefined variable (look for \"<no value>\" in the rendered output, or guard with {{if .X}} / {{default \"y\" .X}})")

	// ErrReservedAttr is returned when an attribute is named "Vars", which
	// would shadow the profile-vars namespace.
	ErrReservedAttr = errors.New("attribute name \"Vars\" is reserved (used to namespace profile vars as {{.Vars.X}} in templates)")
)

// noValueMarker is the string text/template emits for an undefined key under
// missingkey=invalid.
const noValueMarker = "<no value>"

var funcMap = template.FuncMap{
	// default returns def when v is nil or an empty string, like Ansible's
	// default filter. Works on both absent keys (received as nil under
	// missingkey=invalid) and present-but-empty values.
	"default": func(def any, v any) any {
		if v == nil {
			return def
		}
		if s, ok := v.(string); ok && s == "" {
			return def
		}
		return v
	},
}

// newTemplate returns a template configured with the package funcMap and the
// missingkey=invalid option, applying custom delims when set. Shared by Parse
// (template source in an fs.FS) and RenderString (template source already in
// memory, rendered to a string).
func newTemplate(name string, delims Delims) *template.Template {
	t := template.New(name).Funcs(funcMap).Option("missingkey=invalid")
	if delims.Left != "" && delims.Right != "" {
		t = t.Delims(delims.Left, delims.Right)
	}
	return t
}

// Delims selects the action delimiters a template is parsed with. The zero
// value keeps the text/template default ("{{" / "}}"). A skeleton whose
// content legitimately contains "{{" (e.g. a Taskfile runtime var) declares
// custom delims so its scaffold-time substitutions do not collide with the
// embedded template syntax: {<.module>} is resolved at copy time while a
// literal {{.project}} passes through to the runtime consumer (task).
type Delims struct {
	Left  string
	Right string
}

// Parse reads the template at name from src and returns a parsed template
// configured with the "default" func, missingkey=invalid, and the given delims,
// ready to Execute. It reads the file content and parses it as the body of the
// template named name (rather than template.ParseFS, which names the parsed
// body by the file's base name — a mismatch that leaves the returned template
// unexecutable when name contains a directory component).
func Parse(src fs.FS, name string, delims Delims) (*template.Template, error) {
	content, err := fs.ReadFile(src, name)
	if err != nil {
		return nil, fmt.Errorf("read template %q: %w", name, err)
	}
	return newTemplate(name, delims).Parse(string(content))
}

// Execute renders tmpl into dst with the given profile vars and feature attrs.
// Attrs are top-level in the template ({{.port}}); vars are namespaced
// ({{.Vars.SCW_PROJECT_ID}}). The output is streamed to dst as it renders,
// including any "<no value>" markers; if a marker is found, ErrUndefinedVar is
// returned after the write completes so the user can inspect the broken file.
func Execute(tmpl *template.Template, vars map[string]string, attrs map[string]any, dst io.Writer) error {
	if _, ok := attrs["Vars"]; ok {
		return ErrReservedAttr
	}

	data := make(map[string]any, len(attrs)+1)
	maps.Copy(data, attrs)
	data["Vars"] = vars

	scanner := newNoValueScanner(dst)
	if err := tmpl.Execute(scanner, data); err != nil {
		return err
	}
	if scanner.found {
		return ErrUndefinedVar
	}
	return nil
}

// noValueScanner is an io.Writer that forwards writes to dst while scanning
// for the "<no value>" marker. The marker is emitted atomically by
// text/template (a single Write of the constant string), so a per-Write
// bytes.Contains is enough — no cross-Write boundary buffer needed.
type noValueScanner struct {
	dst   io.Writer
	found bool
}

func newNoValueScanner(dst io.Writer) *noValueScanner {
	return &noValueScanner{dst: dst}
}

func (s *noValueScanner) Write(p []byte) (int, error) {
	if !s.found && bytes.Contains(p, []byte(noValueMarker)) {
		s.found = true
	}
	return s.dst.Write(p)
}

// RenderString renders a template text (typically a dst path containing
// {{.host}} or {{.port}} vars for port-forward multi-instance naming) with
// vars, attrs, and the given delims, returning the rendered string. It is the
// string-output counterpart to Execute (which writes to an io.Writer): a
// filename is a value, not a writable destination, so it is built by rendering
// to a buffer. On ErrUndefinedVar the partial string (with "<no value>") is
// returned alongside the error; callers using the result as a path should not
// use it on error.
func RenderString(text string, vars map[string]string, attrs map[string]any, delims Delims) (string, error) {
	tmpl, err := newTemplate("string", delims).Parse(text)
	if err != nil {
		return "", fmt.Errorf("parse template string: %w", err)
	}
	var buf bytes.Buffer
	if err := Execute(tmpl, vars, attrs, &buf); err != nil {
		return buf.String(), err
	}
	return buf.String(), nil
}
