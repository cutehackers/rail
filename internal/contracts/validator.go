package contracts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rail/internal/assets"
	"rail/internal/request"

	"gopkg.in/yaml.v3"
)

type Validator struct {
	projectRoot string
}

func NewValidator(projectRoot string) (*Validator, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return nil, errors.New("project root is required")
	}

	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	return &Validator{projectRoot: root}, nil
}

func (v *Validator) ValidateRequestFile(requestPath string) (request.CanonicalRequest, error) {
	requestFile, err := ResolvePathWithinRoot(v.projectRoot, requestPath)
	if err != nil {
		return request.CanonicalRequest{}, err
	}
	if _, err := os.Stat(requestFile); err != nil {
		if os.IsNotExist(err) {
			return request.CanonicalRequest{}, fmt.Errorf("request file not found: %s", requestPath)
		}
		return request.CanonicalRequest{}, fmt.Errorf("stat request file: %w", err)
	}

	value, err := readYAMLFile(requestFile)
	if err != nil {
		return request.CanonicalRequest{}, err
	}
	schema, err := v.loadSchema("request")
	if err != nil {
		return request.CanonicalRequest{}, err
	}
	if err := schema.Validate(value, requestPath); err != nil {
		return request.CanonicalRequest{}, err
	}

	canonical, err := parseCanonicalRequest(value)
	if err != nil {
		return request.CanonicalRequest{}, err
	}
	return canonical, nil
}

func (v *Validator) ValidateArtifactFile(filePath, schemaName string) (map[string]any, error) {
	artifactFile, err := ResolvePathWithinRoot(v.projectRoot, filePath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(artifactFile); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("artifact file not found: %s", filePath)
		}
		return nil, fmt.Errorf("stat artifact file: %w", err)
	}

	value, err := readYAMLFile(artifactFile)
	if err != nil {
		return nil, err
	}
	schema, err := v.loadSchema(schemaName)
	if err != nil {
		return nil, err
	}
	if err := schema.Validate(value, filePath); err != nil {
		return nil, err
	}
	return asMap(value, filePath)
}

func (v *Validator) loadSchema(schemaName string) (*SchemaValidator, error) {
	schemaPath := schemaPathForName(schemaName)
	data, _, err := assets.Resolve(v.projectRoot, schemaPath)
	if err != nil {
		return nil, fmt.Errorf("load schema %q: %w", schemaName, err)
	}

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode schema %q: %w", schemaName, err)
	}

	schema, err := asMap(normalizeYAMLValue(raw), schemaPath)
	if err != nil {
		return nil, err
	}
	return &SchemaValidator{schema: schema, schemaName: schemaName}, nil
}

func parseCanonicalRequest(value any) (request.CanonicalRequest, error) {
	requestMap, err := asMap(value, "request")
	if err != nil {
		return request.CanonicalRequest{}, err
	}

	raw, err := yaml.Marshal(requestMap)
	if err != nil {
		return request.CanonicalRequest{}, fmt.Errorf("marshal canonical request: %w", err)
	}

	var canonical request.CanonicalRequest
	if err := yaml.Unmarshal(raw, &canonical); err != nil {
		return request.CanonicalRequest{}, fmt.Errorf("decode canonical request: %w", err)
	}

	if strings.TrimSpace(canonical.TaskType) == "" {
		return request.CanonicalRequest{}, errors.New("task_type is required")
	}
	if strings.TrimSpace(canonical.Goal) == "" {
		return request.CanonicalRequest{}, errors.New("goal is required")
	}
	if strings.TrimSpace(canonical.RiskTolerance) == "" {
		return request.CanonicalRequest{}, errors.New("risk_tolerance is required")
	}
	return canonical, nil
}

func readYAMLFile(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read yaml file: %w", err)
	}

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode yaml file: %w", err)
	}
	return normalizeYAMLValue(raw), nil
}

func ResolvePathWithinRoot(projectRoot, path string) (string, error) {
	if strings.TrimSpace(projectRoot) == "" {
		return "", errors.New("project root is required")
	}
	if strings.TrimSpace(path) == "" {
		return "", errors.New("file path is required")
	}

	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	rootCanonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		rootCanonical = root
	}

	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	candidate = filepath.Clean(candidate)

	canonicalTarget, err := canonicalWithinRoot(rootCanonical, candidate)
	if err != nil {
		return "", err
	}
	if !isWithinRoot(rootCanonical, canonicalTarget) {
		return "", fmt.Errorf("path escapes project root %s: %s", projectRoot, path)
	}
	return canonicalTarget, nil
}

func canonicalWithinRoot(root, candidate string) (string, error) {
	current := candidate
	for {
		_, err := os.Stat(current)
		switch {
		case err == nil:
			canonical, resolveErr := filepath.EvalSymlinks(current)
			if resolveErr != nil {
				return "", fmt.Errorf("resolve path %s: %w", candidate, resolveErr)
			}
			if current == candidate {
				return canonical, nil
			}
			suffix, relErr := filepath.Rel(current, candidate)
			if relErr != nil {
				return "", fmt.Errorf("resolve path %s: %w", candidate, relErr)
			}
			return filepath.Join(canonical, suffix), nil
		case os.IsNotExist(err):
			parent := filepath.Dir(current)
			if parent == current {
				return "", fmt.Errorf("path escapes project root %s: %s", root, candidate)
			}
			current = parent
		default:
			return "", fmt.Errorf("stat path %s: %w", candidate, err)
		}
	}
}

func isWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func schemaPathForName(schemaName string) string {
	switch schemaName {
	case "request":
		return ".harness/templates/user_request.schema.yaml"
	case "evaluation_result":
		return ".harness/templates/evaluation_result.schema.yaml"
	case "execution_report":
		return ".harness/templates/execution_report.schema.yaml"
	default:
		panic(fmt.Sprintf("unsupported schema: %s", schemaName))
	}
}

func normalizeYAMLValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalized[key] = normalizeYAMLValue(nested)
		}
		return normalized
	case map[any]any:
		normalized := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalized[fmt.Sprint(key)] = normalizeYAMLValue(nested)
		}
		return normalized
	case []any:
		normalized := make([]any, len(typed))
		for i, nested := range typed {
			normalized[i] = normalizeYAMLValue(nested)
		}
		return normalized
	default:
		return value
	}
}

func asMap(value any, context string) (map[string]any, error) {
	mapValue, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected object for %s", context)
	}
	return mapValue, nil
}

func ReadYAMLFile(path string) (any, error) {
	return readYAMLFile(path)
}

func AsMap(value any, context string) (map[string]any, error) {
	return asMap(value, context)
}

type SchemaValidator struct {
	schema     map[string]any
	schemaName string
}

func (v *SchemaValidator) Validate(value any, fileLabel string) error {
	errors := v.validateNode(v.schema, value, "$")
	if len(errors) == 0 {
		return nil
	}
	return fmt.Errorf(
		"schema validation failed for %s against %s:\n- %s",
		fileLabel,
		v.schemaName,
		strings.Join(errors, "\n- "),
	)
}

func (v *SchemaValidator) validateNode(schema map[string]any, value any, path string) []string {
	var validationErrors []string

	if oneOf, ok := schema["oneOf"].([]any); ok {
		matched := false
		for _, branch := range oneOf {
			branchMap, ok := branch.(map[string]any)
			if !ok {
				continue
			}
			if len(v.validateNode(branchMap, value, path)) == 0 {
				matched = true
				break
			}
		}
		if !matched {
			validationErrors = append(validationErrors, fmt.Sprintf("%s did not match any allowed schema branch", path))
		}
		return validationErrors
	}

	if allOf, ok := schema["allOf"].([]any); ok {
		for _, branch := range allOf {
			branchMap, ok := branch.(map[string]any)
			if !ok {
				continue
			}
			validationErrors = append(validationErrors, v.validateNode(branchMap, value, path)...)
		}
	}

	if conditional, ok := schema["if"].(map[string]any); ok {
		matches := len(v.validateNode(conditional, value, path)) == 0
		if matches {
			if thenSchema, ok := schema["then"].(map[string]any); ok {
				validationErrors = append(validationErrors, v.validateNode(thenSchema, value, path)...)
			}
		} else if elseSchema, ok := schema["else"].(map[string]any); ok {
			validationErrors = append(validationErrors, v.validateNode(elseSchema, value, path)...)
		}
	}

	if constValue, ok := schema["const"]; ok && !schemaLiteralEquals(value, constValue) {
		validationErrors = append(validationErrors, fmt.Sprintf("%s expected constant value %v", path, constValue))
	}

	expectedType, hasType := schema["type"]
	treatsAsObject := expectedType == "object" || schema["properties"] != nil || schema["required"] != nil || schema["additionalProperties"] != nil
	treatsAsArray := expectedType == "array" || schema["items"] != nil || schema["minItems"] != nil

	if hasType && !matchesSchemaType(expectedType, value) {
		expectedLabel := fmt.Sprint(expectedType)
		if values, ok := expectedType.([]any); ok {
			parts := make([]string, 0, len(values))
			for _, item := range values {
				parts = append(parts, fmt.Sprint(item))
			}
			expectedLabel = strings.Join(parts, "|")
		}
		return append(validationErrors, fmt.Sprintf("%s expected %s", path, expectedLabel))
	}

	if treatsAsObject {
		objectValue, ok := value.(map[string]any)
		if !ok {
			return append(validationErrors, fmt.Sprintf("%s expected object", path))
		}

		if required, ok := schema["required"].([]any); ok {
			for _, field := range required {
				fieldName, ok := field.(string)
				if ok && objectValue[fieldName] == nil {
					validationErrors = append(validationErrors, fmt.Sprintf("%s missing required field `%s`", path, fieldName))
				}
			}
		}

		declared := map[string]struct{}{}
		if properties, ok := schema["properties"].(map[string]any); ok {
			for key, propertySchema := range properties {
				declared[key] = struct{}{}
				childValue, exists := objectValue[key]
				if !exists {
					continue
				}
				childSchema, ok := propertySchema.(map[string]any)
				if !ok {
					continue
				}
				validationErrors = append(validationErrors, v.validateNode(childSchema, childValue, path+"."+key)...)
			}
		}

		switch additional := schema["additionalProperties"].(type) {
		case bool:
			if !additional {
				for key := range objectValue {
					if _, ok := declared[key]; !ok {
						validationErrors = append(validationErrors, fmt.Sprintf("%s contains unsupported field `%s`", path, key))
					}
				}
			}
		case map[string]any:
			for key, childValue := range objectValue {
				if _, ok := declared[key]; ok {
					continue
				}
				validationErrors = append(validationErrors, v.validateNode(additional, childValue, path+"."+key)...)
			}
		}
	} else if treatsAsArray {
		listValue, ok := value.([]any)
		if !ok {
			return append(validationErrors, fmt.Sprintf("%s expected array", path))
		}
		if minItems, ok := schema["minItems"].(int); ok && len(listValue) < minItems {
			validationErrors = append(validationErrors, fmt.Sprintf("%s expected at least %d item(s)", path, minItems))
		}
		if itemSchema, ok := schema["items"].(map[string]any); ok {
			for i, entry := range listValue {
				validationErrors = append(validationErrors, v.validateNode(itemSchema, entry, fmt.Sprintf("%s[%d]", path, i))...)
			}
		}
	}

	if minimum, ok := schema["minimum"].(float64); ok {
		if numeric, ok := toFloat(value); ok && numeric < minimum {
			validationErrors = append(validationErrors, fmt.Sprintf("%s expected >= %v", path, minimum))
		}
	}
	if maximum, ok := schema["maximum"].(float64); ok {
		if numeric, ok := toFloat(value); ok && numeric > maximum {
			validationErrors = append(validationErrors, fmt.Sprintf("%s expected <= %v", path, maximum))
		}
	}

	if enumValues, ok := schema["enum"].([]any); ok && !enumContains(enumValues, value) {
		parts := make([]string, 0, len(enumValues))
		for _, entry := range enumValues {
			parts = append(parts, fmt.Sprint(entry))
		}
		validationErrors = append(validationErrors, fmt.Sprintf("%s expected one of %s", path, strings.Join(parts, ", ")))
	}

	return validationErrors
}

func matchesSchemaType(expectedType any, value any) bool {
	switch typed := expectedType.(type) {
	case []any:
		for _, entry := range typed {
			if matchesSchemaType(entry, value) {
				return true
			}
		}
		return false
	case string:
		switch typed {
		case "object":
			_, ok := value.(map[string]any)
			return ok
		case "array":
			_, ok := value.([]any)
			return ok
		case "string":
			_, ok := value.(string)
			return ok
		case "integer":
			switch value.(type) {
			case int, int64, uint64:
				return true
			case float64:
				return value.(float64) == float64(int(value.(float64)))
			default:
				return false
			}
		case "number":
			_, ok := toFloat(value)
			return ok
		case "null":
			return value == nil
		default:
			return true
		}
	default:
		return true
	}
}

func schemaLiteralEquals(left, right any) bool {
	switch leftTyped := left.(type) {
	case map[string]any:
		rightTyped, ok := right.(map[string]any)
		if !ok || len(leftTyped) != len(rightTyped) {
			return false
		}
		for key, leftValue := range leftTyped {
			rightValue, ok := rightTyped[key]
			if !ok || !schemaLiteralEquals(leftValue, rightValue) {
				return false
			}
		}
		return true
	case []any:
		rightTyped, ok := right.([]any)
		if !ok || len(leftTyped) != len(rightTyped) {
			return false
		}
		for i, leftValue := range leftTyped {
			if !schemaLiteralEquals(leftValue, rightTyped[i]) {
				return false
			}
		}
		return true
	default:
		return left == right
	}
}

func enumContains(values []any, target any) bool {
	for _, candidate := range values {
		if schemaLiteralEquals(candidate, target) {
			return true
		}
	}
	return false
}

func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}
