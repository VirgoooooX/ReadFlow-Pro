# 架构重构计划：服务端 RSS 处理与客户端适配

## 1. 服务端改进 (`ReadFlow Gateway`)

### A. 图片处理与压缩
*   **目标**: 自动化抓取并处理图片，根据用户选择返回不同版本的图片。
*   **行动**:
    1.  **启用图片处理**: 修复并启用 `internal/worker/worker.go` 中的图片处理逻辑（目前被注释掉的部分）。
    2.  **存储策略**: 服务端统一将图片下载并压缩为 WebP 格式存储。
    3.  **API 更新**:
        *   修改 `Sync` 接口 (`internal/api/sync.go`)，支持 `image_compression` 参数。
        *   **逻辑实现**:
            *   **开启压缩**: 返回指向服务端本地静态文件的 URL（如 `http://server/static/images/...`），节省流量。
            *   **关闭压缩**: 返回代理 URL（如 `http://server/api/proxy/image?url=ORIGINAL_URL`），由服务端中转原图，解决防盗链问题但不压缩画质。

### B. 自动化抓取
*   **目标**: 服务端完全自动定期抓取。
*   **行动**:
    *   确认 `Worker` 的定时抓取机制（Ticker）正常工作，无需客户端触发即可在后台运行。

### C. 增量同步
*   **目标**: 用户开启 App 后增量同步。
*   **行动**:
    *   利用现有的 `Sync` 接口（基于 `user_deliveries` 表的 `status` 字段），客户端只需请求 `/api/sync?mode=sync`，服务端即返回自上次同步以来新产生的文章。

## 2. 客户端适配 (`ReadFlow Pro`)

### A. 移除客户端 RSS 处理
*   **目标**: 去除客户端处理 RSS 的功能，仅保留渲染。
*   **行动**:
    1.  **删除**: `src/services/rss/LocalRSSService.ts` 及相关解析逻辑。
    2.  **清理**: 移除 `linkedom`, `@mozilla/readability` 等仅用于本地解析的依赖。
    3.  **重构 `RSSService.ts`**:
        *   废弃“直连模式” (Direct Mode)。
        *   统一所有数据获取逻辑调用服务端的 `/api/sync` 接口。
        *   适配服务端返回的 JSON 数据结构，转换为客户端的 `Article` 对象。

### B. 保留与新增功能
*   **保留功能**:
        *   **生词本 (Vocabulary)**: 保留查词、翻译、复习等所有功能。
        *   **阅读设置**: 保留字体、背景色等偏好设置。
        *   **JSON 渲染**: 保留 `ArticleDetailScreen`，直接渲染服务端处理好的 HTML 内容。
*   **新增设置**:
    *   在设置页面（如 `RSSStartupSettingsScreen` 或 `ReadingSettingsScreen`）增加 **"开启图片压缩"** 开关。
    *   同步时将此开关状态作为参数传递给服务端。

## 3. 执行步骤
1.  **服务端**: 修复 `worker.go` 的图片处理代码，确保自动抓取时生成压缩图。
2.  **服务端**: 改造 `Sync` 接口，根据 `image_compression` 参数动态返回图片 URL。
3.  **客户端**: 移除 `LocalRSSService`，重写 `RSSService` 对接服务端。
4.  **客户端**: 添加图片压缩设置 UI 并联调。
