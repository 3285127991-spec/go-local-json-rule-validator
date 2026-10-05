// Package jsonvalidator 提供一个轻量级的本地 JSON 规则校验器。
//
// 校验器接收一份规则（Rule）和一份待校验的 JSON 数据，递归校验后
// 返回全部可发现的错误，错误带有 $.user.name、$.items[2].price
// 形式的明确路径，并按 (路径, 错误类型) 稳定排序。
//
// 本包刻意不实现完整 JSON Schema 标准，只支持五种类型及其常用约束。
package jsonvalidator

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// ErrorKind 标识校验错误的类型，用于稳定排序和程序化判断。
type ErrorKind string

const (
	ErrType               ErrorKind = "type"                // 值类型与规则类型不符
	ErrRequired           ErrorKind = "required"            // 缺少 required 字段
	ErrAdditionalProperty ErrorKind = "additional_property" // additionalProperties=false 时出现未知字段
	ErrMinItems           ErrorKind = "min_items"           // 数组元素个数少于 minItems
	ErrMaxItems           ErrorKind = "max_items"           // 数组元素个数多于 maxItems
	ErrMinLength          ErrorKind = "min_length"          // 字符串长度小于 minLength
	ErrMaxLength          ErrorKind = "max_length"          // 字符串长度大于 maxLength
	ErrEnum               ErrorKind = "enum"                // 字符串不在 enum 允许值中
	ErrMinimum            ErrorKind = "minimum"             // 数值小于 minimum
	ErrMaximum            ErrorKind = "maximum"             // 数值大于 maximum
)

// ValidationError 描述单个校验错误。
type ValidationError struct {
	Path    string    // 出错位置，如 "$.user.name"、"$.items[2].price"
	Kind    ErrorKind // 错误类型
	Message string    // 面向阅读的错误描述
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: [%s] %s", e.Path, e.Kind, e.Message)
}

func (e ValidationError) String() string { return e.Error() }

// Validator 按固定规则校验 JSON 数据。
//
// Validator 创建后规则不再变化，Validate 只读取规则，
// 因此可安全地被多个 goroutine 并发调用。
type Validator struct {
	root *Rule
}

// NewValidator 用给定规则创建 Validator。规则不合法时返回错误。
// 创建后调用方不应再修改 rule。
func NewValidator(rule *Rule) (*Validator, error) {
	if rule == nil {
		return nil, errors.New("rule must not be nil")
	}
	if err := rule.validate(); err != nil {
		return nil, err
	}
	return &Validator{root: rule}, nil
}

// NewValidatorFromJSON 从 JSON 文本解析规则并创建 Validator。
func NewValidatorFromJSON(ruleJSON []byte) (*Validator, error) {
	rule, err := ParseRuleJSON(ruleJSON)
	if err != nil {
		return nil, err
	}
	return NewValidator(rule)
}

// Validate 递归校验 data，返回全部可以继续发现的错误。
//
// data 必须是由 encoding/json 解码得到的值，即
// map[string]any、[]any、string、float64、bool 或 nil 的组合。
// 返回的错误按 (Path, Kind, Message) 排序，多次调用结果稳定。
func (v *Validator) Validate(data any) []ValidationError {
	var errs []ValidationError
	v.validateValue(v.root, data, "$", &errs)
	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].Path != errs[j].Path {
			return errs[i].Path < errs[j].Path
		}
		if errs[i].Kind != errs[j].Kind {
			return errs[i].Kind < errs[j].Kind
		}
		return errs[i].Message < errs[j].Message
	})
	return errs
}

// ValidateJSON 解析 JSON 文本并校验，是 ParseDataJSON + Validate 的便捷组合。
func (v *Validator) ValidateJSON(dataJSON []byte) ([]ValidationError, error) {
	data, err := ParseDataJSON(dataJSON)
	if err != nil {
		return nil, err
	}
	return v.Validate(data), nil
}

func (v *Validator) validateValue(rule *Rule, value any, path string, errs *[]ValidationError) {
	add := func(kind ErrorKind, format string, args ...any) {
		*errs = append(*errs, ValidationError{Path: path, Kind: kind, Message: fmt.Sprintf(format, args...)})
	}
	typeMismatch := func() {
		add(ErrType, "expected %s, got %s", rule.Type, jsonTypeName(value))
	}

	switch rule.Type {
	case TypeObject:
		obj, ok := value.(map[string]any)
		if !ok {
			typeMismatch()
			return
		}
		for _, name := range rule.Required {
			if _, ok := obj[name]; !ok {
				*errs = append(*errs, ValidationError{
					Path:    joinPath(path, name),
					Kind:    ErrRequired,
					Message: fmt.Sprintf("required field %q is missing", name),
				})
			}
		}
		for _, name := range sortedKeys(rule.Properties) {
			if field, ok := obj[name]; ok {
				v.validateValue(rule.Properties[name], field, joinPath(path, name), errs)
			}
		}
		if rule.AdditionalProperties != nil && !*rule.AdditionalProperties {
			for _, name := range sortedKeys(obj) {
				if _, known := rule.Properties[name]; !known {
					*errs = append(*errs, ValidationError{
						Path:    joinPath(path, name),
						Kind:    ErrAdditionalProperty,
						Message: fmt.Sprintf("unknown field %q", name),
					})
				}
			}
		}

	case TypeArray:
		arr, ok := value.([]any)
		if !ok {
			typeMismatch()
			return
		}
		if rule.MinItems != nil && len(arr) < *rule.MinItems {
			add(ErrMinItems, "array has %d item(s), minimum is %d", len(arr), *rule.MinItems)
		}
		if rule.MaxItems != nil && len(arr) > *rule.MaxItems {
			add(ErrMaxItems, "array has %d item(s), maximum is %d", len(arr), *rule.MaxItems)
		}
		if rule.Items != nil {
			for i, item := range arr {
				v.validateValue(rule.Items, item, fmt.Sprintf("%s[%d]", path, i), errs)
			}
		}

	case TypeString:
		s, ok := value.(string)
		if !ok {
			typeMismatch()
			return
		}
		length := utf8.RuneCountInString(s)
		if rule.MinLength != nil && length < *rule.MinLength {
			add(ErrMinLength, "string length %d is less than minimum %d", length, *rule.MinLength)
		}
		if rule.MaxLength != nil && length > *rule.MaxLength {
			add(ErrMaxLength, "string length %d is greater than maximum %d", length, *rule.MaxLength)
		}
		if rule.Enum != nil {
			found := false
			for _, allowed := range rule.Enum {
				if s == allowed {
					found = true
					break
				}
			}
			if !found {
				add(ErrEnum, "value %q is not one of the allowed enum values", s)
			}
		}

	case TypeNumber:
		f, ok := value.(float64)
		if !ok {
			typeMismatch()
			return
		}
		if rule.Minimum != nil && f < *rule.Minimum {
			add(ErrMinimum, "value %v is less than minimum %v", f, *rule.Minimum)
		}
		if rule.Maximum != nil && f > *rule.Maximum {
			add(ErrMaximum, "value %v is greater than maximum %v", f, *rule.Maximum)
		}

	case TypeBoolean:
		if _, ok := value.(bool); !ok {
			typeMismatch()
		}
	}
}

// jsonTypeName 返回 JSON 解码值的类型名，用于类型错误信息。
func jsonTypeName(value any) string {
	switch value.(type) {
	case map[string]any:
		return string(TypeObject)
	case []any:
		return string(TypeArray)
	case string:
		return string(TypeString)
	case float64:
		return string(TypeNumber)
	case bool:
		return string(TypeBoolean)
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", value)
	}
}

var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// joinPath 拼接字段路径：常规标识符用 ".name"，其余用 '["name"]'。
func joinPath(base, key string) string {
	if identPattern.MatchString(key) {
		return base + "." + key
	}
	return base + `["` + strings.ReplaceAll(key, `"`, `\"`) + `"]`
}

// sortedKeys 返回排序后的键，保证遍历顺序稳定（Go map 遍历顺序随机）。
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
