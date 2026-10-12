# Cloudflare D1 + R2 图片 API

这是一个部署在 Cloudflare Workers 上的 Go + Gin 后端。图片二进制保存在 R2，图片元数据保存在 D1；原有消息接口保持不变。

## 图片数据表

表名：`images`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | INTEGER | 自增主键 |
| `original_name` | TEXT | 上传时的原始文件名 |
| `object_key` | TEXT | R2 中唯一的对象键 |
| `content_type` | TEXT | 根据文件内容识别出的 MIME 类型 |
| `size` | INTEGER | 文件字节数 |
| `etag` | TEXT | R2 返回的 ETag |
| `created_at` | TEXT | 创建时间（UTC） |

建表 SQL 位于 `backend/schema.sql`。

## 图片接口

### 上传图片

`POST /images`

- 请求类型：`multipart/form-data`
- 文件字段名：`image`
- 文件上限：10 MiB
- 仅接受能够识别为 `image/*` 的文件内容
- 成功状态码：`201 Created`

```bash
curl -X POST "http://127.0.0.1:8787/images" \
  -F "image=@./photo.png"
```

响应示例：

```json
{
  "id": 1,
  "original_name": "photo.png",
  "object_key": "images/2026/10/0123456789abcdef0123456789abcdef.png",
  "content_type": "image/png",
  "size": 12345,
  "etag": "...",
  "created_at": "2026-10-12 10:00:00",
  "download_url": "/images/1/download"
}
```

### 图片列表

`GET /images`

从 D1 按 `id` 倒序返回全部图片元数据；没有数据时返回 `[]`。

```bash
curl "http://127.0.0.1:8787/images"
```

### 下载图片

`GET /images/:id/download`

根据 D1 记录读取 R2 对象，以附件形式返回原始图片。

```bash
curl -OJ "http://127.0.0.1:8787/images/1/download"
```

## 本地开发

在 `backend` 目录执行：

```bash
npm install
go mod tidy
npm run db:init:local
npm run dev
```

Wrangler 会在本地模拟 D1 和 R2，默认地址为 `http://127.0.0.1:8787`。

## 部署

配置文件 `backend/wrangler.jsonc` 使用：

- D1 绑定：`DB` → `cloudflare-test-db`
- R2 绑定：`IMAGES` → `cloudflare-test-images`

首次部署前，在 `backend` 目录执行：

```bash
npx wrangler login
npm run r2:create
npm run db:init:remote
npm run deploy
```

如果 R2 桶已在 Cloudflare 控制台创建，请跳过 `npm run r2:create`。如果修改了数据库名或 R2 桶名，请同步修改 `wrangler.jsonc`。

## Cloudflare 控制台配置

1. 进入 **Storage & databases → R2 → Overview**，创建名为 `cloudflare-test-images` 的桶。
2. 进入 **Storage & databases → D1**，打开 `cloudflare-test-db`，在 **Console** 中执行 `backend/schema.sql` 的 SQL。
3. 进入 **Workers & Pages → cloudflare-test → Bindings**，确认：
   - D1 database 变量名为 `DB`，资源为 `cloudflare-test-db`；
   - R2 bucket 变量名为 `IMAGES`，资源为 `cloudflare-test-images`。
4. 绑定发生变更后重新部署 Worker，再使用上面的 curl 命令测试线上 URL。

R2 桶不需要开启公开访问；下载由 Worker 接口完成。

## 项目结构

```text
├── backend/
│   ├── main.go
│   ├── schema.sql
│   ├── wrangler.jsonc
│   ├── package.json
│   └── scripts/build.mjs
└── frontend/
```

注意：D1 的 `datetime('now')` 返回 UTC 时间。
