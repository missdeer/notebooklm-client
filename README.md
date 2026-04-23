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

# 方式三：从已登录的 Firefox/Safari 或 Netscape cookies.txt 引导 session（无需启动浏览器）
npx @missdeer/notebooklm import-cookies                             # 自动扫描所有 Firefox profile + macOS Safari
npx @missdeer/notebooklm import-cookies --browser firefox           # 只扫 Firefox 的全部 profile
npx @missdeer/notebooklm import-cookies --browser firefox --profile ~/.mozilla/firefox/abc.default-release
npx @missdeer/notebooklm import-cookies --browser safari            # 仅 macOS
npx @missdeer/notebooklm import-cookies --file ~/cookies.txt        # 任何 Netscape cookies.txt 导出
```

> **关于 `import-cookies`**：只读取 `google.com` / `youtube.com` / `googleusercontent.com` 域的 cookie，拿到后立即用它们向 NotebookLM 换取 `at`/`bl`/`fsid`，最后写入 `session.json`。全流程**不启动浏览器**，适合服务器 / CI / 无图形环境。
>
> - **不带任何参数**：扫描默认路径下所有 Firefox profile（Windows `%APPDATA%\Mozilla\Firefox\Profiles`、macOS `~/Library/Application Support/Firefox/Profiles`、Linux `~/.mozilla/firefox` 及 XDG / snap / flatpak 变体）以及 macOS 上的 Safari，合并所有 profile 的 cookie。
> - **同一个 cookie 在多个 profile 中出现时**：按 Firefox `lastAccessed` / Safari `creation_date` 取**最新**一份，确保拿到的是你当前实际使用的账号。命令末尾会打印每个来源贡献了几个 cookie。
> - **Firefox**：直接读 `cookies.sqlite`（自动复制以避开文件锁），兼容 schema v16+ 的毫秒级过期时间。
> - **Safari**：直接解析 `Cookies.binarycookies`（仅 macOS）。
> - **Chrome/Edge/Brave**：**不直接支持**——Chrome 127+ 的 App-Bound Encryption 需要 Chrome 自己解密。请先用浏览器扩展（如「Get cookies.txt LOCALLY」）或 `yt-dlp --cookies-from-browser chrome --cookies cookies.txt` 导出成 Netscape 格式，再走 `--file` 路径。

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

## 浏览器依赖

并非所有命令都需要启动浏览器。多数日常操作仅用 HTTP 调用，速度更快、资源更省。

### 必须启动浏览器

| 命令 | 说明 |
|---|---|
| `export-session` | 首次登录 Google 账号、导出 session，强制使用 `browser` transport |

### 完全不需要浏览器（纯 HTTP）

| 命令 | 说明 |
|---|---|
| `import-session` | 从 JSON 文件/字符串导入 session，纯文件 I/O |
| `import-cookies` | 从 Firefox/Safari profile 或 Netscape cookies.txt 引导 session；读 cookie + 一次 HTTP GET |
| `refresh-session` | 用长期 cookie 刷新短期 token，仅发一次 GET 请求 |
| `session-status`  | 展示 session.json 中 cookie 的过期时间 |

> 只要 `~/.notebooklm/session.json` 中的长期 cookie 未过期，`refresh-session` 就可以在无浏览器环境（如服务器、CI）中续期 `at`/`bl`/`fsid`。若 cookie 也失效，才需要重新跑 `export-session`（或 `import-cookies`）。

### 可选（取决于 `--transport`）

以下命令默认使用 `auto`（会优先选非浏览器的 transport），也可显式指定 `--transport browser` 以获得 100% TLS 指纹保真：

`list` · `audio` · `report` · `slides` · `quiz` · `flashcards` · `video` · `infographic` · `data-table` · `analyze` · `chat` · `detail` · `delete` · `source add` · `diagnose`

```bash
# 默认路径（不启动浏览器）
npx @missdeer/notebooklm list

# 显式强制用浏览器（需要本机有 Chrome/Edge/Brave/Chromium，或让 rod 自动下载 Chromium)
npx @missdeer/notebooklm list --transport browser
```

## Transport 模式

| 模式 | 说明 | TLS 保真度 | 需要浏览器 |
|---|---|---|---|
| `auto` (默认) | 自动选择最佳可用 transport（优先非浏览器） | — | 否* |
| `http` | utls 原生 Go TLS | 99% | 否 |
| `curl` | curl-impersonate 子进程 | 100% | 否（需要 curl-impersonate 二进制） |
| `browser` | rod 启动真实浏览器 | 100% | 是 |

\* `auto` 仅在其他 transport 都不可用时才会回退到 `browser`。

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
