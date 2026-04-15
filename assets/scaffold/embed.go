package scaffold

import (
	"bytes"
	"embed"
	"text/template"
)

// FS exposes the canonical embedded project scaffold.
//
//go:embed project.yaml
var FS embed.FS

var projectYAMLTemplate = template.Must(template.New("project.yaml").ParseFS(FS, "project.yaml"))

func RenderProjectYAML(projectName string) ([]byte, error) {
	var buf bytes.Buffer
	if err := projectYAMLTemplate.Execute(&buf, struct {
		ProjectName string
	}{
		ProjectName: projectName,
	}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
