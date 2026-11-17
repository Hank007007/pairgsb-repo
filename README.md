# rulelang

`rulelang` 是边缘防护网关的规则表达式编译器。控制台里写下的规则文本先经过词法和语法分
析得到表达式树，再对每个请求的字段上下文求值，得到拦截与否的布尔结果。

规则来自手写、模板复制与批量导入，因此编译器把任何输入都当作不可信：非法规则必须以带
位置信息的错误结束，不能使进程崩溃，也不能进入求值。

## 目录

```
cmd/rulelang          CLI：compile / eval / demo
internal/ast          表达式树节点与渲染
internal/lexer        词法分析
internal/parser       语法分析
internal/compiler     词法 + 语法的组合入口
internal/eval         表达式求值与请求上下文
internal/limits       编译期资源上限
internal/rlerr        贯穿各阶段的诊断错误
docs/rule-syntax.md   语法与求值语义规范
docs/design.md        设计说明
```

## 构建与测试

```sh
go build ./...
go test ./...
```

仅使用 Go 标准库。

## 使用

编译一条规则并打印语法树：

```sh
go run ./cmd/rulelang compile -rule 'src in 10.0.0.0/8 and method == "POST"'
```

对请求上下文求值：

```sh
go run ./cmd/rulelang eval \
  -rule 'src in 10.0.0.0/8 and method == "POST" and body.len > 4096' \
  -ctx '{"src":"10.7.7.7","method":"POST","body.len":8192}'
```

上下文是 JSON 对象。默认把名为 `src` 的字段按 IP 地址解释，可通过 `-ip` 指定多个字段
（逗号分隔）。数值字段映射为整数，字符串字段映射为字符串。

规则也可以从文件或标准输入读取：

```sh
go run ./cmd/rulelang compile -rule-file rule.txt
echo 'method == "POST"' | go run ./cmd/rulelang compile
```

运行内置演示，查看优先级、否定、成员判断和字符串转义的实际表现：

```sh
go run ./cmd/rulelang demo
```

## 错误

所有编译与求值错误都是 `*rlerr.Error`，形如：

```
parse: offset 7: expected expression, got end of input
```

`offset` 是源串中的字节位置。编译失败的规则不会被求值。
