# jsonvalidator — 轻量级本地 JSON 规则校验器

一个纯 Go 实现的本地 JSON 规则校验器：后端库 + 命令行演示 + 自动化测试。
不依赖数据库、Docker、云服务或任何第三方模块（仅使用 Go 标准库），也**不是**完整 JSON Schema 的实现。

## 功能概览

- 五种类型：object、array、string、number、boolean
- 递归校验嵌套对象与数组，一次校验尽量收集全部错误
- 错误带 JSONPath 风格路径（$.user.name、$.items[2].price），按（路径, 错误类型, 消息）排序，多次运行结果稳定一致
- 规则自身合法性检查：未知类型、minimum > maximum、minLength > maxLength、minItems > maxItems、负数长度/数量、空 enum、约束字段用错类型、未知 JSON 键等都会被拒绝
- Validator 创建后不可变，可被多个 goroutine 安全并发调用 Validate

## 规则格式

规则本身是 JSON，每个节点必须含 type 字段：

| 类型 | 支持的约束 |
| --- | --- |
| object | properties（字段规则表）、required（必填字段名数组）、additionalProperties: false（禁止未知字段） |
| array | items（元素规则）、minItems、maxItems |
| string | minLength、maxLength（按 Unicode 字符数计）、enum（允许值列表） |
| number | minimum、maximum（均为闭区间） |
| boolean | 无 |

示例（见 examples/rule.json）：

```json
{
  "type": "object",
  "properties": {
    "name":   {"type": "string", "minLength": 1, "maxLength": 20},
    "age":    {"type": "number", "minimum": 0, "maximum": 150},
    "role":   {"type": "string", "enum": ["admin", "user", "guest"]},
    "tags":   {"type": "array", "items": {"type": "string"}, "minItems": 1, "maxItems": 3},
    "active": {"type": "boolean"}
  },
  "required": ["name", "age"],
  "additionalProperties": false
}
```

约束字段只能用于对应类型（例如 minLength 不允许出现在 number 规则上），未声明的 JSON 键会被拒绝。

## 错误类型

Error.Type 取值：type、required、unknown_field、min_length、max_length、enum、minimum、maximum、min_items、max_items。

## 库用法

```go
rule, err := jsonvalidator.ParseRule(ruleText)   // 从 JSON 文本解析并校验规则
if err != nil { /* *jsonvalidator.RuleError，列出全部非法配置 */ }
data, err := jsonvalidator.ParseData(dataText)    // 从 JSON 文本解析待校验数据
if err != nil { /* JSON 语法错误 */ }

v, err := jsonvalidator.New(rule)                 // 创建后可并发使用
if err != nil { /* 规则非法 */ }
errs := v.Validate(data)
for _, e := range errs {
    fmt.Printf("%s [%s] %s
", e.Path, e.Type, e.Message)
}
```

Validate 返回 nil 表示校验通过；否则返回按（Path, Type, Message）排序的 []Error。

## 命令行演示

```powershell
# 内置示例（数据故意包含 5 处错误，退出码为 1）
go run ./cmd/demo

# 指定规则文件和数据文件
go run ./cmd/demo -rule examples/rule.json -data examples/data.json
```

输出示例：

```text
invalid: 2 error(s)
  $.age [maximum] value 200 is greater than maximum 150
  $.extra [unknown_field] unknown field "extra"
```

退出码：0 = 校验通过，1 = 存在校验错误，2 = 参数/文件/JSON 解析错误。

## 测试与检查（可重复运行）

```powershell
# 全部单元测试与 CLI 测试
go test ./...

# 使用 race detector 验证并发安全（含 32 goroutine 并发 Validate 用例）
go test -race ./...

# 静态检查与格式化检查
go vet ./...
gofmt -l .
```

测试覆盖：五种类型、嵌套对象路径、数组元素路径、required、additionalProperties=false、enum、长度与数值边界（闭区间）、非法规则配置、多错误同时返回、错误顺序稳定性（50 次重复比对）、并发 Validate、CLI 基本流程（内置示例/文件输入/坏规则/坏 JSON/文件缺失）。

## 项目结构

```text
rule.go            规则类型定义、ParseRule、规则自校验（CheckRule/RuleError）
validate.go        Error 类型、Validator、递归校验、路径构造、稳定排序
json.go            ParseData：从 JSON 文本解析待校验数据
cmd/demo/          命令行演示（内置示例，-rule/-data 指定文件）
examples/          示例规则与数据文件
*_test.go          自动化测试
```

## 明确不支持

完整 JSON Schema 标准、网络服务、GUI、数据库、磁盘持久化、运行时修改规则。需要修改规则时请创建新的 Validator。

