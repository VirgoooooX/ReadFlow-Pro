-- ================================================================
-- ReadFlowGateway Complete Schema Migration
-- 完整的数据库Schema扩展脚本
-- 目标：支持完整的RSS处理、多用户、单词本等功能
-- ================================================================

-- ================================================================
-- Part 1: 扩展现有表
-- ================================================================

-- 1.1 扩展 sources 表（RSS源全局属性）
-- 添加内容类型、语言、分类等字段
ALTER TABLE sources ADD COLUMN content_type TEXT DEFAULT 'image_text';
ALTER TABLE sources ADD COLUMN language TEXT DEFAULT 'en';
ALTER TABLE sources ADD COLUMN category TEXT DEFAULT 'Technology';
ALTER TABLE sources ADD COLUMN favicon TEXT;
ALTER TABLE sources ADD COLUMN article_count INTEGER DEFAULT 0;
ALTER TABLE sources ADD COLUMN update_frequency INTEGER DEFAULT 3600;

-- 1.2 扩展 subscriptions 表（用户订阅配置）
-- 添加用户专属的订阅配置
ALTER TABLE subscriptions ADD COLUMN source_mode TEXT DEFAULT 'direct';
ALTER TABLE subscriptions ADD COLUMN sort_order INTEGER DEFAULT 0;
ALTER TABLE subscriptions ADD COLUMN group_id INTEGER;
ALTER TABLE subscriptions ADD COLUMN max_articles INTEGER DEFAULT 20;
ALTER TABLE subscriptions ADD COLUMN unread_count INTEGER DEFAULT 0;
ALTER TABLE subscriptions ADD COLUMN custom_title TEXT;

-- 1.3 扩展 items 表（文章内容）
-- 添加完整HTML内容、URL、元数据等
ALTER TABLE items ADD COLUMN content TEXT;
ALTER TABLE items ADD COLUMN url TEXT;
ALTER TABLE items ADD COLUMN category TEXT;
ALTER TABLE items ADD COLUMN difficulty TEXT DEFAULT 'medium';
ALTER TABLE items ADD COLUMN tags TEXT;
ALTER TABLE items ADD COLUMN image_caption TEXT;
ALTER TABLE items ADD COLUMN image_credit TEXT;

-- 1.4 扩展 user_deliveries 表（用户文章状态）
-- 添加已读标记、滚动位置等
ALTER TABLE user_deliveries ADD COLUMN is_read BOOLEAN DEFAULT 0;
ALTER TABLE user_deliveries ADD COLUMN scroll_position INTEGER DEFAULT 0;
ALTER TABLE user_deliveries ADD COLUMN user_tags TEXT;
ALTER TABLE user_deliveries ADD COLUMN reading_time_spent INTEGER DEFAULT 0;

-- ================================================================
-- Part 2: 创建新表
-- ================================================================

-- 2.1 RSS分组表（支持多用户）
CREATE TABLE IF NOT EXISTS rss_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    icon TEXT,
    color TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 2.2 过滤规则表（支持多用户）
CREATE TABLE IF NOT EXISTS filter_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    keyword TEXT NOT NULL,
    is_regex INTEGER DEFAULT 0,
    mode TEXT DEFAULT 'exclude',
    scope TEXT DEFAULT 'specific',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 2.3 过滤规则绑定表
CREATE TABLE IF NOT EXISTS filter_bindings (
    rule_id INTEGER,
    user_id INTEGER NOT NULL,
    source_id INTEGER NOT NULL,
    PRIMARY KEY (rule_id, user_id, source_id),
    FOREIGN KEY (rule_id) REFERENCES filter_rules(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, source_id) REFERENCES subscriptions(user_id, source_id) ON DELETE CASCADE
);

-- 2.4 用户设置表
CREATE TABLE IF NOT EXISTS user_preferences (
    user_id INTEGER PRIMARY KEY,
    reading_settings TEXT NOT NULL DEFAULT '{}',
    translation_provider TEXT DEFAULT 'google',
    enable_auto_translation INTEGER DEFAULT 0,
    enable_title_translation INTEGER DEFAULT 1,
    max_concurrent_translations INTEGER DEFAULT 5,
    translation_timeout INTEGER DEFAULT 5000,
    default_category TEXT DEFAULT 'technology',
    enable_notifications INTEGER DEFAULT 1,
    proxy_mode_enabled INTEGER DEFAULT 0,
    proxy_server_url TEXT,
    proxy_token TEXT,
    created_at INTEGER DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ================================================================
-- Part 3: 创建索引
-- ================================================================

-- 3.1 sources 表索引
CREATE INDEX IF NOT EXISTS idx_sources_category ON sources(category);
CREATE INDEX IF NOT EXISTS idx_sources_language ON sources(language);
CREATE INDEX IF NOT EXISTS idx_sources_content_type ON sources(content_type);

-- 3.2 subscriptions 表索引
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_sort ON subscriptions(user_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_group ON subscriptions(user_id, group_id);

-- 3.3 items 表索引
CREATE INDEX IF NOT EXISTS idx_items_category ON items(category);
CREATE INDEX IF NOT EXISTS idx_items_difficulty ON items(difficulty);
CREATE INDEX IF NOT EXISTS idx_items_url ON items(url);

-- 3.4 user_deliveries 表索引
CREATE INDEX IF NOT EXISTS idx_deliveries_user_read ON user_deliveries(user_id, is_read);
CREATE INDEX IF NOT EXISTS idx_deliveries_scroll ON user_deliveries(user_id, scroll_position);

-- 3.5 rss_groups 表索引
CREATE INDEX IF NOT EXISTS idx_rss_groups_user_id ON rss_groups(user_id);
CREATE INDEX IF NOT EXISTS idx_rss_groups_sort_order ON rss_groups(user_id, sort_order);

-- 3.6 filter_rules 表索引
CREATE INDEX IF NOT EXISTS idx_filter_rules_user_id ON filter_rules(user_id);
CREATE INDEX IF NOT EXISTS idx_filter_rules_scope ON filter_rules(user_id, scope);

-- 3.7 filter_bindings 表索引
CREATE INDEX IF NOT EXISTS idx_filter_bindings_rule_id ON filter_bindings(rule_id);
CREATE INDEX IF NOT EXISTS idx_filter_bindings_user_source ON filter_bindings(user_id, source_id);

-- ================================================================
-- Part 4: 数据验证和初始化
-- ================================================================

-- 验证 vocabularies 表是否存在且完整（应该已经存在）
-- 如果不存在，使用现有的 schema.go 中的定义

-- ================================================================
-- 迁移完成
-- ================================================================
