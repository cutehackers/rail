package scaffold

import (
	"bytes"
	"embed"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// FS exposes the canonical embedded project scaffold.
//
//go:embed project.yaml
var FS embed.FS

var projectYAMLTemplate = template.Must(template.New("project.yaml").Funcs(template.FuncMap{
	"yamlScalar": yamlScalar,
}).ParseFS(FS, "project.yaml"))

func yamlScalar(value string) (string, error) {
	data, err := yaml.Marshal(value)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

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
