# 📝 ReadFlow Gateway - 更新日志

## [未发布]

### ✨ 新增功能

#### 系统设置管理 (Configuration Management)
- ✅ **运行时动态配置** - 无需重启服务即可修改系统参数
- ✅ **Web 设置页面** - 专门的设置标签页，支持表单操作
- ✅ **6 个可配置参数**
  - RSS 抓取间隔 (60-86,400秒)
  - 每次抓取最多保留文章数 (10-5,000篇)
  - 图片压缩品质 (10-100%)
  - 图片最大宽度 (300-4,000px)
  - 图片处理并发数 (1-10个)
  - 日志级别 (debug/info/warn/error)

- ✅ **参数验证和范围检查** - 自动矫正超出范围的值
- ✅ **线程安全** - 使用 sync.RWMutex 保证并发访问安全
- ✅ **REST API** 
  - `GET /api/admin/config` - 获取当前配置
  - `POST /api/admin/config` - 更新配置

### 📝 新增文件

1. **`internal/config/runtime.go`** (234行)
   - RuntimeConfig 结构体
   - 单例模式实现
   - 完整的 Getter/Setter 方法
   - 批量更新方法

2. **`SETTINGS_GUIDE.md`** (374行)
   - 完整的参数说明
   - 调优建议
   - 4 个预设配置场景
   - 故障排查指南

3. **`SETTINGS_IMPLEMENTATION.md`** (521行)
   - 实现细节文档
   - API 完整文档
   - 代码结构说明
   - 测试场景

4. **`QUICK_SETTINGS_REFERENCE.md`** (185行)
   - 快速参考卡片
   - 常见调整预设
   - FAQ

5. **`CHANGELOG.md`** (本文件)
   - 更新历史记录

### 🔧 修改文件

1. **`internal/api/admin.go`**
   - 添加 config 包导入
   - 新增 `GetConfig()` 方法 (50行)
   - 新增 `UpdateConfig()` 方法 (37行)

2. **`internal/api/admin.html`**
   - 添加设置标签页导航按钮
   - 新增设置标签页内容区域
   - JavaScript 配置加载和保存函数 (144行)
   - CSS 样式表扩展 (113行)
   - 总计：+265行

3. **`cmd/server/main.go`**
   - 修复 SyncHandler 初始化（移除 cfg 参数）
   - 修复 AckHandler 初始化（添加 staticDir 参数）
   - 添加配置管理路由注册 (3行)

### 🐛 Bug 修复

- 修复了 admin.go 中缺失的 config 包导入

### 💬 提交信息模板

```
feat: 添加系统设置管理功能

- 实现运行时动态配置管理
- 添加 6 个可配置系统参数
- 提供 Web 设置界面
- 支持 REST API 访问
- 完整的参数验证和范围检查
- 线程安全的并发访问

新增文件：
- internal/config/runtime.go
- SETTINGS_GUIDE.md
- SETTINGS_IMPLEMENTATION.md
- QUICK_SETTINGS_REFERENCE.md

修改文件：
- internal/api/admin.go (+87行)
- internal/api/admin.html (+265行)
- cmd/server/main.go (+3行)

相关 Issue: 
Feature Request: Web 管理界面设置功能
```

---

## [v1.0.0] - 2024-12-19 (Admin Dashboard)

### ✨ 新增功能

#### 管理后台 (Admin Dashboard)
- ✅ **Web 管理界面** - `http://localhost:8080/admin`
- ✅ **4 个主要功能模块**
  - 👥 用户管理 - 查看所有用户的订阅统计
  - 📡 订阅源管理 - 查看源的健康度和投递情况
  - 📈 系统指标 - 实时的性能和业务指标
  - 💾 缓存管理 - 图片缓存的统计信息

- ✅ **仪表板卡片** - 6 个关键指标快速查看
  - 总用户数
  - 活跃源数
  - 总文章数
  - 总投递数
  - API 请求成功率
  - RSS 抓取成功率

- ✅ **完整的 REST API**
  - `GET /api/admin/dashboard` - 仪表板数据
  - `GET /api/admin/users` - 用户统计
  - `GET /api/admin/sources` - 源统计
  - `GET /api/admin/cache-stats` - 缓存统计
  - `GET /api/admin/metrics` - 系统指标

### 📝 新增文件

1. **`internal/api/admin.go`** (306行)
   - AdminHandler 结构体
   - 6 个主要方法
   - 8 个辅助统计方法

2. **`internal/db/admin_queries.go`** (296行)
   - 20+ 数据库查询方法
   - 用户/源/投递的聚合统计

3. **`internal/api/admin.html`** (587行)
   - 响应式 Web UI
   - 完整的 JavaScript 交互

4. **`ADMIN_GUIDE.md`** (311行)
   - 功能使用说明
   - 8 类增强指标建议

5. **`ADMIN_IMPLEMENTATION.md`** (340行)
   - 实现细节说明
   - API 完整文档

### 🔧 修改文件

1. **`cmd/server/main.go`** (+20行)
   - AdminHandler 初始化
   - 管理后台路由注册

2. **`internal/db/item_operations.go`**
   - 删除未使用的 "database/sql" 导入

3. **`Dockerfile`**
   - 调整构建顺序
   - 修改基础镜像 (alpine → debian)
   - 添加 libvips-dev 依赖

---

## [v0.1.0] - 2024-12-18 (Initial Release)

### ✨ 核心功能

#### 用户认证
- ✅ 简化版认证机制
- ✅ 全局密码 + JWT Token
- ✅ 用户自动创建/查找

#### 订阅管理
- ✅ RSS 源订阅
- ✅ 取消订阅
- ✅ 订阅列表查询

#### RSS 同步
- ✅ 文章自动抓取
- ✅ RSS XML 格式返回
- ✅ 文章去重机制

#### 投递确认
- ✅ 文章确认接收
- ✅ 自动 GC 清理

#### 生词本同步
- ✅ 生词本推送
- ✅ 生词本拉取
- ✅ 时间戳冲突解决

#### 部署支持
- ✅ Docker 容器化
- ✅ docker-compose 编排
- ✅ 一键启动脚本

### 📝 项目结构

```
ReadFlowGateway/
├── cmd/server/main.go
├── internal/
│   ├── api/
│   │   ├── auth.go
│   │   ├── subscribe.go
│   │   ├── sync.go
│   │   ├── ack.go
│   │   └── vocab.go
│   ├── config/config.go
│   ├── db/
│   │   ├── db.go
│   │   ├── schema.go
│   │   ├── operations.go
│   │   └── item_operations.go
│   └── worker/worker.go
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── Makefile
└── README.md
```

---

## 版本说明

### 版本号格式
`MAJOR.MINOR.PATCH`

- **MAJOR** - 大版本，功能大幅变化
- **MINOR** - 小版本，新增功能
- **PATCH** - 补丁版本，bug 修复

### 版本历史

| 版本 | 发布日期 | 主要内容 |
|------|---------|---------|
| v1.0.0 | 2024-12-19 | 管理后台完整实现 |
| v0.1.0 | 2024-12-18 | MVP 基础功能 |
| (upcoming) | TBD | 系统设置管理 |

---

## 后续规划

### Phase 2: 图片处理优化
- [ ] 图片质量设置优化
- [ ] 图片缓存清理
- [ ] 缓存分析工具

### Phase 3: 增强功能
- [ ] 用户权限管理
- [ ] 操作日志审计
- [ ] 数据导出功能
- [ ] 批量操作支持

### Phase 4: 性能优化
- [ ] 数据库索引优化
- [ ] 缓存策略改进
- [ ] 并发性能提升
- [ ] 监控告警集成

---

## 发布流程

### 开发流程
1. 在功能分支上开发（`feature/xxx`）
2. 完成后创建 Pull Request
3. 代码审查通过后合并到 `main`
4. 从 `main` 分支创建 release

### 发布步骤
1. 更新 CHANGELOG.md
2. 更新版本号（go.mod, Dockerfile 等）
3. 创建 Git Tag
4. 发布 Release Notes
5. 编译并发布 Docker 镜像

---

**最后更新**: 2025-12-20
**当前开发分支**: 系统设置管理功能
**下一个发布版本**: v1.1.0 (预计)
