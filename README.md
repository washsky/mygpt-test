# mygpt-test

一个纯 Go 的本地 Web 示例，包含计算器和轻量文件管理。文件默认保存在可执行文件旁的 `mygpt-test-data` 目录中；升级二进制不会覆盖该目录。

## 运行

在仓库根目录运行：

```sh
go run ./cmd/mygpt-test
```

默认监听 `127.0.0.1:8080`。打开终端打印的地址使用计算器，访问 `/files` 管理上传文件。

也可以指定端口和数据目录：

```sh
./mygpt-test -addr 127.0.0.1:9090 -data-dir ./my-data
```

数据目录也可通过 `MYGPT_DATA_DIR` 环境变量指定，命令行 `-data-dir` 优先级更高。若要从其他设备访问，可显式设置 `-addr 0.0.0.0:8080`；文件管理目前没有用户登录，建议只在可信网络使用。

数据目录结构：

```text
mygpt-test-data/
├── files/
│   ├── uploads/    # 上传的文件，使用随机内部文件名
│   └── metadata/   # 文件元数据与预留的帖子/回复/用户关联
├── database/       # 为后续 SQLite 数据库预留
├── config/         # 配置文件预留
└── tmp/            # 临时文件
```

## 文件管理

文件管理页面支持上传、列表、下载和删除；单个文件上限 20 MB。上传时可以预留关联类型和记录 ID，类型包括 `post`、`reply`、`user`。当前元数据使用本地 JSON 文件，之后可将 `filemanager.Store` 换成 SQLite 实现，并把关联 ID 作为外键或关联表记录。

主要接口：

- `GET /api/files`：列出文件及元数据
- `POST /api/files`：multipart 上传，文件字段名为 `file`，可附带 `related_type` 和 `related_id`
- `GET /api/files/{id}`：下载文件
- `DELETE /api/files/{id}`：删除文件
- `PUT /api/files/{id}/association`：更新关联，JSON 示例：`{"type":"post","id":"42"}`

例如：

```sh
curl -F 'file=@photo.jpg' -F 'related_type=post' -F 'related_id=42' \
  http://127.0.0.1:8080/api/files
```

下载响应按附件处理，不会把用户上传的 HTML/SVG 当作网页执行。当前示例不包含用户认证、权限控制或 SQLite 数据库；不要把未加认证的文件管理接口直接开放到公网。

## 计算 API

```sh
curl -X POST http://127.0.0.1:8080/api/calculate \
  -H 'Content-Type: application/json' \
  -d '{"a":12,"b":3,"op":"*"}'
```

支持 `+`、`-`、`*`、`/` 四种运算。健康检查地址为 `/healthz`，`./mygpt-test -version` 可查看程序版本。

## GitHub Actions

- **CI** 仅手动运行，执行格式检查、测试和编译检查。
- **Release** 仅手动运行，会读取最新的正式版本标签、递增补丁版本、创建新标签并发布 Linux、Android、Windows 和 macOS 二进制文件。
