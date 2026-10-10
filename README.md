# Cloudflare D1 Message API

这是一个部署在 Cloudflare Workers 上的简单后端服务，使用 Go + Gin + D1 数据库。

## 项目结构

```
├── main.go              # 主程序（Gin 路由 + D1 操作）
├── schema.sql           # D1 数据库表结构
├── go.mod / go.sum      # Go 依赖
├── package.json         # npm 脚本（wrangler 构建和部署）
├── wrangler.jsonc       # Cloudflare Workers 配置
├── scripts/build.mjs    # 跨平台构建脚本（编译 Wasm）
└── build/               # 构建输出（app.wasm + worker.mjs）
```

## 数据库设计

**表名：** `messages`

| 字段        | 类型    | 说明                       |
|-------------|---------|----------------------------|
| id          | INTEGER | 主键，自增                 |
| content     | TEXT    | 消息内容                   |
| create_time | TEXT    | 创建时间（ISO8601 格式）   |

## API 接口

### 1. 健康检查
- **路径：** `GET /`
- **返回：** `{"status":"ok"}`

### 2. 创建消息
- **路径：** `POST /messages`
- **请求体：** 纯文本（消息内容）
- **返回：** `201 Created`
  ```json
  {
    "id": 1,
    "content": "Hello from Cloudflare D1!",
    "create_time": "2026-10-10 07:15:23"
  }
  ```

### 3. 查询最新消息
- **路径：** `GET /messages/latest`
- **返回：** `200 OK`
  ```json
  {
    "id": 2,
    "content": "Testing latest message endpoint",
    "create_time": "2026-10-10 07:15:23"
  }
  ```
- **无消息时：** `404 Not Found` + `{"error":"no messages yet"}`

## 本地开发

### 1. 安装依赖
```bash
npm install
go mod tidy
```

### 2. 初始化本地 D1 数据库
```bash
npm run db:init:local
```

### 3. 本地运行
```bash
npm run dev
```
服务将运行在 `http://127.0.0.1:8787`

### 4. 测试（Postman 或 curl）
```bash
# 创建消息
curl -X POST http://127.0.0.1:8787/messages -d "Hello World"

# 获取最新消息
curl http://127.0.0.1:8787/messages/latest
```

## 部署到 Cloudflare

### 1. 登录 Cloudflare
```bash
npx wrangler login
```

### 2. 创建 D1 数据库
```bash
npx wrangler d1 create cloudflare-test-db
```
命令会返回 `database_id`，复制它。

### 3. 更新 wrangler.jsonc
打开 `wrangler.jsonc`，将 `database_id` 字段替换为上一步获取的 ID：
```json
{
  "d1_databases": [
    {
      "binding": "DB",
      "database_name": "cloudflare-test-db",
      "database_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
    }
  ]
}
```

### 4. 初始化远程数据库表
```bash
npm run db:init:remote
```

### 5. 部署
```bash
npm run deploy
```
部署成功后，Cloudflare 会返回 Worker 的 URL，例如：
```
https://cloudflare-test.<YOUR_SUBDOMAIN>.workers.dev
```

### 6. 测试线上环境
```bash
curl -X POST https://cloudflare-test.<YOUR_SUBDOMAIN>.workers.dev/messages -d "Production test"
curl https://cloudflare-test.<YOUR_SUBDOMAIN>.workers.dev/messages/latest
```

## 技术栈

- **语言：** Go 1.26.4
- **框架：** Gin (net/http 兼容)
- **运行时：** Cloudflare Workers (WebAssembly)
- **数据库：** Cloudflare D1 (SQLite-compatible)
- **构建工具：** wrangler 4.42.0

## 注意事项

1. **WASM 体积：** 当前 Wasm 文件约 23MB，如需优化可考虑改用 TinyGo。
2. **时区：** D1 的 `datetime('now')` 返回 UTC 时间。
3. **限制：** Cloudflare Workers 免费层有请求数限制，详见 [Cloudflare 定价](https://developers.cloudflare.com/workers/platform/pricing/)。
4. **错误处理：** 生产环境建议增加日志、限流等中间件。

## 许可

MIT
