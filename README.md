# TeamNav Go · 星云导航

> 参考 [tuituidan/team-nav](https://github.com/tuituidan/team-nav) 的功能设计，使用 **Go + SQLite + 原生前端** 完整重写。
> 单个二进制、零外部依赖，自带精致的管理后台与图片缓存。

[![CI](https://github.com/Songxwn/team-nav-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Songxwn/team-nav-go/actions/workflows/ci.yml)
[![Container](https://github.com/Songxwn/team-nav-go/actions/workflows/docker.yml/badge.svg)](https://github.com/Songxwn/team-nav-go/actions/workflows/docker.yml)
[![Release](https://github.com/Songxwn/team-nav-go/actions/workflows/release.yml/badge.svg)](https://github.com/Songxwn/team-nav-go/actions/workflows/release.yml)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go)](https://go.dev)
[![SQLite](https://img.shields.io/badge/SQLite-embedded-003B57?logo=sqlite)](https://sqlite.org)
[![License](https://img.shields.io/badge/license-MIT-22c55e)](LICENSE)

---

## 为什么重写

原项目基于 Spring Boot + H2 + Vue 2 / Element UI，功能完善但技术栈较重。本项目在保留「团队服务导航」核心体验的前提下做了取舍与升级：

| | team-nav (Java) | TeamNav Go |
|---|---|---|
| 运行时 | JDK + Spring Boot | **单个静态二进制**（约 15MB） |
| 数据库 | H2 / MySQL / PostgreSQL | **SQLite（WAL 模式，无需额外服务）** |
| 前端 | Vue 2 + Element UI | **原生 ES2020 + 手写 Cocoon UI 设计系统** |
| 图片 | 文件系统挂载 | **SQLite BLOB + 内存 LRU + ETag/immutable 缓存** |
| 部署 | jar + 挂载目录 | **Docker 镜像自动发布到 GHCR** |
| 构建 | Maven + Node | **GitHub Actions，仅 Go 工具链** |

## 功能

### 前台
- 三级分类导航、分类侧边栏与滚动高亮
- 全局搜索面板（`Ctrl / ⌘ + K`），支持键盘上下选择、回车打开
- 服务卡片：Emoji / 文字 / 图片 / 图片链接四种图标
- 标签、描述、访问域名、点击次数展示
- 星标收藏（保存在浏览器 `localStorage`），一键只看收藏
- 通知公告轮播、截止时间倒计时、级别配色
- 深色 / 浅色主题、五种主题色、移动端抽屉导航
- 访客提交卡片，管理员审核后上线

### 后台
- 概览：卡片 / 分类 / 待审核 / 用户 / 累计点击 / 今日访问 / 数据库大小
- 最近 7 天访问柱状图、热门服务排行、最近添加
- 分类管理：三级树、可见性、排序、级联删除保护
- 卡片管理：搜索、状态筛选、分类筛选、审核、排序、图标上传
- 公告管理：级别、启用状态、截止时间
- 用户与权限：管理员 / 普通成员、重置密码、最后一名管理员保护
- 图片库：上传、拖拽、复制地址、删除、内容去重
- 系统设置：站点名称、副标题、欢迎语、LOGO、主题色、公告间隔、页脚、登录会话有效期
- 数据备份：一键下载 SQLite 快照（图片也在库内）

## 快速开始

### Docker（推荐）

```bash
docker run -d \
  --name teamnav \
  -p 8080:8080 \
  -v "$PWD/data:/data" \
  -e TEAMNAV_ADMIN_PASSWORD="your-strong-password" \
  ghcr.io/songxwn/team-nav-go:latest
```

打开 <http://localhost:8080>，使用 `admin` 和上面设置的密码登录后台。

### Docker Compose

```bash
# 修改 docker-compose.yml 中的 TEAMNAV_ADMIN_PASSWORD
docker compose up -d
```

### 本地运行

```bash
git clone https://github.com/Songxwn/team-nav-go.git
cd team-nav-go
go run ./cmd/teamnav -data ./data
# 或
make run
```

### 从源码构建

```bash
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(git describe --tags --always)" \
  -o bin/teamnav ./cmd/teamnav
```

> 前端资源通过 `go:embed` 打进二进制，构建不需要 Node.js。

## 配置

所有配置均可通过环境变量或命令行参数传入。

| 环境变量 | 命令行 | 默认值 | 说明 |
|---|---|---|---|
| `TEAMNAV_ADDR` | `-addr` | `:8080` | 监听地址 |
| `TEAMNAV_DATA_DIR` | `-data` | `./data` | 数据目录（存放 SQLite） |
| `TEAMNAV_DB` | — | `$DATA/team-nav.db` | 直接指定数据库文件 |
| `TEAMNAV_ADMIN_USER` | — | `admin` | 首次启动创建的管理员用户名 |
| `TEAMNAV_ADMIN_PASSWORD` | — | `admin123` | 首次启动创建的管理员密码 |
| `TEAMNAV_IMAGE_CACHE_MB` | `-image-cache-mb` | `64` | 图片内存缓存预算（MiB，`0` 关闭） |
| `TEAMNAV_COOKIE_SECURE` | — | `0` | HTTPS 部署时设为 `1` |
| `TEAMNAV_TRUST_PROXY` | `-trust-proxy` | `0` | 信任 `X-Forwarded-For` / `X-Real-IP` |
| `TEAMNAV_DEV_WEB` | `-dev-web` | 空 | 从目录加载前端，便于调试 |

管理员账号只在**空数据库**首次启动时创建。如需重置密码，可删除数据卷重新初始化，或进入后台用另一名管理员重置。

### 站点设置（存于数据库，后台可改）

| 设置项 | 默认值 | 说明 |
|---|---|---|
| `siteName` | 星云导航 | 站点名称 |
| `siteSubtitle` | 团队服务，一站式入口 | 副标题 |
| `siteLogo` | 空 | LOGO 图片地址或上传的图片 |
| `welcomeText` | 今天也要高效出发 | 首页欢迎语 |
| `footerText` | Built with Go · SQLite · Cocoon UI | 页脚文字 |
| `accent` | indigo | 主题色：indigo / emerald / rose / amber / ocean |
| `noticeInterval` | 8 | 公告轮播间隔（3-120 秒） |
| `allowApply` | 1 | 是否允许访客提交卡片 |
| `showClicks` | 1 | 卡片是否显示点击次数 |
| `sessionTTL` | 66 | 登录会话有效期（1-365 天），修改后对新登录生效 |

## 图片存储与缓存

图片不落磁盘文件，而是以 BLOB 保存在 SQLite 的 `images` 表中，并以 SHA-256 去重：

```
上传 ──► 计算 SHA-256 ──► 已存在？ ── 是 ──► 直接复用
                              │
                              否
                              ▼
                      写入 images 表（BLOB）
                              │
        首次访问 ──► 读库 ──► 写入内存 LRU ──► 返回 ETag
        再次访问 ──► 命中 LRU ──► 304 / immutable 响应
```

- **内存 LRU**：按字节限制的并发安全 LRU（`internal/imgcache`）
- **HTTP 缓存**：`Cache-Control: public, max-age=31536000, immutable` + 内容 SHA 派生的 `ETag`
- **协商缓存**：`If-None-Match` 直接返回 `304 Not Modified`
- **可观测**：`GET /api/healthz` 返回缓存条目数、占用字节、命中/未命中计数

## GitHub Actions 与产物

仓库内置三条流水线：

1. **CI**（`.github/workflows/ci.yml`）：`gofmt` 检查、`go vet`、`go test -race`、静态编译
2. **Container**（`.github/workflows/docker.yml`）：多架构（amd64 / arm64）构建并推送 GHCR
3. **Release**（`.github/workflows/release.yml`）：推送 `v*` 标签时交叉编译全平台二进制并发布 GitHub Release

### 预编译二进制

`Release` 流水线为以下平台生成压缩包，并附带 `SHA256SUMS`：

| 平台 | 架构 | 文件 |
|---|---|---|
| Linux | amd64 | `teamnav_<version>_linux_amd64.tar.gz` |
| Linux | arm64 | `teamnav_<version>_linux_arm64.tar.gz` |
| macOS | amd64 | `teamnav_<version>_darwin_amd64.tar.gz` |
| macOS | arm64 | `teamnav_<version>_darwin_arm64.tar.gz` |
| Windows | amd64 | `teamnav_<version>_windows_amd64.zip` |
| Windows | arm64 | `teamnav_<version>_windows_arm64.zip` |

下载地址：<https://github.com/Songxwn/team-nav-go/releases>

```bash
# Linux / macOS
tar -xzf teamnav_<version>_linux_amd64.tar.gz
./teamnav -data ./data

# 校验
sha256sum -c SHA256SUMS      # macOS: shasum -a 256 -c SHA256SUMS
```

发布新版本：

```bash
git tag v1.0.0 && git push origin v1.0.0
```

镜像地址：

```
ghcr.io/songxwn/team-nav-go:latest
ghcr.io/songxwn/team-nav-go:sha-<commit>
ghcr.io/songxwn/team-nav-go:v1.0.0
```

镜像使用 GitHub 内置 `GITHUB_TOKEN` 推送，无需额外 Secret。仓库为公开仓库时 GHCR 包默认可匿名拉取；若改为私有，可在 **Packages** 设置中调整可见性。

## API 速览

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/bootstrap` | 首页一次性数据（设置 / 分类树 / 卡片 / 公告 / 当前用户） |
| `GET` | `/api/categories` | 可见分类树 |
| `GET` | `/api/cards?categoryId=&q=` | 卡片列表 |
| `GET` | `/api/search?q=` | 搜索卡片与分类 |
| `POST` | `/api/cards/{id}/click` | 记录点击 |
| `POST` | `/api/apply` | 提交卡片申请（限流） |
| `POST` | `/api/auth/login` | 登录（限流） |
| `POST` | `/api/auth/logout` | 退出 |
| `GET` | `/api/auth/me` | 当前用户 |
| `GET` | `/api/images/{id}` | 读取图片（LRU + ETag） |
| `GET` | `/api/healthz` | 健康检查与缓存指标 |

管理接口位于 `/api/admin/*`，需要管理员会话：

```
overview  categories  cards  notices  users  images  settings  sort  backup
```

示例：

```bash
curl -s localhost:8080/api/healthz | jq
curl -s "localhost:8080/api/search?q=监控" | jq '.cards[].name'
```

## 项目结构

```
.
├── cmd/teamnav/main.go          # 启动、配置、优雅退出
├── internal
│   ├── auth/                    # 会话 Cookie、bcrypt、登录
│   ├── imgcache/                # 字节上限 LRU
│   ├── model/                   # 实体定义
│   ├── server/                  # 路由、API、中间件、静态资源
│   ├── store/                   # SQLite schema、查询、种子数据
│   └── web/                     # go:embed 的前端（Cocoon UI）
│       ├── index.html           # 导航首页
│       ├── admin.html           # 管理后台
│       └── assets/{app,admin}.css|js
├── Dockerfile                   # 多阶段构建，非 root 运行
├── docker-compose.yml
└── .github/workflows/           # CI + GHCR 镜像 + 全平台二进制发布
```

## 安全设计

- 密码使用 **bcrypt** 哈希；登录会话默认有效期 **66 天**，可在后台「系统设置」中调整为 1-365 天（仅对之后的新登录生效）
- 会话 Token 仅以 **SHA-256 摘要**存入数据库，Cookie 为 `HttpOnly` + `SameSite=Lax`
- 登录、卡片申请按 IP **令牌桶限流**
- 所有写操作校验 `Origin`，防范跨站请求
- CSP、`X-Content-Type-Options`、`X-Frame-Options`、`Referrer-Policy`
- 图片上传校验真实 MIME、限制 2MB，仅允许常见位图格式
- 前端所有动态内容转义渲染，避免存储型 XSS
- 容器以 UID 10001 非 root 运行

## 设计说明

前端没有引入 UI 框架，而是手写了以 **Cocoon UI** 命名的设计系统：

- 玻璃拟态面板 + 极光渐变背景，深浅色共用一套 CSS 变量
- 卡片图标根据名称哈希生成稳定的渐变色调，避免视觉重复
- 侧边栏滚动高亮、命令面板、抽屉导航、Toast 均为原生实现
- 移动端优先的断点与 `prefers-reduced-motion` 支持
- 全站无第三方 CDN 请求，离线可用，首屏只加载约 40KB CSS/JS

## 致谢

- 功能与交互灵感来自 [tuituidan/team-nav](https://github.com/tuituidan/team-nav)（MIT）
- SQLite 驱动：[modernc.org/sqlite](https://gitlab.com/cznic/sqlite)（纯 Go，无 CGO）
- 图标：手写 SVG

## License

[MIT](LICENSE) © 2026 Songxwn