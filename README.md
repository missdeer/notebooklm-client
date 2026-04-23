# notebooklm-client (Go)

Google [NotebookLM](https://notebooklm.google.com/) CLI 的 Go 重写版本 — 单一静态二进制，零运行时依赖。

生成音频播客、报告、幻灯片、测验、视频、信息图、数据表、闪卡，分析内容，管理笔记本，以及与笔记本对话。

> **Note:** [TypeScript 版本](https://github.com/icebear0828/notebooklm-client)是功能完备的原始实现，Go 版本与之对齐。

## 与 TypeScript 版本的区别

| | TypeScript | Go |
|---|---|---|
| 运行时 | Node.js 20+ | 无（静态二进制） |
| TLS 指纹 | undici / curl-impersonate / tls-client FFI | utls 原生 (99% Chrome 指纹) |
| 浏览器自动化 | Puppeteer | rod |
| 部署 | `npm install` + node_modules | 单个可执行文件 |

## 构建

```bash
go build -o notebooklm ./cmd/notebooklm
```

## 快速开始

### 1. 登录（首次）

```bash
# 方式一：通过浏览器登录并导出 session（自动探测 Chrome/Edge/Brave/Chromium）
npx @missdeer/notebooklm export-session

# 指定浏览器路径
npx @missdeer/notebooklm export-session --chrome-path "/path/to/browser"

# 方式二：从 TypeScript 版本导入已有 session
npx @missdeer/notebooklm import-session ~/.notebooklm/session.json
```

### 2. 使用

```bash
# 列出笔记本
npx @missdeer/notebooklm list

# 从 URL 生成音频播客
npx @missdeer/notebooklm audio --url "https://en.wikipedia.org/wiki/Go_(programming_language)" -o ./output -l en

# 辩论格式，短篇
npx @missdeer/notebooklm audio --topic "quantum computing" -o ./output --format debate --length short

# 生成报告
npx @missdeer/notebooklm report --url "https://example.com/article" -o ./output --template study_guide

# 生成幻灯片
npx @missdeer/notebooklm slides --url "https://example.com/article" -o ./output

# 生成测验
npx @missdeer/notebooklm quiz --url "https://example.com/article" -o ./output --difficulty medium

# 生成闪卡
npx @missdeer/notebooklm flashcards --url "https://example.com/article" -o ./output

# 生成视频
npx @missdeer/notebooklm video --url "https://example.com/article" -o ./output --format explainer --style whiteboard

# 生成信息图
npx @missdeer/notebooklm infographic --url "https://example.com/article" -o ./output --orientation landscape

# 生成数据表
npx @missdeer/notebooklm data-table --url "https://example.com/article" -o ./output

# 分析内容
npx @missdeer/notebooklm analyze --url "https://example.com/paper.pdf" -q "What are the key findings?"

# 与笔记本对话
npx @missdeer/notebooklm chat <notebook-id> -q "Summarize this"

# 查看笔记本详情
npx @missdeer/notebooklm detail <notebook-id>

# 删除笔记本
npx @missdeer/notebooklm delete <notebook-id>

# 向已有笔记本添加源
npx @missdeer/notebooklm source add <notebook-id> --url "https://example.com"

# 刷新 token（无需浏览器）
npx @missdeer/notebooklm refresh-session

# 系统诊断
npx @missdeer/notebooklm diagnose
```

## Transport 模式

| 模式 | 说明 | TLS 保真度 |
|---|---|---|
| `auto` (默认) | 自动选择最佳可用 transport | — |
| `http` | utls 原生 Go TLS | 99% |
| `curl` | curl-impersonate 子进程 | 100% |
| `browser` | rod 启动真实浏览器 | 100% |

```bash
npx @missdeer/notebooklm list --transport http
npx @missdeer/notebooklm audio --transport curl --url "https://example.com" -o ./output
```

### 浏览器自动探测

使用 `browser` transport 或 `export-session` 时，会按以下顺序自动探测本地浏览器：

1. Google Chrome
2. Microsoft Edge
3. Brave Browser
4. Chromium

支持 Windows、macOS、Linux。可通过 `--chrome-path` 手动指定。若均未找到，rod 将自动下载 Chromium。

## 全局选项

```
--home <dir>    配置目录 (默认: ~/.notebooklm)
```

## 代理

支持 SOCKS5 / HTTP / HTTPS 代理，所有代理类型均支持 `user:pass@` 认证。代理在所有网络路径上生效：RPC 请求（HTTP/1.1 和 HTTP/2）、文件上传、产物下载以及 session token 刷新。

### CLI 参数

| 参数 | 说明 | 示例 |
|---|---|---|
| `--socks5-proxy` | SOCKS5 代理地址（省略 scheme 时自动补 `socks5://`） | `--socks5-proxy 127.0.0.1:1080` |
| `--http-proxy` | HTTP 代理地址（省略 scheme 时自动补 `http://`） | `--http-proxy 127.0.0.1:8080` |
| `--https-proxy` | HTTPS 代理地址（省略 scheme 时自动补 `https://`） | `--https-proxy 127.0.0.1:8443` |
| `--proxy` | 通用代理 URL（需自带 scheme） | `--proxy socks5://user:pass@host:1080` |

### 解析优先级

CLI 参数优先于环境变量，依次检查：

1. `--socks5-proxy`
2. `--http-proxy`
3. `--https-proxy`
4. `--proxy`
5. 环境变量 `SOCKS5_PROXY` → `HTTP_PROXY` → `HTTPS_PROXY` → `ALL_PROXY`（大小写均可）

### 示例

```bash
# SOCKS5 with authentication
npx @missdeer/notebooklm list --socks5-proxy user:pass@127.0.0.1:1080

# HTTP proxy via environment
HTTP_PROXY=http://127.0.0.1:8080 npx @missdeer/notebooklm list
```

> **注意**：`browser` transport（rod）通过 Chrome 的 `--proxy-server` 传递代理，该参数**不支持**内嵌凭据。若需代理认证，请使用 `http`/`curl` transport。

## 环境变量

| 变量 | 说明 |
|---|---|
| `NOTEBOOKLM_HOME` | 覆盖默认配置目录 |
| `NOTEBOOKLM_AUTH_JSON` | 内联 session JSON（跳过文件加载） |
| `SOCKS5_PROXY` / `HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY` | 代理 URL（支持 socks5/http/https，可带 `user:pass@`） |

## Session 管理

Session 保存在 `~/.notebooklm/session.json`，与 TypeScript 版本格式完全兼容。

- **Token**（`at`, `bl`, `fsid`）有效期 ~1-2 小时
- **Cookie** 有效期数周/数月
- `refresh-session` 命令使用长期 cookie 刷新短期 token，无需浏览器

## 项目结构

```
├── cmd/notebooklm/        入口
├── internal/
│   ├── types/             域类型、枚举、错误
│   ├── rpc/               RPC ID、URL、路径、配置覆盖
│   ├── session/           Session 持久化 + token 刷新
│   ├── transport/         Transport 接口 + utls/curl/rod 实现
│   ├── parser/            BOQ 协议 + RPC 响应解析
│   ├── payload/           Artifact payload 构建器
│   ├── api/               无状态 RPC 函数
│   ├── download/          文件下载 + CDN 重试
│   ├── client/            NotebookClient + workflow 编排
│   ├── cli/               cobra CLI 命令
│   └── util/              jitter/sleep + refresh guard
├── go.mod
└── go.sum
```

## License

MIT
