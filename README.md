# mygpt-test

单文件运行的本地 Go 论坛与博客。首页可以浏览主题、发布文章、发表回复并上传附件；保留 `/files` 文件管理和 `/calculator` 计算器。升级二进制时，程序旁的 `mygpt-test-data` 数据目录继续沿用。

## 运行

下载 GitHub Release 中适合系统的二进制文件后直接运行，默认访问 `http://127.0.0.1:8080/`。也可使用以下参数：

```sh
./mygpt-test -addr 127.0.0.1:9090 -data-dir ./my-data
```

开发环境使用 `go run ./cmd/mygpt-test -data-dir ./mygpt-test-data`。数据目录也可通过 `MYGPT_DATA_DIR` 指定，`-data-dir` 优先。默认仅监听本机；若手动设置 `-addr 0.0.0.0:8080`，访客文章、回复及文件上传接口都可以从网络访问，目前没有完整的用户账户或反滥用机制，请仅部署在可信网络中。

## 使用论坛

首次启动会生成“公告”主题。文章和回复支持纯文本内容；选择附件后提交，程序先创建文章或回复，再逐个上传并关联附件。如果附件上传失败，页面会保留已发布内容并显示失败文件。管理员可创建更多主题，在首页的“设置”中更改站点名称、简介和访客发帖/回复开关。管理员还可编辑帖子和回复；删除内容会先移入回收站，附件关系会保留，支持恢复或彻底删除。文件删除也会进入回收站。

主题、帖子和回复旁边提供复制按钮，可复制主题信息、正文或回复内容；帖子详情也能复制可分享链接。

管理员令牌首次运行自动生成于 `mygpt-test-data/config/admin-token`，创建主题、修改设置、管理帖子回复、删除/恢复文件或清空回收站时输入即可。令牌只保存在浏览器当前会话中。把“允许访客发布帖子”或“允许访客回复”关闭后，管理员仍可用令牌发布。上传接口仍允许访客上传文件，单文件最大 20 MB；目前没有完整用户账户和反滥用机制，请保持本地监听。

文件页面 `/files` 可上传、关联已有文章或回复 ID、查看列表、预览、下载和移入回收站。单文件最大 20 MB。文本预览限制为 1 MB，支持 TXT、Markdown、LOG、JSON 格式化及 CSV 表格；PNG/JPEG/GIF/WebP 和 PDF 可以直接预览。其他类型仅下载，HTML/SVG 不会在站点来源下作为网页执行。彻底删除回收站中的文件后无法恢复。

论坛首页“设置”还提供两个默认关闭的选项：启用帖子搜索（搜索标题、正文和署名），以及启用正文资源渲染（安全显示 HTTPS 图片、链接和图片附件）。资源渲染使用 DOM 文本节点构造内容，不执行帖子中的 HTML 或脚本。

## 数据与升级

```text
mygpt-test-data/
├── files/
│   ├── uploads/             # 文件内容，随机内部文件名
│   └── metadata/            # 与旧版本兼容的 JSON 文件元数据
├── database/
│   ├── catalog.sqlite       # 主题、文章索引、回复索引、设置及附件关系
│   └── posts-YYYY-MM.sqlite # 按文章创建月份存储正文和回复
├── config/
│   └── admin-token          # 管理员令牌，首次启动自动创建
└── tmp/
```

回复跟随所属文章保存在同一月份分片；附件内容始终在 `files/uploads`，关系在目录数据库。保留并备份整个数据目录，包括 SQLite 的临时日志文件。不要在另一个程序进程中同时操作这些数据库；Android 构建采用 SQLite 点锁兼容模式，尤其不要用其他 SQLite 工具并发访问同一数据库。

旧版本上传的文件和 JSON 文件元数据会保留；回收站状态也保存在这些元数据中。帖子软删除字段会在启动时迁移到目录数据库，回复软删除字段会随对应月份分片按需升级。旧版本设置的帖子/用户占位关联不会自动变成论坛中的有效帖子关系。新的附件请从文章或回复表单上传，或在文件管理页输入真实的帖子/回复 ID。

## API

- `GET /api/forum/settings`、`PUT /api/forum/settings`：读取或修改设置
- `GET /api/forum/topics`、`POST /api/forum/topics`：主题列表或创建主题
- `GET /api/forum/posts?topic_id=...&limit=20&offset=0`、`POST /api/forum/posts`：文章列表或创建文章
- `GET /api/forum/posts?q=...`：搜索；需先在论坛设置中启用
- `GET /api/forum/posts/{id}`、`POST /api/forum/posts/{id}/replies`：文章、回复及其附件
- `PUT /api/forum/posts/{id}`、`DELETE /api/forum/posts/{id}`：管理员编辑文章或将其移入回收站
- `PUT /api/forum/replies/{id}`、`DELETE /api/forum/replies/{id}`：管理员编辑回复或将其移入回收站
- `GET /api/forum/trash`、`POST /api/forum/trash/posts/{id}/restore`、`POST /api/forum/trash/replies/{id}/restore`：查看或恢复帖子/回复
- `DELETE /api/forum/trash/posts/{id}`、`DELETE /api/forum/trash/replies/{id}`、`DELETE /api/forum/trash`：彻底删除回收站内容或清空
- `GET /api/files`、`POST /api/files`、`GET /api/files/{id}`、`GET /api/files/{id}/preview`：上传与读取文件
- `DELETE /api/files/{id}`、`GET /api/files/trash`、`POST /api/files/{id}/restore`、`DELETE /api/files/{id}/purge`、`DELETE /api/files/trash`：移入、查看、恢复或彻底删除文件
- `PUT /api/files/{id}/association`：更改文件关联
- `POST /api/calculate`：原有计算器接口；`GET /healthz`：健康检查

修改设置、创建主题、回收站操作、修改文件关联、删除或恢复文件需要请求头 `X-Admin-Token`。上传时使用 multipart 字段 `file`、`related_type=post|reply` 和实际记录 `related_id`；文章/回复表单上传会自动关联，文件管理页上传可选择关联。默认上传不关联任何内容。

## GitHub Actions

CI 和 Release 均只在手动触发时运行。CI 在 GitHub 上解析 Go 依赖、测试和编译；Release 测试通过后将最新正式版本的补丁号加一，创建版本标签，并构建 Linux、Android、Windows、macOS 的二进制文件。SQLite 使用不依赖 CGO 的驱动；Release 的 Android 构建启用点锁支持。
