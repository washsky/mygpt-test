# mygpt-test

一个纯 Go 的本地 Web 计算器示例。运行程序后，用浏览器打开终端打印出的地址即可使用。

## 运行

在仓库根目录运行：

```sh
go run ./cmd/mygpt-test
```

默认监听 `:8080`，然后打开 http://127.0.0.1:8080。

也可以指定监听地址：

```sh
go run ./cmd/mygpt-test -addr 127.0.0.1:9090
```

查看二进制版本：

```sh
./mygpt-test -version
```

## API

发送 JSON 请求：

```sh
curl -X POST http://127.0.0.1:8080/api/calculate \
  -H 'Content-Type: application/json' \
  -d '{"a":12,"b":3,"op":"*"}'
```

支持 `+`、`-`、`*`、`/` 四种运算。健康检查地址为 `/healthz`。

## GitHub Actions

- **CI** 工作流只通过 GitHub Actions 页面中的 **Run workflow** 手动运行，执行格式检查、测试和编译检查。
- **Release** 工作流同样手动运行。每次运行会读取最新的正式版本标签并递增补丁版本（例如 `v0.1.5` → `v0.1.6`），随后构建并发布 Linux、Android、Windows、macOS 二进制文件。
