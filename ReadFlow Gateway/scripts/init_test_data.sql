-- 初始化测试用户
INSERT OR IGNORE INTO users (id, username, token, created_at, last_login_at) VALUES
(1, 'testuser1', 'test_token_1', datetime('now'), datetime('now')),
(2, 'testuser2', 'test_token_2', datetime('now'), datetime('now')),
(3, 'admin', 'admin_token', datetime('now'), datetime('now'));

-- 初始化订阅关系（用户订阅所有活跃的订阅源）
INSERT OR IGNORE INTO subscriptions (user_id, source_id, subscribed_at)
SELECT u.id, s.id, datetime('now')
FROM users u
CROSS JOIN sources s
WHERE s.is_active = 1
AND NOT EXISTS (
    SELECT 1 FROM subscriptions
    WHERE user_id = u.id AND source_id = s.id
);
