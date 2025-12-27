-- Quest 5: 扩展 user_deliveries 表，添加阅读状态管理字段
-- 执行时间：手动执行（在数据库升级时）

-- 添加收藏标记字段
ALTER TABLE user_deliveries ADD COLUMN is_favorite BOOLEAN DEFAULT 0;

-- 添加阅读进度字段（0-100）
ALTER TABLE user_deliveries ADD COLUMN read_progress INTEGER DEFAULT 0;

-- 添加首次标记已读时间字段
ALTER TABLE user_deliveries ADD COLUMN read_at DATETIME;

-- 添加状态最后更新时间字段
ALTER TABLE user_deliveries ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP;

-- 创建收藏索引（用于"我的收藏"列表查询）
CREATE INDEX IF NOT EXISTS idx_deliveries_favorite ON user_deliveries(user_id, is_favorite);

-- 创建更新时间索引（用于增量同步）
CREATE INDEX IF NOT EXISTS idx_deliveries_updated ON user_deliveries(updated_at DESC);

-- 字段说明：
-- status: 投递状态（0=未读，1=已投递，2=已读）
-- is_favorite: 收藏标记（独立于 status）
-- read_progress: 阅读进度百分比（0-100）
-- read_at: 首次标记已读的时间
-- updated_at: 任何状态变更时更新
