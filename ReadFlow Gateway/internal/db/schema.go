package db

// SQL 建表语句
const Schema = `
-- 用户管理表
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    token TEXT UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_users_token ON users(token);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- 订阅源表（全局共享）
CREATE TABLE IF NOT EXISTS sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL UNIQUE,
    title TEXT,
    description TEXT,
    last_fetch_time DATETIME,
    fetch_interval INTEGER DEFAULT 900,
    is_active BOOLEAN DEFAULT 1,
    error_count INTEGER DEFAULT 0,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    -- 扩展字段
    content_type TEXT DEFAULT 'image_text',
    language TEXT DEFAULT 'en',
    category TEXT DEFAULT 'Technology',
    favicon TEXT,
    article_count INTEGER DEFAULT 0,
    update_frequency INTEGER DEFAULT 3600
);

CREATE INDEX IF NOT EXISTS idx_sources_url ON sources(url);
CREATE INDEX IF NOT EXISTS idx_sources_active_fetch ON sources(is_active, last_fetch_time);
CREATE INDEX IF NOT EXISTS idx_sources_category ON sources(category);
CREATE INDEX IF NOT EXISTS idx_sources_language ON sources(language);

-- 用户订阅关系表（用户专属配置）
CREATE TABLE IF NOT EXISTS subscriptions (
    user_id INTEGER NOT NULL,
    source_id INTEGER NOT NULL,
    subscribed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    -- 扩展字段：用户专属配置
    source_mode TEXT DEFAULT 'direct',
    sort_order INTEGER DEFAULT 0,
    group_id INTEGER,
    max_articles INTEGER DEFAULT 20,
    unread_count INTEGER DEFAULT 0,
    custom_title TEXT,
    PRIMARY KEY (user_id, source_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (source_id) REFERENCES sources(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_source ON subscriptions(source_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_sort ON subscriptions(user_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_group ON subscriptions(user_id, group_id);

-- 文章内容暂存表
CREATE TABLE IF NOT EXISTS items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id INTEGER NOT NULL,
    guid TEXT NOT NULL,
    title TEXT NOT NULL,
    xml_content TEXT NOT NULL,
    image_paths TEXT,
    published_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    -- Quest 3 字段
    summary TEXT,
    word_count INTEGER DEFAULT 0,
    reading_time INTEGER DEFAULT 0,
    cover_image TEXT,
    author TEXT,
    clean_content TEXT,
    content_hash TEXT,
    -- 新增扩展字段
    content TEXT,
    url TEXT,
    category TEXT,
    difficulty TEXT DEFAULT 'medium',
    tags TEXT,
    image_caption TEXT,
    image_credit TEXT,
    FOREIGN KEY (source_id) REFERENCES sources(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_items_source_guid ON items(source_id, guid);
CREATE INDEX IF NOT EXISTS idx_items_published ON items(published_at DESC);
CREATE INDEX IF NOT EXISTS idx_items_source_published ON items(source_id, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_items_hash ON items(content_hash);
CREATE INDEX IF NOT EXISTS idx_items_category ON items(category);
CREATE INDEX IF NOT EXISTS idx_items_difficulty ON items(difficulty);
CREATE INDEX IF NOT EXISTS idx_items_url ON items(url);

-- 用户投递状态表
CREATE TABLE IF NOT EXISTS user_deliveries (
    user_id INTEGER NOT NULL,
    item_id INTEGER NOT NULL,
    status INTEGER DEFAULT 0,
    delivered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    -- Quest 5: 阅读状态字段
    is_favorite BOOLEAN DEFAULT 0,
    read_progress INTEGER DEFAULT 0,
    read_at DATETIME,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    -- 新增扩展字段
    is_read BOOLEAN DEFAULT 0,
    scroll_position INTEGER DEFAULT 0,
    user_tags TEXT,
    reading_time_spent INTEGER DEFAULT 0,
    PRIMARY KEY (user_id, item_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES items(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_deliveries_user_status ON user_deliveries(user_id, status);
CREATE INDEX IF NOT EXISTS idx_deliveries_item ON user_deliveries(item_id);
CREATE INDEX IF NOT EXISTS idx_deliveries_user_status_item ON user_deliveries(user_id, status, item_id);
CREATE INDEX IF NOT EXISTS idx_deliveries_favorite ON user_deliveries(user_id, is_favorite);
CREATE INDEX IF NOT EXISTS idx_deliveries_updated ON user_deliveries(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_deliveries_user_read ON user_deliveries(user_id, is_read);

-- 生词本表（完整版，已包含所有字段）
CREATE TABLE IF NOT EXISTS vocabularies (
    -- 主键和基础信息
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    word TEXT NOT NULL,
    definition TEXT,
    translation TEXT,
    example TEXT,
    context TEXT,
    
    -- 来源信息
    source_article_id TEXT,
    source_article_title TEXT,
    article_id INTEGER,
    
    -- 学习进度
    review_count INTEGER DEFAULT 0,
    correct_count INTEGER DEFAULT 0,
    last_review_at INTEGER,
    next_review_at INTEGER,
    mastery_level INTEGER DEFAULT 0,
    
    -- 分类和备注
    difficulty TEXT DEFAULT 'medium',
    tags TEXT,
    notes TEXT DEFAULT '',
    
    -- 系统时间戳
    added_at INTEGER NOT NULL,
    created_at INTEGER DEFAULT (cast(strftime('%s', 'now') as integer)),
    updated_at INTEGER DEFAULT (cast(strftime('%s', 'now') as integer)),
    is_deleted BOOLEAN DEFAULT 0,
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE (user_id, word)
);

CREATE INDEX IF NOT EXISTS idx_vocabularies_user ON vocabularies(user_id);
CREATE INDEX IF NOT EXISTS idx_vocabularies_user_word ON vocabularies(user_id, word);
CREATE INDEX IF NOT EXISTS idx_vocabularies_updated ON vocabularies(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_vocabularies_next_review ON vocabularies(user_id, next_review_at);
CREATE INDEX IF NOT EXISTS idx_vocabularies_deleted ON vocabularies(is_deleted);
CREATE INDEX IF NOT EXISTS idx_vocabularies_mastery ON vocabularies(user_id, mastery_level);

-- RSS分组表
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

CREATE INDEX IF NOT EXISTS idx_rss_groups_user_id ON rss_groups(user_id);
CREATE INDEX IF NOT EXISTS idx_rss_groups_sort_order ON rss_groups(user_id, sort_order);

-- 过滤规则表
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

CREATE INDEX IF NOT EXISTS idx_filter_rules_user_id ON filter_rules(user_id);
CREATE INDEX IF NOT EXISTS idx_filter_rules_scope ON filter_rules(user_id, scope);

-- 过滤规则绑定表
CREATE TABLE IF NOT EXISTS filter_bindings (
    rule_id INTEGER,
    user_id INTEGER NOT NULL,
    source_id INTEGER NOT NULL,
    PRIMARY KEY (rule_id, user_id, source_id),
    FOREIGN KEY (rule_id) REFERENCES filter_rules(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, source_id) REFERENCES subscriptions(user_id, source_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_filter_bindings_rule_id ON filter_bindings(rule_id);
CREATE INDEX IF NOT EXISTS idx_filter_bindings_user_source ON filter_bindings(user_id, source_id);

-- 用户设置表
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
`
