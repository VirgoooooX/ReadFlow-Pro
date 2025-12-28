package db

import (
	"database/sql"
	"fmt"
	"time"
)

// User 相关操作

// CreateUser 创建新用户
func (db *DB) CreateUser(username, email, passwordHash string) (*User, error) {
	// 检查用户是否存在
	_, err := db.GetUserByUsername(username)
	if err == nil {
		return nil, fmt.Errorf("username already exists")
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// 检查邮箱是否存在 (如果提供了邮箱)
	if email != "" {
		_, err := db.GetUserByEmail(email)
		if err == nil {
			return nil, fmt.Errorf("email already exists")
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	result, err := db.Exec(
		"INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)",
		username, email, passwordHash,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return db.GetUserByID(id)
}

// CreateOrGetUser 创建或获取用户 (Legacy support, no password)
func (db *DB) CreateOrGetUser(username string) (*User, error) {
	// 尝试直接插入，如果已存在则忽略 (原子操作更安全)
	_, err := db.Exec(
		"INSERT OR IGNORE INTO users (username, created_at, last_login_at) VALUES (?, ?, ?)",
		username, time.Now(), time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure user exists: %w", err)
	}

	return db.GetUserByUsername(username)
}

// GetUserByID 根据 ID 获取用户
func (db *DB) GetUserByID(id int64) (*User, error) {
	user := &User{}
	err := db.QueryRow(
		"SELECT id, username, COALESCE(email, ''), COALESCE(password_hash, ''), COALESCE(token, ''), created_at, last_login_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Token, &user.CreatedAt, &user.LastLoginAt)

	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByUsername 根据用户名获取用户
func (db *DB) GetUserByUsername(username string) (*User, error) {
	user := &User{}
	err := db.QueryRow(
		"SELECT id, username, COALESCE(email, ''), COALESCE(password_hash, ''), COALESCE(token, ''), created_at, last_login_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Token, &user.CreatedAt, &user.LastLoginAt)

	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByEmail 根据邮箱获取用户
func (db *DB) GetUserByEmail(email string) (*User, error) {
	user := &User{}
	err := db.QueryRow(
		"SELECT id, username, COALESCE(email, ''), COALESCE(password_hash, ''), COALESCE(token, ''), created_at, last_login_at FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Token, &user.CreatedAt, &user.LastLoginAt)

	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByToken 根据 Token 获取用户
func (db *DB) GetUserByToken(token string) (*User, error) {
	user := &User{}
	err := db.QueryRow(
		"SELECT id, username, COALESCE(email, ''), COALESCE(password_hash, ''), COALESCE(token, ''), created_at, last_login_at FROM users WHERE token = ?",
		token,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Token, &user.CreatedAt, &user.LastLoginAt)

	if err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateUserToken 更新用户 Token
func (db *DB) UpdateUserToken(userID int64, token string) error {
	_, err := db.Exec(
		"UPDATE users SET token = ?, last_login_at = ? WHERE id = ?",
		token, time.Now(), userID,
	)
	return err
}

// DeleteUser 删除用户
func (db *DB) DeleteUser(userID int64) error {
	_, err := db.Exec("DELETE FROM users WHERE id = ?", userID)
	return err
}

// UpsertUserPreferences 更新或插入用户偏好设置
func (db *DB) UpsertUserPreferences(pref *UserPreference) error {
	_, err := db.Exec(`
		INSERT INTO user_preferences (
			user_id, reading_settings, translation_provider, 
			enable_auto_translation, enable_title_translation, 
			max_concurrent_translations, translation_timeout,
			default_category, enable_notifications,
			proxy_mode_enabled, proxy_server_url, proxy_token,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			reading_settings = excluded.reading_settings,
			translation_provider = excluded.translation_provider,
			enable_auto_translation = excluded.enable_auto_translation,
			enable_title_translation = excluded.enable_title_translation,
			max_concurrent_translations = excluded.max_concurrent_translations,
			translation_timeout = excluded.translation_timeout,
			default_category = excluded.default_category,
			enable_notifications = excluded.enable_notifications,
			proxy_mode_enabled = excluded.proxy_mode_enabled,
			proxy_server_url = excluded.proxy_server_url,
			proxy_token = excluded.proxy_token,
			updated_at = excluded.updated_at
	`,
		pref.UserID, pref.ReadingSettings, pref.TranslationProvider,
		pref.EnableAutoTranslation, pref.EnableTitleTranslation,
		pref.MaxConcurrentTranslations, pref.TranslationTimeout,
		pref.DefaultCategory, pref.EnableNotifications,
		pref.ProxyModeEnabled, pref.ProxyServerURL, pref.ProxyToken,
		time.Now().Unix(),
	)
	return err
}

// GetUserPreferences 获取用户偏好设置
func (db *DB) GetUserPreferences(userID int64) (*UserPreference, error) {
	pref := &UserPreference{}
	err := db.QueryRow(`
		SELECT user_id, reading_settings, translation_provider, 
		       enable_auto_translation, enable_title_translation, 
		       max_concurrent_translations, translation_timeout,
		       default_category, enable_notifications,
		       proxy_mode_enabled, proxy_server_url, proxy_token,
		       created_at, updated_at
		FROM user_preferences WHERE user_id = ?
	`, userID).Scan(
		&pref.UserID, &pref.ReadingSettings, &pref.TranslationProvider,
		&pref.EnableAutoTranslation, &pref.EnableTitleTranslation,
		&pref.MaxConcurrentTranslations, &pref.TranslationTimeout,
		&pref.DefaultCategory, &pref.EnableNotifications,
		&pref.ProxyModeEnabled, &pref.ProxyServerURL, &pref.ProxyToken,
		&pref.CreatedAt, &pref.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return pref, nil
}

// Source 相关操作

// CreateSource 创建订阅源
func (db *DB) CreateSource(url, title, description string) (*Source, error) {
	result, err := db.Exec(
		"INSERT INTO sources (url, title, description) VALUES (?, ?, ?)",
		url, title, description,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create source: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return db.GetSourceByID(id)
}

// GetSourceByID 根据 ID 获取订阅源
func (db *DB) GetSourceByID(id int64) (*Source, error) {
	source := &Source{}
	err := db.QueryRow(`
		SELECT id, url, COALESCE(title, ''), COALESCE(description, ''), 
		       last_fetch_time, fetch_interval, is_active, error_count, 
		       COALESCE(last_error, ''), created_at 
		FROM sources WHERE id = ?`,
		id,
	).Scan(
		&source.ID, &source.URL, &source.Title, &source.Description,
		&source.LastFetchTime, &source.FetchInterval, &source.IsActive,
		&source.ErrorCount, &source.LastError, &source.CreatedAt,
	)

	if err != nil {
		return nil, err
	}
	return source, nil
}

// GetSourceByURL 根据 URL 获取订阅源
func (db *DB) GetSourceByURL(url string) (*Source, error) {
	source := &Source{}
	err := db.QueryRow(`
		SELECT id, url, COALESCE(title, ''), COALESCE(description, ''), 
		       last_fetch_time, fetch_interval, is_active, error_count, 
		       COALESCE(last_error, ''), created_at 
		FROM sources WHERE url = ?`,
		url,
	).Scan(
		&source.ID, &source.URL, &source.Title, &source.Description,
		&source.LastFetchTime, &source.FetchInterval, &source.IsActive,
		&source.ErrorCount, &source.LastError, &source.CreatedAt,
	)

	if err != nil {
		return nil, err
	}
	return source, nil
}

// GetActiveSources 获取所有活跃的订阅源
func (db *DB) GetActiveSources() ([]*Source, error) {
	rows, err := db.Query(`
		SELECT id, url, COALESCE(title, ''), COALESCE(description, ''), 
		       last_fetch_time, fetch_interval, is_active, error_count, 
		       COALESCE(last_error, ''), created_at 
		FROM sources 
		WHERE is_active = 1
		ORDER BY last_fetch_time ASC NULLS FIRST
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []*Source
	for rows.Next() {
		source := &Source{}
		err := rows.Scan(
			&source.ID, &source.URL, &source.Title, &source.Description,
			&source.LastFetchTime, &source.FetchInterval, &source.IsActive,
			&source.ErrorCount, &source.LastError, &source.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}

	return sources, rows.Err()
}

// UpdateSourceFetchTime 更新源的抓取时间
func (db *DB) UpdateSourceFetchTime(sourceID int64) error {
	_, err := db.Exec(
		"UPDATE sources SET last_fetch_time = ?, error_count = 0, last_error = '' WHERE id = ?",
		time.Now(), sourceID,
	)
	return err
}

// UpdateSourceError 更新源的错误信息
func (db *DB) UpdateSourceError(sourceID int64, errMsg string) error {
	_, err := db.Exec(`
		UPDATE sources 
		SET error_count = error_count + 1, 
		    last_error = ?,
		    is_active = CASE WHEN error_count + 1 >= 3 THEN 0 ELSE 1 END
		WHERE id = ?
	`, errMsg, sourceID)
	return err
}

// UpdateSourceActive 更新源的活跃状态
func (db *DB) UpdateSourceActive(sourceID int64, isActive bool) error {
	_, err := db.Exec("UPDATE sources SET is_active = ? WHERE id = ?", isActive, sourceID)
	return err
}

// DeleteSource 删除订阅源（级联删除关联的 items、subscriptions、user_deliveries 由外键负责）
func (db *DB) DeleteSource(sourceID int64) error {
	_, err := db.Exec("DELETE FROM sources WHERE id = ?", sourceID)
	return err
}

// Subscription 相关操作

// CreateSubscription 创建订阅关系
func (db *DB) CreateSubscription(userID, sourceID int64) error {
	_, err := db.Exec(
		"INSERT OR IGNORE INTO subscriptions (user_id, source_id) VALUES (?, ?)",
		userID, sourceID,
	)
	return err
}

// DeleteSubscription 删除订阅关系
func (db *DB) DeleteSubscription(userID, sourceID int64) error {
	_, err := db.Exec(
		"DELETE FROM subscriptions WHERE user_id = ? AND source_id = ?",
		userID, sourceID,
	)
	return err
}

// GetSubscriptionCount 获取订阅源的订阅数
func (db *DB) GetSubscriptionCount(sourceID int64) (int, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM subscriptions WHERE source_id = ?",
		sourceID,
	).Scan(&count)
	return count, err
}

// GetUserSubscriptions 获取用户的订阅列表
func (db *DB) GetUserSubscriptions(userID int64) ([]*Source, error) {
	rows, err := db.Query(`
		SELECT s.id, s.url, COALESCE(s.title, ''), COALESCE(s.description, ''), 
		       s.last_fetch_time, s.fetch_interval, s.is_active, s.error_count, 
		       COALESCE(s.last_error, ''), s.created_at 
		FROM sources s
		INNER JOIN subscriptions sub ON s.id = sub.source_id
		WHERE sub.user_id = ?
		ORDER BY sub.subscribed_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []*Source
	for rows.Next() {
		source := &Source{}
		err := rows.Scan(
			&source.ID, &source.URL, &source.Title, &source.Description,
			&source.LastFetchTime, &source.FetchInterval, &source.IsActive,
			&source.ErrorCount, &source.LastError, &source.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}

	return sources, rows.Err()
}

// GetSubscribedUserIDs 获取订阅某个源的所有用户 ID
func (db *DB) GetSubscribedUserIDs(sourceID int64) ([]int64, error) {
	rows, err := db.Query(
		"SELECT user_id FROM subscriptions WHERE source_id = ?",
		sourceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}

	return userIDs, rows.Err()
}

// GetUserSourceByURL 根据 URL 获取用户订阅的源
func (db *DB) GetUserSourceByURL(userID int64, sourceURL string) (*Source, error) {
	source := &Source{}
	err := db.QueryRow(`
		SELECT s.id, s.url, COALESCE(s.title, ''), COALESCE(s.description, ''), 
		       s.last_fetch_time, s.fetch_interval, s.is_active, s.error_count, 
		       COALESCE(s.last_error, ''), s.created_at 
		FROM sources s
		INNER JOIN subscriptions sub ON s.id = sub.source_id
		WHERE sub.user_id = ? AND s.url = ?
		LIMIT 1
	`, userID, sourceURL).Scan(
		&source.ID, &source.URL, &source.Title, &source.Description,
		&source.LastFetchTime, &source.FetchInterval, &source.IsActive,
		&source.ErrorCount, &source.LastError, &source.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return source, nil
}
