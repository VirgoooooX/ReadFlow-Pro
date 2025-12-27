package db

import (
	"fmt"
	"log"
)

// GetAllUsers 获取所有用户
func (db *DB) GetAllUsers() ([]*User, error) {
	rows, err := db.Query(`
		SELECT id, username, token, created_at, last_login_at
		FROM users
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user := &User{}
		if err := rows.Scan(&user.ID, &user.Username, &user.Token, &user.CreatedAt, &user.LastLoginAt); err != nil {
			log.Printf("Error scanning user: %v", err)
			continue
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

// GetAllSources 获取所有订阅源
func (db *DB) GetAllSources() ([]*Source, error) {
	rows, err := db.Query(`
		SELECT id, url, title, description, last_fetch_time, fetch_interval, 
		       is_active, error_count, COALESCE(last_error, ''), created_at
		FROM sources
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []*Source
	for rows.Next() {
		source := &Source{}
		if err := rows.Scan(
			&source.ID, &source.URL, &source.Title, &source.Description,
			&source.LastFetchTime, &source.FetchInterval, &source.IsActive,
			&source.ErrorCount, &source.LastError, &source.CreatedAt,
		); err != nil {
			log.Printf("Error scanning source: %v", err)
			continue
		}
		sources = append(sources, source)
	}

	return sources, rows.Err()
}

// GetTotalUsers 获取用户总数
func (db *DB) GetTotalUsers() (int64, error) {
	var count int64
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// GetActiveSourcesCount 获取活跃源数量
func (db *DB) GetActiveSourcesCount() (int64, error) {
	var count int64
	err := db.QueryRow("SELECT COUNT(*) FROM sources WHERE is_active = 1").Scan(&count)
	return count, err
}

// GetTotalItems 获取文章总数
func (db *DB) GetTotalItems() (int64, error) {
	var count int64
	err := db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	return count, err
}

// GetTotalDeliveries 获取投递总数
func (db *DB) GetTotalDeliveries() (int64, error) {
	var count int64
	err := db.QueryRow("SELECT COUNT(*) FROM user_deliveries").Scan(&count)
	return count, err
}

// GetItemCountBySource 获取源的文章数量
func (db *DB) GetItemCountBySource(sourceID int64) (int64, error) {
	var count int64
	err := db.QueryRow(
		"SELECT COUNT(*) FROM items WHERE source_id = ?",
		sourceID,
	).Scan(&count)
	return count, err
}

// GetSubscriberCountBySource 获取订阅者数量
func (db *DB) GetSubscriberCountBySource(sourceID int64) (int64, error) {
	var count int64
	err := db.QueryRow(
		"SELECT COUNT(*) FROM subscriptions WHERE source_id = ?",
		sourceID,
	).Scan(&count)
	return count, err
}

// GetDeliveryCountBySource 获取源的投递数量
func (db *DB) GetDeliveryCountBySource(sourceID int64) (int64, error) {
	var count int64
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM user_deliveries ud
		INNER JOIN items i ON ud.item_id = i.id
		WHERE i.source_id = ?
	`, sourceID).Scan(&count)
	return count, err
}

// GetSubscriptionCountByUser 获取用户的订阅数量
func (db *DB) GetSubscriptionCountByUser(userID int64) (int64, error) {
	var count int64
	err := db.QueryRow(
		"SELECT COUNT(*) FROM subscriptions WHERE user_id = ?",
		userID,
	).Scan(&count)
	return count, err
}

// GetDeliveryCountByUser 获取用户的投递数量
func (db *DB) GetDeliveryCountByUser(userID int64) (int64, error) {
	var count int64
	err := db.QueryRow(
		"SELECT COUNT(*) FROM user_deliveries WHERE user_id = ?",
		userID,
	).Scan(&count)
	return count, err
}

// GetDeliveredCountByUserAndSource 获取用户从某个源拉取的文章数量
func (db *DB) GetDeliveredCountByUserAndSource(userID, sourceID int64) (int64, error) {
	var count int64
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM user_deliveries ud
		INNER JOIN items i ON ud.item_id = i.id
		WHERE ud.user_id = ? AND i.source_id = ?
	`, userID, sourceID).Scan(&count)
	return count, err
}

// GetVocabularyCountByUser 获取用户的生词数量
func (db *DB) GetVocabularyCountByUser(userID int64) (int64, error) {
	var count int64
	err := db.QueryRow(
		"SELECT COUNT(*) FROM vocabularies WHERE user_id = ? AND is_deleted = 0",
		userID,
	).Scan(&count)
	return count, err
}

// GetSubscriptionsByUser 获取用户的所有订阅
func (db *DB) GetSubscriptionsByUser(userID int64) ([]*Subscription, error) {
	rows, err := db.Query(
		"SELECT user_id, source_id, subscribed_at FROM subscriptions WHERE user_id = ? ORDER BY subscribed_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		sub := &Subscription{}
		if err := rows.Scan(&sub.UserID, &sub.SourceID, &sub.SubscribedAt); err != nil {
			log.Printf("Error scanning subscription: %v", err)
			continue
		}
		subs = append(subs, sub)
	}

	return subs, rows.Err()
}

// GetDetailedSourceStats 获取源的详细统计（包括最近一周的数据）
func (db *DB) GetDetailedSourceStats(sourceID int64) (map[string]interface{}, error) {
	source, err := db.GetSourceByID(sourceID)
	if err != nil {
		return nil, err
	}

	// 获取过去7天的投递数量
	var weekDeliveries int64
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM user_deliveries ud
		INNER JOIN items i ON ud.item_id = i.id
		WHERE i.source_id = ? AND ud.delivered_at > datetime('now', '-7 days')
	`, sourceID).Scan(&weekDeliveries)

	// 获取最近添加的文章数
	var recentItems int64
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM items
		WHERE source_id = ? AND created_at > datetime('now', '-7 days')
	`, sourceID).Scan(&recentItems)

	return map[string]interface{}{
		"source":                 source,
		"week_deliveries":        weekDeliveries,
		"recent_items":           recentItems,
		"avg_items_per_day":      float64(recentItems) / 7.0,
		"avg_deliveries_per_day": float64(weekDeliveries) / 7.0,
	}, nil
}

// GetUserActivityStats 获取用户活动统计（最近一周）
func (db *DB) GetUserActivityStats(userID int64) (map[string]interface{}, error) {
	user, err := db.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	// 过去7天的投递数量
	var weekDeliveries int64
	db.QueryRow(`
		SELECT COUNT(*)
		FROM user_deliveries
		WHERE user_id = ? AND delivered_at > datetime('now', '-7 days')
	`, userID).Scan(&weekDeliveries)

	// 过去7天的已读数量
	var weekAcked int64
	db.QueryRow(`
		SELECT COUNT(*)
		FROM user_deliveries
		WHERE user_id = ? AND status = 1 AND delivered_at > datetime('now', '-7 days')
	`, userID).Scan(&weekAcked)

	// 未读数量
	var unread int64
	db.QueryRow(
		"SELECT COUNT(*) FROM user_deliveries WHERE user_id = ? AND status = 0",
		userID,
	).Scan(&unread)

	return map[string]interface{}{
		"user":             user,
		"week_deliveries":  weekDeliveries,
		"week_acked":       weekAcked,
		"unread":           unread,
		"week_unread_rate": fmt.Sprintf("%.2f%%", float64(weekDeliveries-weekAcked)/float64(weekDeliveries+1)*100),
	}, nil
}
