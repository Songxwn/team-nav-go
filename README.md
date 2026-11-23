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
- **简体中文 / English 双语界面**，右上角一键切换并记住选择
- 访客提交卡片，管理员审核后上线

### 后台
- 概览：卡片 / 分类 / 待审核 / 用户 / 累计点击 / 今日访问 / 数据库大小
- 最近 7 天访问柱状图、热门服务排行、最近添加
- 分类管理：三级树、可见性、排序、级联删除保护
- 卡片管理：搜索、状态筛选、分类筛选、审核、排序、图标上传
- 公告管理：级别、启用状态、截止时间
- 用户与权限：管理员 / 普通成员、重置密码、最后一名管理员保护
- 认证接入：**LDAP / AD** 目录登录与 **OIDC 单点登录（SSO）**，支持组映射管理员、自动开通账号
- 图片库：上传、拖拽、复制地址、删除、内容去重
- 系统设置：站点名称、副标题、欢迎语、界面语言、LOGO、主题色、公告间隔、页脚、登录会话有效期
- 数据备份：一键下载 SQLite 快照（图片也在库内）
- 关于卡片内含 GitHub 仓库入口（首页不再展示 GitHub 链接）

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
| `language` | zh | 默认界面语言：`zh` / `en`；访客可在右上角切换，个人选择优先 |
| `footerText` | Built with Go · SQLite · Cocoon UI | 页脚文字 |
| `accent` | indigo | 主题色：indigo / emerald / rose / amber / ocean |
| `noticeInterval` | 8 | 公告轮播间隔（3-120 秒） |
| `allowApply` | 1 | 是否允许访客提交卡片 |
| `showClicks` | 1 | 卡片是否显示点击次数 |
| `sessionTTL` | 66 | 登录会话有效期（1-365 天），修改后对新登录生效 |

## 界面语言

界面支持 **简体中文** 与 **English** 两种语言：

- 首页与后台右上角均有语言按钮，切换后立即重绘，并写入 `localStorage`（`tn-lang`）与 `tn_lang` Cookie
- 也可以直接用 `/?lang=en` 或 `/admin?lang=en` 打开英文界面，便于分享
- 未手动切换过的访客使用后台「系统设置 → 界面语言」的默认值
- 服务端错误消息同样跟随语言：优先 `?lang=en`，其次 `tn_lang` Cookie，最后 `Accept-Language`
- 站点名称、分类、卡片、公告等内容属于管理员录入的数据，不会被自动翻译

翻译源文件：

| 位置 | 作用 |
|---|---|
| `internal/i18n/i18n.go` | 服务端错误消息的英文对照表 |
| `internal/web/assets/i18n.js` | 前端静态文案与动态消息的英文对照表 |

新增文案时，以中文原文为键补上英文值即可；缺少翻译时自动回退中文，不会报错。

## 企业认证（AD / LDAP 与 SSO）

后台「认证接入」页面可以启用两类企业登录方式，配置保存在 SQLite，无需改环境变量。

### LDAP / Active Directory

支持两种绑定方式，任选其一：

| 方式 | 必填项 | 适用场景 |
|---|---|---|
| 服务账号搜索 | 目录地址、服务账号 DN / 密码、Base DN、用户过滤器 | AD 域、OpenLDAP，按 `sAMAccountName` 或 `uid` 搜索 |
| 直接 DN 绑定 | 目录地址、用户 DN 模板 | AD 的 `{username}@corp.com`，或固定 OU 的 `uid={username},ou=People,dc=corp,dc=com` |

补充说明：

- 地址支持 `ldap://host:389`、`ldaps://host:636`、`host:636`；`ldap://` 可再勾选 StartTLS
- `用户过滤器` 使用 `{username}` 占位符，AD 常用 `(sAMAccountName={username})`，OpenLDAP 常用 `(uid={username})`
- `用户名属性` 默认 `uid`，AD 建议 `sAMAccountName`；显示名默认 `displayName`，回退 `cn`
- 填写「管理员组 DN」后，用户 `memberOf` 命中该组即授予管理员；留空则沿用账号现有角色
- 支持「自动创建用户」；关闭后首次登录会被拒绝，此前已开通的账号仍可登录
- 空密码会被直接拒绝，避免 LDAP 匿名绑定；服务账号密码与 SSO Client Secret 在接口中始终以 `********` 返回

### OIDC 单点登录

适用于 Entra ID（Azure AD）、Keycloak、Okta、Authentik、GitLab 等标准 OIDC 提供方：

1. 在身份提供方创建 Web 应用，回调地址填 `https://<你的域名>/api/auth/oidc/callback`
2. 在「认证接入 → SSO / OIDC」填写 Issuer、Client ID、Client Secret
3. 按需调整用户名 / 显示名 / 用户组 Claim，以及管理员组名称
4. 保存并勾选「启用 SSO 登录」，首页登录框会出现统一登录按钮

实现细节：

- 授权码流程 + `state` + `nonce` + PKCE（S256），`state` 一次性存储于 SQLite，10 分钟过期
- ID Token 通过提供方 JWKS 校验签名、issuer、audience 与 nonce
- `oidcRedirectURL` 留空时按当前请求域名自动推导；反向代理后建议显式填写，并设置 `TEAMNAV_TRUST_PROXY=1`
- 用户以 `(issuer, sub)` 作为稳定标识，用户名变更不会重复建号；显示名与组角色每次登录同步
- 组角色同步不会移除最后一名管理员

### 账号来源与密码

- 本地账号（`source=local`）继续使用用户名密码登录，可在后台重置密码
- LDAP / SSO 账号首次登录自动创建（可关闭），本地密码为随机值，不能用于密码登录
- 同名本地账号优先：如果本地已存在同名账号，目录/SSO 登录会以“账号名称已被占用”拒绝，避免目录账号覆盖本地管理员
- 外部账号的密码修改需在身份提供方完成，后台会隐藏相应入口

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
│   ├── auth/                    # 会话 Cookie、bcrypt、LDAP / OIDC 认证
│   ├── i18n/                    # 服务端消息的中英对照
│   ├── imgcache/                # 字节上限 LRU
│   ├── model/                   # 实体定义
│   ├── server/                  # 路由、API、中间件、OIDC 回调、静态资源
│   ├── store/                   # SQLite schema、查询、种子数据
│   └── web/                     # go:embed 的前端（Cocoon UI）
│       ├── index.html           # 导航首页
│       ├── admin.html           # 管理后台
│       └── assets/{app,admin}.css|js, i18n.js
├── Dockerfile                   # 多阶段构建，非 root 运行
├── docker-compose.yml
└── .github/workflows/           # CI + GHCR 镜像 + 全平台二进制发布
```

## 安全设计

- 密码使用 **bcrypt** 哈希；登录会话默认有效期 **66 天**，可在后台「系统设置」中调整为 1-365 天（仅对之后的新登录生效）
- LDAP/AD 与 OIDC 账号的本地密码为随机值；密码登录只接受本地账号，目录服务不可用时不会回退为弱校验
- OIDC 使用一次性 `state`、`nonce` 与 PKCE，服务端校验 ID Token 签名、issuer、audience；LDAP 拒绝空密码匿名绑定
- 目录/SSO 配置与 Secret 仅管理员可读，接口返回时以 `********` 脱敏，公开 bootstrap 只暴露站点展示项
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