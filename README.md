# go-local-json-rule-validator

一个轻量级的 Windows 本地 JSON 规则校验器，使用 Go 实现，仅包含后端库、
命令行演示和自动化测试。不依赖数据库、Docker、云服务或任何第三方服务，
只使用 Go 标准库。

校验器接收一份规则和一份待校验 JSON 数据，递归校验后返回全部可发现的
错误，每个错误都带有明确路径（如 `$.user.name`、`$.items[2].price`），
并按 (路径, 错误类型) 稳定排序。

本项目刻意**不**实现完整 JSON Schema 标准，也不提供网络服务、GUI、
数据库或磁盘持久化。

## 功能

- 五种规则类型：`object`、`array`、`string`、`number`、`boolean`
- `object`：`properties`、`required`、`additionalProperties: false`（拒绝未知字段）
- `array`：`items`、`minItems`、`maxItems`
- `string`：`minLength`、`maxLength`（按 Unicode 字符数计）、`enum`
- `number`：`minimum`、`maximum`（含端点）
- 一次校验尽量收集全部可继续发现的错误（类型错误的节点不再向下级联）
- 错误结果按 (路径, 错误类型, 描述) 排序，多次运行顺序稳定
- 规则自检：拒绝未知类型、未知字段、`minimum > maximum`、
  `minLength > maxLength`、`minItems > maxItems`、负数长度/个数、
  约束与类型不匹配（如 string 规则里写 `minimum`）等非法配置
- `Validator` 创建后规则不可变，`Validate` 可被多个 goroutine 安全并发调用
  （已用 race detector 验证）

## 项目结构

```
rule.go                    规则类型定义、JSON 解码、规则自检
validator.go               Validator、递归校验、错误类型与路径
parse.go                   ParseRuleJSON / ParseDataJSON 辅助函数
cmd/jsonvalidate/main.go   命令行演示（内置示例或指定规则/数据文件）
examples/                  示例规则与数据文件
*_test.go                  自动化测试
```

## 规则格式

规则本身是 JSON，`type` 必填，约束字段按类型区分，不允许出现与类型
无关的字段：

```json
{
  "type": "object",
  "properties": {
    "name": {"type": "string", "minLength": 1, "maxLength": 20},
    "age":  {"type": "number", "minimum": 0, "maximum": 150},
    "role": {"type": "string", "enum": ["admin", "user", "guest"]},
    "tags": {"type": "array", "items": {"type": "string"}, "minItems": 1, "maxItems": 5}
  },
  "required": ["name"],
  "additionalProperties": false
}
```

完整示例见 [examples/rule.json](examples/rule.json) 与
[examples/data.json](examples/data.json)。

## 错误与路径

错误类型 `ValidationError{Path, Kind, Message}`，`Kind` 取值：
`type`、`required`、`additional_property`、`min_items`、`max_items`、
`min_length`、`max_length`、`enum`、`minimum`、`maximum`。

路径以 `$` 为根，字段用 `.name`（非常规标识符用 `["name"]`），
数组元素用 `[下标]`，例如 `$.user.name`、`$.items[2].price`。

## 库用法

```go
import "jsonvalidator"

// 从 JSON 文本创建（也可以先 ParseRuleJSON 再 NewValidator）
v, err := jsonvalidator.NewValidatorFromJSON(ruleJSON)
if err != nil { /* 规则非法 */ }

// 方式一：直接校验 JSON 文本
errs, err := v.ValidateJSON(dataJSON)

// 方式二：先解析数据再校验（data 也可来自自己的 json.Unmarshal）
data, err := jsonvalidator.ParseDataJSON(dataJSON)
errs := v.Validate(data) // 无错误时返回空列表；可并发调用
```

## 命令行演示

```powershell
go run ./cmd/jsonvalidate                                      # 内置示例（数据故意含 8 处错误）
go run ./cmd/jsonvalidate -rule examples/rule.json -data examples/data.json
```

退出码：`0` 校验通过；`1` 存在校验错误；`2` 参数、文件或 JSON 解析错误。

## 运行测试

需要 Go 1.27+（race detector 需要 gcc，如 MSYS2/MinGW 提供的 gcc）：

```powershell
go test ./... -count=1        # 全部单元测试与 CLI 测试
go test -race ./... -count=1  # 使用 race detector 验证并发 Validate
go vet ./...
```

测试覆盖：五种类型、嵌套对象、数组、required、additionalProperties、
enum、长度与数值边界、非法规则、多错误同时返回、稳定错误顺序、
并发 Validate 以及 CLI 基本流程。
