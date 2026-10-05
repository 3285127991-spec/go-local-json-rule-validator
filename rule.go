package jsonvalidator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// RuleType 是规则支持的数据类型，共五种。
type RuleType string

const (
	TypeObject  RuleType = "object"
	TypeArray   RuleType = "array"
	TypeString  RuleType = "string"
	TypeNumber  RuleType = "number"
	TypeBoolean RuleType = "boolean"
)

// Rule 描述对 JSON 数据中某个节点的校验规则。
//
// 不同 Type 只使用与之相关的字段，其余字段必须保持零值，
// 否则 NewValidator 会拒绝该规则：
//   - object: Properties、Required、AdditionalProperties
//   - array: Items、MinItems、MaxItems
//   - string: MinLength、MaxLength、Enum
//   - number: Minimum、Maximum
//   - boolean: 无
type Rule struct {
	Type RuleType `json:"type"`

	// object 约束
	Properties           map[string]*Rule `json:"properties,omitempty"`
	Required             []string         `json:"required,omitempty"`
	AdditionalProperties *bool            `json:"additionalProperties,omitempty"`

	// array 约束
	Items    *Rule `json:"items,omitempty"`
	MinItems *int  `json:"minItems,omitempty"`
	MaxItems *int  `json:"maxItems,omitempty"`

	// string 约束
	MinLength *int     `json:"minLength,omitempty"`
	MaxLength *int     `json:"maxLength,omitempty"`
	Enum      []string `json:"enum,omitempty"`

	// number 约束
	Minimum *float64 `json:"minimum,omitempty"`
	Maximum *float64 `json:"maximum,omitempty"`
}

// UnmarshalJSON 从 JSON 解码规则。解码时拒绝未知字段、未知类型和缺失的
// type；约束合法性（如 minimum > maximum）由 ParseRuleJSON 和
// NewValidator 通过 validate 统一检查，以便报告嵌套规则的位置。
func (r *Rule) UnmarshalJSON(data []byte) error {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}
	if head.Type == "" {
		return errors.New(`invalid rule: missing "type"`)
	}

	// decode 严格解码：规则中不允许出现未定义的字段名。
	decode := func(body any) error {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(body); err != nil {
			return fmt.Errorf("invalid %s rule: %w", head.Type, err)
		}
		return nil
	}

	*r = Rule{}
	switch RuleType(head.Type) {
	case TypeObject:
		var body struct {
			Type                 string           `json:"type"`
			Properties           map[string]*Rule `json:"properties"`
			Required             []string         `json:"required"`
			AdditionalProperties *bool            `json:"additionalProperties"`
		}
		if err := decode(&body); err != nil {
			return err
		}
		r.Type = TypeObject
		r.Properties = body.Properties
		r.Required = body.Required
		r.AdditionalProperties = body.AdditionalProperties
	case TypeArray:
		var body struct {
			Type     string `json:"type"`
			Items    *Rule  `json:"items"`
			MinItems *int   `json:"minItems"`
			MaxItems *int   `json:"maxItems"`
		}
		if err := decode(&body); err != nil {
			return err
		}
		r.Type = TypeArray
		r.Items = body.Items
		r.MinItems = body.MinItems
		r.MaxItems = body.MaxItems
	case TypeString:
		var body struct {
			Type      string   `json:"type"`
			MinLength *int     `json:"minLength"`
			MaxLength *int     `json:"maxLength"`
			Enum      []string `json:"enum"`
		}
		if err := decode(&body); err != nil {
			return err
		}
		r.Type = TypeString
		r.MinLength = body.MinLength
		r.MaxLength = body.MaxLength
		r.Enum = body.Enum
	case TypeNumber:
		var body struct {
			Type    string   `json:"type"`
			Minimum *float64 `json:"minimum"`
			Maximum *float64 `json:"maximum"`
		}
		if err := decode(&body); err != nil {
			return err
		}
		r.Type = TypeNumber
		r.Minimum = body.Minimum
		r.Maximum = body.Maximum
	case TypeBoolean:
		var body struct {
			Type string `json:"type"`
		}
		if err := decode(&body); err != nil {
			return err
		}
		r.Type = TypeBoolean
	default:
		return fmt.Errorf("invalid rule: unknown type %q", head.Type)
	}
	return nil
}

// validate 递归检查整棵规则树，供 NewValidator 校验以编程方式构造的规则。
func (r *Rule) validate() error {
	if r == nil {
		return errors.New("rule must not be nil")
	}
	switch r.Type {
	case TypeObject, TypeArray, TypeString, TypeNumber, TypeBoolean:
	default:
		return fmt.Errorf("invalid rule: unknown type %q", r.Type)
	}
	if err := r.checkConstraints(); err != nil {
		return err
	}
	for _, name := range sortedKeys(r.Properties) {
		if err := r.Properties[name].validate(); err != nil {
			return fmt.Errorf("property %q: %w", name, err)
		}
	}
	if r.Items != nil {
		if err := r.Items.validate(); err != nil {
			return fmt.Errorf("items: %w", err)
		}
	}
	return nil
}

// checkConstraints 校验单个规则节点的约束配置是否合法。
func (r *Rule) checkConstraints() error {
	notAllowed := func(field string) error {
		return fmt.Errorf("invalid %s rule: %q is not allowed for this type", r.Type, field)
	}
	if r.Type != TypeObject {
		if r.Properties != nil {
			return notAllowed("properties")
		}
		if r.Required != nil {
			return notAllowed("required")
		}
		if r.AdditionalProperties != nil {
			return notAllowed("additionalProperties")
		}
	}
	if r.Type != TypeArray {
		if r.Items != nil {
			return notAllowed("items")
		}
		if r.MinItems != nil {
			return notAllowed("minItems")
		}
		if r.MaxItems != nil {
			return notAllowed("maxItems")
		}
	}
	if r.Type != TypeString {
		if r.MinLength != nil {
			return notAllowed("minLength")
		}
		if r.MaxLength != nil {
			return notAllowed("maxLength")
		}
		if r.Enum != nil {
			return notAllowed("enum")
		}
	}
	if r.Type != TypeNumber {
		if r.Minimum != nil {
			return notAllowed("minimum")
		}
		if r.Maximum != nil {
			return notAllowed("maximum")
		}
	}

	switch r.Type {
	case TypeObject:
		seen := make(map[string]bool, len(r.Required))
		deduped := r.Required[:0]
		for _, name := range r.Required {
			if name == "" {
				return errors.New("invalid object rule: required contains an empty field name")
			}
			if !seen[name] {
				seen[name] = true
				deduped = append(deduped, name)
			}
		}
		r.Required = deduped
	case TypeArray:
		if err := checkIntRange(r.Type, "minItems", r.MinItems, "maxItems", r.MaxItems); err != nil {
			return err
		}
	case TypeString:
		if err := checkIntRange(r.Type, "minLength", r.MinLength, "maxLength", r.MaxLength); err != nil {
			return err
		}
	case TypeNumber:
		if r.Minimum != nil && r.Maximum != nil && *r.Minimum > *r.Maximum {
			return fmt.Errorf("invalid number rule: minimum %v is greater than maximum %v", *r.Minimum, *r.Maximum)
		}
	}
	return nil
}

func checkIntRange(ruleType RuleType, minName string, minVal *int, maxName string, maxVal *int) error {
	if minVal != nil && *minVal < 0 {
		return fmt.Errorf("invalid %s rule: %s must not be negative", ruleType, minName)
	}
	if maxVal != nil && *maxVal < 0 {
		return fmt.Errorf("invalid %s rule: %s must not be negative", ruleType, maxName)
	}
	if minVal != nil && maxVal != nil && *minVal > *maxVal {
		return fmt.Errorf("invalid %s rule: %s %d is greater than %s %d", ruleType, minName, *minVal, maxName, *maxVal)
	}
	return nil
}
