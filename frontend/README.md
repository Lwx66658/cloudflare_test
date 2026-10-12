# 前端

这是一个无框架、无构建步骤的静态前端，用于调用 Cloudflare Worker 后端。

## 本地预览

在项目根目录运行：

```powershell
npx wrangler pages dev frontend/dist --port 8788
```

打开 `http://127.0.0.1:8788`，在页面顶部填写后端地址。后端本地开发地址默认为 `http://127.0.0.1:8787`。

## Cloudflare Pages 配置

- Root directory：`frontend`
- Build command：留空
- Build output directory：`dist`

部署后，在页面顶部填写 Worker 的 `https://...workers.dev` 地址并保存。
