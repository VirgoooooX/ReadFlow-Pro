package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/readflow/gateway/internal/utils"
)

// Item 相关操作

// CreateItem 创建文章
func (db *DB) CreateItem(
	sourceID int64,
	guid, title, xmlContent, imagePaths string,
	publishedAt *time.Time,
	summary string,
	wordCount, readingTime int,
	coverImage, author, cleanContent, content, contentHash string,
	imageCaption, imageCredit string,
) (*Item, error) {
	result, err := db.Exec(`
		INSERT INTO items (
			source_id, guid, title, xml_content, image_paths, published_at,
			summary, word_count, reading_time, cover_image, author, clean_content, content, content_hash,
			image_caption, image_credit
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, sourceID, guid, title, xmlContent, imagePaths, publishedAt,
		summary, wordCount, readingTime, coverImage, author, cleanContent, content, contentHash,
		imageCaption, imageCredit)

	if err != nil {
		return nil, fmt.Errorf("failed to create item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return db.GetItemByID(id)
}

// GetItemByID 根据 ID 获取文章
func (db *DB) GetItemByID(id int64) (*Item, error) {
	item := &Item{}
	err := db.QueryRow(`
		SELECT id, source_id, guid, title, xml_content, 
		       COALESCE(image_paths, ''), published_at, created_at,
		       COALESCE(summary, ''), COALESCE(word_count, 0), COALESCE(reading_time, 0),
		       COALESCE(cover_image, ''), COALESCE(author, ''),
		       COALESCE(clean_content, ''), COALESCE(content, ''), COALESCE(content_hash, ''),
		       COALESCE(image_caption, ''), COALESCE(image_credit, '')
		FROM items WHERE id = ?
	`, id).Scan(
		&item.ID, &item.SourceID, &item.GUID, &item.Title,
		&item.XMLContent, &item.ImagePaths, &item.PublishedAt, &item.CreatedAt,
		&item.Summary, &item.WordCount, &item.ReadingTime,
		&item.CoverImage, &item.Author, &item.CleanContent, &item.Content, &item.ContentHash,
		&item.ImageCaption, &item.ImageCredit,
	)

	if err != nil {
		return nil, err
	}
	return item, nil
}

// GetItemByGUID 根据源 ID 和 GUID 获取文章（去重检查）
func (db *DB) GetItemByGUID(sourceID int64, guid string) (*Item, error) {
	item := &Item{}
	err := db.QueryRow(`
		SELECT id, source_id, guid, title, xml_content, 
		       COALESCE(image_paths, ''), published_at, created_at,
		       COALESCE(summary, ''), COALESCE(word_count, 0), COALESCE(reading_time, 0),
		       COALESCE(cover_image, ''), COALESCE(author, ''),
		       COALESCE(clean_content, ''), COALESCE(content, ''), COALESCE(content_hash, ''),
		       COALESCE(image_caption, ''), COALESCE(image_credit, '')
		FROM items WHERE source_id = ? AND guid = ?
	`, sourceID, guid).Scan(
		&item.ID, &item.SourceID, &item.GUID, &item.Title,
		&item.XMLContent, &item.ImagePaths, &item.PublishedAt, &item.CreatedAt,
		&item.Summary, &item.WordCount, &item.ReadingTime,
		&item.CoverImage, &item.Author, &item.CleanContent, &item.Content, &item.ContentHash,
		&item.ImageCaption, &item.ImageCredit,
	)

	if err != nil {
		return nil, err
	}
	return item, nil
}

// DeleteItem 删除文章
func (db *DB) DeleteItem(itemID int64) error {
	_, err := db.Exec("DELETE FROM items WHERE id = ?", itemID)
	return err
}

// UserDelivery 相关操作

// CreateUserDelivery 创建用户投递记录
func (db *DB) CreateUserDelivery(userID, itemID int64) error {
	_, err := db.Exec(
		"INSERT OR IGNORE INTO user_deliveries (user_id, item_id, status) VALUES (?, ?, 0)",
		userID, itemID,
	)
	return err
}

// BatchCreateUserDeliveries 批量创建用户投递记录
func (db *DB) BatchCreateUserDeliveries(itemID int64, userIDs []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT OR IGNORE INTO user_deliveries (user_id, item_id, status) VALUES (?, ?, 0)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, userID := range userIDs {
		if _, err := stmt.Exec(userID, itemID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetPendingDeliveries 获取用户待投递的文章
func (db *DB) GetPendingDeliveries(userID int64, limit int) ([]*Item, error) {
	rows, err := db.Query(`
		SELECT i.id, i.source_id, i.guid, i.title, i.xml_content, 
		       COALESCE(i.image_paths, ''), i.published_at, i.created_at,
		       COALESCE(i.clean_content, ''), COALESCE(i.content, ''),
		       COALESCE(i.cover_image, ''), COALESCE(i.summary, ''),
		       s.title, s.url
		FROM user_deliveries ud
		INNER JOIN items i ON ud.item_id = i.id
		INNER JOIN sources s ON i.source_id = s.id
		WHERE ud.user_id = ? AND ud.status = 0
		ORDER BY i.published_at DESC
		LIMIT ?
	`, userID, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*Item
	for rows.Next() {
		item := &Item{}
		err := rows.Scan(
			&item.ID, &item.SourceID, &item.GUID, &item.Title,
			&item.XMLContent, &item.ImagePaths, &item.PublishedAt, &item.CreatedAt,
			&item.CleanContent, &item.Content,
			&item.CoverImage, &item.Summary,
			&item.SourceTitle, &item.SourceURL,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// GetPendingDeliveriesBySourceURL 根据源URL获取用户待投递的文章
func (db *DB) GetPendingDeliveriesBySourceURL(userID int64, sourceURL string, limit int) ([]*Item, error) {
	rows, err := db.Query(`
		SELECT i.id, i.source_id, i.guid, i.title, i.xml_content, 
		       COALESCE(i.image_paths, ''), i.published_at, i.created_at,
		       COALESCE(i.clean_content, ''), COALESCE(i.content, ''),
		       COALESCE(i.cover_image, ''), COALESCE(i.summary, ''),
		       s.title, s.url
		FROM user_deliveries ud
		INNER JOIN items i ON ud.item_id = i.id
		INNER JOIN sources s ON i.source_id = s.id
		WHERE ud.user_id = ? AND ud.status = 0 AND s.url = ?
		ORDER BY i.published_at DESC
		LIMIT ?
	`, userID, sourceURL, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*Item
	for rows.Next() {
		item := &Item{}
		err := rows.Scan(
			&item.ID, &item.SourceID, &item.GUID, &item.Title,
			&item.XMLContent, &item.ImagePaths, &item.PublishedAt, &item.CreatedAt,
			&item.CleanContent, &item.Content,
			&item.CoverImage, &item.Summary,
			&item.SourceTitle, &item.SourceURL,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// UpdateDeliveryStatus 更新投递状态
func (db *DB) UpdateDeliveryStatus(userID, itemID int64, status int) error {
	_, err := db.Exec(
		"UPDATE user_deliveries SET status = ? WHERE user_id = ? AND item_id = ?",
		status, userID, itemID,
	)
	return err
}

// BatchUpdateDeliveryStatus 批量更新投递状态
func (db *DB) BatchUpdateDeliveryStatus(userID int64, itemIDs []int64, status int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("UPDATE user_deliveries SET status = ? WHERE user_id = ? AND item_id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, itemID := range itemIDs {
		if _, err := stmt.Exec(status, userID, itemID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetDeliveryStats 获取文章的投递统计
func (db *DB) GetDeliveryStats(itemID int64) (total, acked int, err error) {
	err = db.QueryRow(`
		SELECT COUNT(*) as total, 
		       SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) as acked
		FROM user_deliveries
		WHERE item_id = ?
	`, itemID).Scan(&total, &acked)
	return
}

// DeleteUserDeliveries 删除文章的所有投递记录
func (db *DB) DeleteUserDeliveries(itemID int64) error {
	_, err := db.Exec("DELETE FROM user_deliveries WHERE item_id = ?", itemID)
	return err
}

// GetUnreadCount 获取用户在某个源的未读文章数
func (db *DB) GetUnreadCount(userID, sourceID int64) (int, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM user_deliveries ud
		INNER JOIN items i ON ud.item_id = i.id
		WHERE ud.user_id = ? AND i.source_id = ? AND ud.status = 0
	`, userID, sourceID).Scan(&count)
	return count, err
}

// GetItemsBySource 获取某个订阅源下的所有文章
func (db *DB) GetItemsBySource(sourceID int64) ([]*Item, error) {
	rows, err := db.Query(`
		SELECT id, source_id, guid, title, xml_content,
		       COALESCE(image_paths, ''), published_at, created_at,
		       COALESCE(clean_content, ''), COALESCE(content, ''),
		       COALESCE(cover_image, ''), COALESCE(summary, '')
		FROM items
		WHERE source_id = ?
		ORDER BY created_at DESC
	`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*Item
	for rows.Next() {
		item := &Item{}
		err := rows.Scan(
			&item.ID, &item.SourceID, &item.GUID, &item.Title,
			&item.XMLContent, &item.ImagePaths, &item.PublishedAt, &item.CreatedAt,
			&item.CleanContent, &item.Content,
			&item.CoverImage, &item.Summary,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// GetUserArticles 获取用户文章列表（包含源信息与投递状态，支持增量同步、游标分页和按源筛选）
// 参数：
//   - userID: 用户 ID
//   - sourceID: 可选，订阅源 ID 过滤
//   - sinceTime: 可选，返回该时间之后发布的文章（增量同步）
//   - cursor: 可选，游标字符串 "timestamp_itemID"（历史翻页）
//   - limit: 返回数量限制
//   - offset: 偏移量（当 sinceTime 和 cursor 都为空时使用）
//
// 返回：
//   - articles: 文章列表
//   - nextCursor: 下一页游标（如果有更多数据）
//   - error: 错误信息
func (db *DB) GetUserArticles(
	userID int64,
	sourceID *int64,
	sinceTime *time.Time,
	cursor *string,
	limit, offset int,
) (articles []*UserArticle, nextCursor *string, err error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	// 多获取一条，用于判断是否有更多数据
	queryLimit := limit + 1

	query := `
		SELECT i.id, i.source_id, i.guid, i.title, i.xml_content,
		       COALESCE(i.image_paths, ''), i.published_at, i.created_at,
		       s.title, s.url, ud.status,
		       COALESCE(i.summary, ''), COALESCE(i.word_count, 0), COALESCE(i.reading_time, 0),
		       COALESCE(i.cover_image, ''), COALESCE(i.author, ''),
		       COALESCE(i.clean_content, ''), COALESCE(i.content, ''), COALESCE(i.content_hash, ''),
		       COALESCE(i.image_caption, ''), COALESCE(i.image_credit, ''),
		       COALESCE(ud.is_favorite, 0), COALESCE(ud.read_progress, 0),
		       ud.read_at, COALESCE(ud.updated_at, ud.delivered_at)
		FROM user_deliveries ud
		INNER JOIN items i ON ud.item_id = i.id
		INNER JOIN sources s ON i.source_id = s.id
		WHERE ud.user_id = ?
	`

	args := []interface{}{userID}

	// 按源过滤
	if sourceID != nil {
		query += " AND i.source_id = ?"
		args = append(args, *sourceID)
	}

	// 增量同步模式：since 优先
	if sinceTime != nil {
		query += " AND i.published_at > ?"
		args = append(args, *sinceTime)
	} else if cursor != nil && *cursor != "" {
		// 游标分页模式：解析 cursor
		cursorData, err := utils.DecodeCursor(*cursor)
		if err == nil {
			cursorTime := cursorData.GetTime()
			// 使用复合条件：(published_at, id) < (cursorTime, cursorID)
			query += " AND (i.published_at < ? OR (i.published_at = ? AND i.id < ?))"
			args = append(args, cursorTime, cursorTime, cursorData.ID)
		}
		// cursor 解析失败则忽略，按默认逻辑查询
	}

	// 排序和限制
	if sinceTime != nil || cursor != nil {
		// 增量或游标模式：不使用 offset
		query += `
			ORDER BY i.published_at DESC, i.id DESC
			LIMIT ?
		`
		args = append(args, queryLimit)
	} else {
		// 默认模式：使用 offset
		query += `
			ORDER BY i.published_at DESC, i.id DESC
			LIMIT ? OFFSET ?
		`
		args = append(args, queryLimit, offset)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var result []*UserArticle
	for rows.Next() {
		ua := &UserArticle{}
		if err := rows.Scan(
			&ua.ID, &ua.SourceID, &ua.GUID, &ua.Title,
			&ua.XMLContent, &ua.ImagePaths, &ua.PublishedAt, &ua.CreatedAt,
			&ua.SourceTitle, &ua.SourceURL, &ua.Status,
			&ua.Summary, &ua.WordCount, &ua.ReadingTime,
			&ua.CoverImage, &ua.Author, &ua.CleanContent, &ua.Content, &ua.ContentHash,
			&ua.ImageCaption, &ua.ImageCredit,
			&ua.IsFavorite, &ua.ReadProgress, &ua.ReadAt, &ua.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		result = append(result, ua)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// 判断是否有更多数据
	hasMore := len(result) > limit
	if hasMore {
		// 移除多余的最后一条
		result = result[:limit]
		// 生成 nextCursor（基于最后一条记录）
		last := result[len(result)-1]
		cursorStr := utils.SimpleCursorEncode(last.PublishedAt.Unix(), last.ID)
		nextCursor = &cursorStr
	}

	return result, nextCursor, nil
}

// Vocabulary 相关操作

// UpsertVocabulary 插入或更新生词
func (db *DB) UpsertVocabulary(vocab *Vocabulary) error {
	_, err := db.Exec(`
		INSERT INTO vocabularies (
			id, user_id, word, definition, translation, example, context,
			source_article_id, source_article_title, article_id,
			review_count, correct_count, last_review_at, next_review_at, mastery_level,
			difficulty, tags, notes,
			added_at, created_at, updated_at, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			definition = excluded.definition,
			translation = excluded.translation,
			example = excluded.example,
			context = excluded.context,
			review_count = excluded.review_count,
			correct_count = excluded.correct_count,
			last_review_at = excluded.last_review_at,
			next_review_at = excluded.next_review_at,
			mastery_level = excluded.mastery_level,
			difficulty = excluded.difficulty,
			tags = excluded.tags,
			notes = excluded.notes,
			updated_at = excluded.updated_at,
			is_deleted = excluded.is_deleted
			WHERE excluded.updated_at > vocabularies.updated_at
	`,
		vocab.ID, vocab.UserID, vocab.Word, vocab.Definition, vocab.Translation, vocab.Example, vocab.Context,
		vocab.SourceArticleID, vocab.SourceArticleTitle, vocab.ArticleID,
		vocab.ReviewCount, vocab.CorrectCount, vocab.LastReviewAt, vocab.NextReviewAt, vocab.MasteryLevel,
		vocab.Difficulty, vocab.Tags, vocab.Notes,
		vocab.AddedAt, vocab.CreatedAt, vocab.UpdatedAt, vocab.IsDeleted,
	)
	return err
}

// GetVocabulariesSince 获取指定时间后更新的生词
func (db *DB) GetVocabulariesSince(userID int64, sinceTimestamp int64) ([]*Vocabulary, error) {
	rows, err := db.Query(`
		SELECT 
			id, user_id, word, COALESCE(definition, ''), COALESCE(translation, ''),
			COALESCE(example, ''), COALESCE(context, ''),
			COALESCE(source_article_id, ''), COALESCE(source_article_title, ''), COALESCE(article_id, 0),
			COALESCE(review_count, 0), COALESCE(correct_count, 0),
			COALESCE(last_review_at, 0), COALESCE(next_review_at, 0), COALESCE(mastery_level, 0),
			COALESCE(difficulty, 'medium'), COALESCE(tags, ''), COALESCE(notes, ''),
			COALESCE(added_at, 0), COALESCE(created_at, 0), COALESCE(updated_at, 0), is_deleted
		FROM vocabularies
		WHERE user_id = ? AND updated_at > ? AND is_deleted = 0
		ORDER BY updated_at DESC
	`, userID, sinceTimestamp)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vocabs []*Vocabulary
	for rows.Next() {
		vocab := &Vocabulary{}
		err := rows.Scan(
			&vocab.ID, &vocab.UserID, &vocab.Word, &vocab.Definition, &vocab.Translation,
			&vocab.Example, &vocab.Context,
			&vocab.SourceArticleID, &vocab.SourceArticleTitle, &vocab.ArticleID,
			&vocab.ReviewCount, &vocab.CorrectCount,
			&vocab.LastReviewAt, &vocab.NextReviewAt, &vocab.MasteryLevel,
			&vocab.Difficulty, &vocab.Tags, &vocab.Notes,
			&vocab.AddedAt, &vocab.CreatedAt, &vocab.UpdatedAt, &vocab.IsDeleted,
		)
		if err != nil {
			return nil, err
		}
		vocabs = append(vocabs, vocab)
	}

	return vocabs, rows.Err()
}

// GetVocabularyByID 根据ID获取生词
func (db *DB) GetVocabularyByID(vocabID string) (*Vocabulary, error) {
	vocab := &Vocabulary{}
	err := db.QueryRow(`
		SELECT 
			id, user_id, word, COALESCE(definition, ''), COALESCE(translation, ''),
			COALESCE(example, ''), COALESCE(context, ''),
			COALESCE(source_article_id, ''), COALESCE(source_article_title, ''), COALESCE(article_id, 0),
			COALESCE(review_count, 0), COALESCE(correct_count, 0),
			COALESCE(last_review_at, 0), COALESCE(next_review_at, 0), COALESCE(mastery_level, 0),
			COALESCE(difficulty, 'medium'), COALESCE(tags, ''), COALESCE(notes, ''),
			COALESCE(added_at, 0), COALESCE(created_at, 0), COALESCE(updated_at, 0), is_deleted
		FROM vocabularies
		WHERE id = ?
	`, vocabID).Scan(
		&vocab.ID, &vocab.UserID, &vocab.Word, &vocab.Definition, &vocab.Translation,
		&vocab.Example, &vocab.Context,
		&vocab.SourceArticleID, &vocab.SourceArticleTitle, &vocab.ArticleID,
		&vocab.ReviewCount, &vocab.CorrectCount,
		&vocab.LastReviewAt, &vocab.NextReviewAt, &vocab.MasteryLevel,
		&vocab.Difficulty, &vocab.Tags, &vocab.Notes,
		&vocab.AddedAt, &vocab.CreatedAt, &vocab.UpdatedAt, &vocab.IsDeleted,
	)

	if err != nil {
		return nil, err
	}
	return vocab, nil
}

// GetVocabularyByWord 根据单词获取生词
func (db *DB) GetVocabularyByWord(userID int64, word string) (*Vocabulary, error) {
	vocab := &Vocabulary{}
	err := db.QueryRow(`
		SELECT 
			id, user_id, word, COALESCE(definition, ''), COALESCE(translation, ''),
			COALESCE(example, ''), COALESCE(context, ''),
			COALESCE(source_article_id, ''), COALESCE(source_article_title, ''), COALESCE(article_id, 0),
			COALESCE(review_count, 0), COALESCE(correct_count, 0),
			COALESCE(last_review_at, 0), COALESCE(next_review_at, 0), COALESCE(mastery_level, 0),
			COALESCE(difficulty, 'medium'), COALESCE(tags, ''), COALESCE(notes, ''),
			COALESCE(added_at, 0), COALESCE(created_at, 0), COALESCE(updated_at, 0), is_deleted
		FROM vocabularies
		WHERE user_id = ? AND word = ?
	`, userID, word).Scan(
		&vocab.ID, &vocab.UserID, &vocab.Word, &vocab.Definition, &vocab.Translation,
		&vocab.Example, &vocab.Context,
		&vocab.SourceArticleID, &vocab.SourceArticleTitle, &vocab.ArticleID,
		&vocab.ReviewCount, &vocab.CorrectCount,
		&vocab.LastReviewAt, &vocab.NextReviewAt, &vocab.MasteryLevel,
		&vocab.Difficulty, &vocab.Tags, &vocab.Notes,
		&vocab.AddedAt, &vocab.CreatedAt, &vocab.UpdatedAt, &vocab.IsDeleted,
	)

	if err != nil {
		return nil, err
	}
	return vocab, nil
}

// DeleteVocabulary 软删除生词
func (db *DB) DeleteVocabulary(vocabID string) error {
	now := int64(time.Now().Unix())
	_, err := db.Exec(
		"UPDATE vocabularies SET is_deleted = 1, updated_at = ? WHERE id = ?",
		now, vocabID,
	)
	return err
}

// GetVocabulariesByUser 获取用户的所有生词
func (db *DB) GetVocabulariesByUser(userID int64, limit int64, offset int64) ([]*Vocabulary, error) {
	rows, err := db.Query(`
		SELECT 
			id, user_id, word, COALESCE(definition, ''), COALESCE(translation, ''),
			COALESCE(example, ''), COALESCE(context, ''),
			COALESCE(source_article_id, ''), COALESCE(source_article_title, ''), COALESCE(article_id, 0),
			COALESCE(review_count, 0), COALESCE(correct_count, 0),
			COALESCE(last_review_at, 0), COALESCE(next_review_at, 0), COALESCE(mastery_level, 0),
			COALESCE(difficulty, 'medium'), COALESCE(tags, ''), COALESCE(notes, ''),
			COALESCE(added_at, 0), COALESCE(created_at, 0), COALESCE(updated_at, 0), is_deleted
		FROM vocabularies
		WHERE user_id = ? AND is_deleted = 0
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`, userID, limit, offset)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vocabs []*Vocabulary
	for rows.Next() {
		vocab := &Vocabulary{}
		err := rows.Scan(
			&vocab.ID, &vocab.UserID, &vocab.Word, &vocab.Definition, &vocab.Translation,
			&vocab.Example, &vocab.Context,
			&vocab.SourceArticleID, &vocab.SourceArticleTitle, &vocab.ArticleID,
			&vocab.ReviewCount, &vocab.CorrectCount,
			&vocab.LastReviewAt, &vocab.NextReviewAt, &vocab.MasteryLevel,
			&vocab.Difficulty, &vocab.Tags, &vocab.Notes,
			&vocab.AddedAt, &vocab.CreatedAt, &vocab.UpdatedAt, &vocab.IsDeleted,
		)
		if err != nil {
			return nil, err
		}
		vocabs = append(vocabs, vocab)
	}

	return vocabs, rows.Err()
}

// GetDeliveredItems 获取所有已发送的文章列表
func (db *DB) GetDeliveredItems() ([]int64, error) {
	rows, err := db.Query(`
		SELECT DISTINCT item_id
		FROM user_deliveries
		WHERE status = 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var itemIDs []int64
	for rows.Next() {
		var itemID int64
		if err := rows.Scan(&itemID); err != nil {
			return nil, err
		}
		itemIDs = append(itemIDs, itemID)
	}
	return itemIDs, rows.Err()
}

// GetItemDeliveredTime 获取文章最近的发送时间
func (db *DB) GetItemDeliveredTime(itemID int64) (*time.Time, error) {
	var deliveredAtStr sql.NullString
	err := db.QueryRow(`
		SELECT COALESCE(MAX(delivered_at), '')
		FROM user_deliveries
		WHERE item_id = ? AND status = 1
	`, itemID).Scan(&deliveredAtStr)
	if err != nil {
		return nil, err
	}

	if !deliveredAtStr.Valid || deliveredAtStr.String == "" {
		return nil, nil
	}

	// 解析时间字符串
	deliveredAt, err := time.Parse("2006-01-02 15:04:05", deliveredAtStr.String)
	if err != nil {
		return nil, err
	}
	return &deliveredAt, nil
}

// Quest 5: 阅读状态管理函数

// MarkArticleAsRead 标记文章为已读
func (db *DB) MarkArticleAsRead(userID, itemID int64) error {
	now := time.Now()
	_, err := db.Exec(`
		UPDATE user_deliveries 
		SET status = 2, 
		    read_at = COALESCE(read_at, ?),
		    updated_at = ?
		WHERE user_id = ? AND item_id = ?
	`, now, now, userID, itemID)
	return err
}

// MarkArticleAsUnread 标记文章为未读
func (db *DB) MarkArticleAsUnread(userID, itemID int64) error {
	now := time.Now()
	_, err := db.Exec(`
		UPDATE user_deliveries 
		SET status = 0, 
		    read_at = NULL,
		    updated_at = ?
		WHERE user_id = ? AND item_id = ?
	`, now, userID, itemID)
	return err
}

// ToggleFavorite 切换文章收藏状态
func (db *DB) ToggleFavorite(userID, itemID int64) (isFavorite bool, err error) {
	now := time.Now()
	var currentFavorite bool

	// 查询当前收藏状态
	err = db.QueryRow(`
		SELECT COALESCE(is_favorite, 0)
		FROM user_deliveries
		WHERE user_id = ? AND item_id = ?
	`, userID, itemID).Scan(&currentFavorite)
	if err != nil {
		return false, err
	}

	// 切换状态
	newFavorite := !currentFavorite
	_, err = db.Exec(`
		UPDATE user_deliveries 
		SET is_favorite = ?,
		    updated_at = ?
		WHERE user_id = ? AND item_id = ?
	`, newFavorite, now, userID, itemID)
	if err != nil {
		return false, err
	}

	return newFavorite, nil
}

// SetFavorite 设置文章收藏状态
func (db *DB) SetFavorite(userID, itemID int64, isFavorite bool) error {
	now := time.Now()
	_, err := db.Exec(`
		UPDATE user_deliveries 
		SET is_favorite = ?,
		    updated_at = ?
		WHERE user_id = ? AND item_id = ?
	`, isFavorite, now, userID, itemID)
	return err
}

// UpdateReadProgress 更新阅读进度
func (db *DB) UpdateReadProgress(userID, itemID int64, progress int) error {
	// 限制进度范围 0-100
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	now := time.Now()
	_, err := db.Exec(`
		UPDATE user_deliveries 
		SET read_progress = ?,
		    updated_at = ?
		WHERE user_id = ? AND item_id = ?
	`, progress, now, userID, itemID)
	return err
}
