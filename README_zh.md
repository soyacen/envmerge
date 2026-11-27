# envmerge

一个轻量、快速、几乎无外部依赖的 **dotenv 模板编译器**。它让 `.env.example`
文件之间通过 `include` 指令相互引用，并编译成扁平、确定性的 `.env` 文件——
让拥有众多服务的 monorepo 复用同一套基础设施配置，而不是在每个文件里重复抄写。

中文 | [English](README.md)

```dotenv
# .api.env.example
include ./template/base.env.example
include ./template/mysql.env.example
include ./template/redis.env.example

# HTTP Server
HTTP_HOST=0.0.0.0
HTTP_PORT=8080

# 覆盖某个共享默认值
LOG_LEVEL=debug
```

```bash
envmerge generate .api.env.example   # -> .api.env（不含 include，可直接使用）
```

## 为什么要用它

- **可组合** —— 引用共享模板，并支持递归引用（模板还能再引用其他模板）。
- **保留文档** —— 每个模板里的每条注释都会原样进入生成文件：文件头注释、分段
  说明、只有注释的「默认值目录」模板、`include` 上方的注释、以及文件末尾的悬空
  注释。无需开关，也不会丢失。
- **私有优先** —— 服务自身的变量始终覆盖其引用的公共模板。
- **确定性** —— 相同输入始终产出逐字节一致的结果。
- **安全** —— 绝不执行 shell 命令；不带 `--force` 绝不覆盖已存在的 `.env`；
  绝不打印敏感变量的值。
- **单一二进制** —— 仅使用 Go 标准库加一个 CLI 框架编译，无运行时依赖。

## 安装

预编译二进制见 [releases](../../releases) 页面。从源码构建（需 Go 1.27+）：

```bash
go build -o envmerge ./cmd/envmerge
# 或直接安装当前发布版本：
go install github.com/soyacen/envmerge/cmd/envmerge@latest
```

## 命令

| 命令             | 用途                                                   |
| ---------------- | ------------------------------------------------------ |
| `generate`       | 将 `.env.example` 模板编译生成 `.env` 文件              |
| `validate`       | 校验语法、被引用文件存在性与是否成环                    |
| `check`          | 生成文件与模板不一致时失败（退出码 4，用于 CI）          |
| `tree`           | 打印依赖树及统计数量                                    |
| `explain`        | 解释某个变量最终值的来源                                |
| `diff`           | 显示模板生成结果与现有文件之间的差异                    |
| `watch`          | 模板或其依赖变化时自动重新生成                          |

完整参数说明见 [`docs/cli.md`](docs/cli.md)，模板语法规范见
[`docs/syntax.md`](docs/syntax.md)。

## 合并模型（就近原则）

重复变量按「相对入口模板的接近程度」再按声明顺序裁决：

1. 某文件里自身声明的变量，优先于从 `include` 引入的同名变量（私有优先）。
2. 同一深度的多个 include 之间，后声明者优先。
3. 同一文件内的重复定义，后出现的一行优先。

每个变量在输出中只出现一次，位于它**第一次出现**的位置；`include` 在它被声明的
地方展开，因此生成文件保持模板书写时的分节顺序。注释跟着它所要说明的行：写在
「冲突落选声明」上方的注释块会被搬到最终生效的赋值行正上方（来源可由 `explain`
与 `--verbose` 查到），因此不会有注释被丢弃或变成无主的孤立注释。

`--conflict` 提供四种策略：`local-wins`（默认，即上述规则）、`last-wins`、
`first-wins`、`error`。

## CI/CD

```yaml
- run: envmerge check --dir ./deploy/env        # 任一 .env 漂移则退出码 4
- run: envmerge generate --dir ./deploy/env --force
```

## SDK

引擎也可作为库被其他 Go 程序引用：

```go
import "github.com/soyacen/envmerge/pkg/envmerge"

engine := envmerge.New(envmerge.Options{
    ConflictPolicy: envmerge.LocalWins, // 注释与空行分组始终保留
})
res, err := engine.Generate(ctx, ".api.env.example", "")
```

## 许可证

MIT —— 详见 [LICENSE](LICENSE)。
