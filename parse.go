package jsonvalidator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// ParseRuleJSON 从 JSON 文本解析规则。
// 非法 JSON、未知字段、未知类型以及明显非法的约束配置都会返回错误。
func ParseRuleJSON(ruleJSON []byte) (*Rule, error) {
	var rule Rule
	if err := decodeSingleJSON(ruleJSON, &rule); err != nil {
		return nil, fmt.Errorf("invalid rule JSON: %w", err)
	}
	if err := rule.validate(); err != nil {
		return nil, err
	}
	return &rule, nil
}

// ParseDataJSON 从 JSON 文本解析待校验数据，
// 返回 encoding/json 默认解码形式的值，可直接传给 Validate。
func ParseDataJSON(dataJSON []byte) (any, error) {
	var data any
	if err := decodeSingleJSON(dataJSON, &data); err != nil {
		return nil, fmt.Errorf("invalid data JSON: %w", err)
	}
	return data, nil
}

// decodeSingleJSON 解码恰好一个 JSON 值，拒绝空输入和尾随的多余内容。
func decodeSingleJSON(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("unexpected trailing data after JSON value")
	}
	return nil
}
