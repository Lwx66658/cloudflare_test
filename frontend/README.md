# 前端

这是一个无框架、无构建步骤的静态前端。生产环境中，它会作为 Worker Static Assets 与 Go API 一起部署，默认调用当前域名下的后端接口。

## 本地预览

在项目根目录运行：

```powershell
npx wrangler pages dev frontend/dist --port 8788
```

打开 `http://127.0.0.1:8788`。如果前后端分别启动，可在页面顶部填写后端地址 `http://127.0.0.1:8787`。

## Cloudflare 部署

项目默认采用单 Worker 部署，不需要单独创建 Pages 项目。`backend/wrangler.jsonc` 中的 `assets.directory` 指向 `../frontend/dist`，运行下面的命令会同时上传后端 Worker 和前端静态文件：

```powershell
cd backend
npm run deploy
```

Cloudflare Git 构建的 Root directory 设置为 `backend`，Build command 设置为 `npm run build`，Deploy command 设置为 `npx wrangler deploy`。
