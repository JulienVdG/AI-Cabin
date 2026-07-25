package render_test

import (
	"bytes"
	"testing"
	"testing/fstest"
	"text/template"

	"github.com/JulienVdG/AI-Cabin/internal/render"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseString parses a template from a single-file in-memory FS, exercising
// the real render.Parse path (FuncMap + missingkey=invalid config). Keeps test
// cases decoupled from how Parse configures the template internally.
func parseString(t *testing.T, name, content string) *template.Template {
	t.Helper()
	fsys := fstest.MapFS{name: {Data: []byte(content)}}
	tmpl, err := render.Parse(fsys, name)
	require.NoError(t, err)
	return tmpl
}

func TestParse(t *testing.T) {
	t.Run("ValidTemplate", func(t *testing.T) {
		fsys := fstest.MapFS{"t.tmpl": {Data: []byte("hello {{.X}}")}}
		tmpl, err := render.Parse(fsys, "t.tmpl")
		require.NoError(t, err)
		assert.NotNil(t, tmpl)
	})

	t.Run("InvalidSyntax", func(t *testing.T) {
		fsys := fstest.MapFS{"t.tmpl": {Data: []byte("{{.X")}}
		_, err := render.Parse(fsys, "t.tmpl")
		require.Error(t, err)
	})

	t.Run("MissingFile", func(t *testing.T) {
		_, err := render.Parse(fstest.MapFS{}, "missing.tmpl")
		require.Error(t, err)
	})
}

func TestExecute(t *testing.T) {
	vars := map[string]string{"SCW_PROJECT_ID": "proj-123"}

	cases := []struct {
		name    string
		tmpl    string
		vars    map[string]string
		attrs   map[string]any
		wantOut string
		wantErr error
	}{
		{
			name:    "VarsNamespaced",
			tmpl:    "{{.Vars.SCW_PROJECT_ID}}",
			vars:    vars,
			wantOut: "proj-123",
		},
		{
			name:    "VarsAbsentNamespaced",
			tmpl:    "{{.Vars.MISSING}}",
			vars:    vars,
			wantOut: "<no value>",
			wantErr: render.ErrUndefinedVar,
		},
		{
			name:    "AttrsTopLevel",
			tmpl:    "{{.port}}",
			attrs:   map[string]any{"port": "3306"},
			wantOut: "3306",
		},
		{
			// Output is written even when a var is undefined (écriture-malgré-erreur):
			// the broken render reaches dst so the user can locate the marker.
			name:    "AttrAbsentWrittenWithMarker",
			tmpl:    "before {{.ABSENT}} after",
			wantOut: "before <no value> after",
			wantErr: render.ErrUndefinedVar,
		},
		{
			// Optionality: {{if .X}} is falsy on an absent attr, no error.
			name:    "IfAbsentSkipsBlock",
			tmpl:    "{{if .ABSENT}}YES{{else}}NO{{end}}",
			wantOut: "NO",
		},
		{
			// default func handles absent keys (received as nil under
			// missingkey=invalid) — Ansible-like fallback, no error.
			name:    "DefaultOnAbsent",
			tmpl:    `{{default "x" .ABSENT}}`,
			wantOut: "x",
		},
		{
			name:    "DefaultOnValue",
			tmpl:    `{{default "x" .PRESENT}}`,
			attrs:   map[string]any{"PRESENT": "value"},
			wantOut: "value",
		},
		{
			name:    "DefaultOnEmpty",
			tmpl:    `{{default "x" .EMPTY}}`,
			attrs:   map[string]any{"EMPTY": ""},
			wantOut: "x",
		},
		{
			name: "AttrsRange",
			tmpl: "{{range .forwards}}{{.host}}:{{.port}} {{end}}",
			attrs: map[string]any{
				"forwards": []map[string]any{
					{"port": "3306", "host": "mariadb"},
					{"port": "8080", "host": "apache"},
				},
			},
			wantOut: "mariadb:3306 apache:8080 ",
		},
		{
			// "Vars" is reserved: rejected before execution, dst stays empty.
			name:    "ReservedAttrVarsRejected",
			tmpl:    "{{.port}}",
			attrs:   map[string]any{"Vars": "collides"},
			wantOut: "",
			wantErr: render.ErrReservedAttr,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := parseString(t, "t.tmpl", tc.tmpl)
			var buf bytes.Buffer
			err := render.Execute(tmpl, tc.vars, tc.attrs, &buf)

			assert.Equal(t, tc.wantOut, buf.String())
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	t.Run("ExecuteErrorPropagated", func(t *testing.T) {
		// A real template execution error (not a <no value> marker) is returned
		// as-is, distinct from ErrUndefinedVar.
		tmpl := parseString(t, "t.tmpl", "{{index .forwards 99}}")
		attrs := map[string]any{"forwards": []map[string]any{{"host": "x"}}}
		var buf bytes.Buffer
		err := render.Execute(tmpl, nil, attrs, &buf)
		require.Error(t, err)
		assert.NotErrorIs(t, err, render.ErrUndefinedVar)
		assert.NotErrorIs(t, err, render.ErrReservedAttr)
	})
}
