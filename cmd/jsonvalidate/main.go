// jsonvalidate 是 jsonvalidator 库的命令行演示程序。
//
// 用法：
//
//	jsonvalidate                 运行内置示例（等价于 -demo）
//	jsonvalidate -demo           运行内置示例
//	jsonvalidate -rule r.json -data d.json   用指定规则文件校验数据文件
//
// 退出码：0 = 校验通过；1 = 存在校验错误；2 = 参数、文件或 JSON 解析错误。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"jsonvalidator"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("jsonvalidate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rulePath := fs.String("rule", "", "path to the rule JSON file")
	dataPath := fs.String("data", "", "path to the data JSON file")
	demo := fs.Bool("demo", false, "run the built-in demo rule and data")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var ruleJSON, dataJSON []byte
	switch {
	case *demo || (*rulePath == "" && *dataPath == ""):
		fmt.Fprintln(stdout, "running built-in demo")
		fmt.Fprintln(stdout, "---- demo data ----")
		fmt.Fprintln(stdout, demoData)
		fmt.Fprintln(stdout, "-------------------")
		ruleJSON, dataJSON = []byte(demoRule), []byte(demoData)
	case *rulePath == "" || *dataPath == "":
		fmt.Fprintln(stderr, "error: -rule and -data must be used together")
		fs.Usage()
		return 2
	default:
		var err error
		if ruleJSON, err = os.ReadFile(*rulePath); err != nil {
			fmt.Fprintf(stderr, "error: cannot read rule file: %v\n", err)
			return 2
		}
		if dataJSON, err = os.ReadFile(*dataPath); err != nil {
			fmt.Fprintf(stderr, "error: cannot read data file: %v\n", err)
			return 2
		}
	}

	validator, err := jsonvalidator.NewValidatorFromJSON(ruleJSON)
	if err != nil {
		fmt.Fprintf(stderr, "error: invalid rule: %v\n", err)
		return 2
	}
	data, err := jsonvalidator.ParseDataJSON(dataJSON)
	if err != nil {
		fmt.Fprintf(stderr, "error: invalid data: %v\n", err)
		return 2
	}

	errs := validator.Validate(data)
	if len(errs) == 0 {
		fmt.Fprintln(stdout, "valid: no errors")
		return 0
	}
	fmt.Fprintf(stdout, "invalid: %d error(s)\n", len(errs))
	for _, e := range errs {
		fmt.Fprintf(stdout, "  %s\n", e)
	}
	return 1
}

// 内置示例：规则覆盖五种类型、嵌套对象、数组、required、
// additionalProperties=false、enum、长度与数值边界。
const demoRule = `{
  "type": "object",
  "properties": {
    "user": {
      "type": "object",
      "properties": {
        "name":   {"type": "string", "minLength": 1, "maxLength": 20},
        "age":    {"type": "number", "minimum": 0, "maximum": 150},
        "role":   {"type": "string", "enum": ["admin", "user", "guest"]},
        "active": {"type": "boolean"}
      },
      "required": ["name", "age"],
      "additionalProperties": false
    },
    "tags": {
      "type": "array",
      "items": {"type": "string"},
      "minItems": 1,
      "maxItems": 5
    },
    "items": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "sku":   {"type": "string", "minLength": 1},
          "price": {"type": "number", "minimum": 0, "maximum": 10000},
          "qty":   {"type": "number", "minimum": 1}
        },
        "required": ["sku", "price"],
        "additionalProperties": false
      },
      "maxItems": 100
    }
  },
  "required": ["user", "items", "tags"],
  "additionalProperties": false
}`

// 内置示例数据故意包含多处错误，用于演示错误收集与路径输出。
const demoData = `{
  "user": {"name": "", "age": 200, "role": "superuser", "extra": true},
  "items": [
    {"sku": "A-1", "price": 9.5, "qty": 2},
    {"sku": "B-2"},
    {"sku": "C-3", "price": -5}
  ],
  "debug": true
}`
