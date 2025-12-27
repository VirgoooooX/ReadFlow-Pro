-- Quest 3: 扩展 items 表字段
-- 用于存储服务端生成的结构化数据，减少客户端解析负担

-- 添加新字段
ALTER TABLE items ADD COLUMN summary TEXT;
ALTER TABLE items ADD COLUMN word_count INTEGER DEFAULT 0;
ALTER TABLE items ADD COLUMN reading_time INTEGER DEFAULT 0;
ALTER TABLE items ADD COLUMN cover_image TEXT;
ALTER TABLE items ADD COLUMN author TEXT;
ALTER TABLE items ADD COLUMN clean_content TEXT;
ALTER TABLE items ADD COLUMN content_hash TEXT;

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_items_hash ON items(content_hash);
