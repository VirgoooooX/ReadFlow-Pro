package utils

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CursorData 游标数据结构
type CursorData struct {
	Timestamp int64
	ID        int64
}

// EncodeCursor 编码游标（格式："timestamp_id" -> Base64）
// 参数：
//   - timestamp: Unix 时间戳
//   - id: 记录 ID
// 返回：Base64 编码的游标字符串
func EncodeCursor(timestamp int64, id int64) string {
	raw := fmt.Sprintf("%d_%d", timestamp, id)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

// EncodeCursorFromTime 从 time.Time 编码游标
func EncodeCursorFromTime(t time.Time, id int64) string {
	return EncodeCursor(t.Unix(), id)
}

// DecodeCursor 解码游标（Base64 -> "timestamp_id"）
// 返回：
//   - data: 游标数据
//   - err: 解码错误
func DecodeCursor(cursor string) (*CursorData, error) {
	if cursor == "" {
		return nil, fmt.Errorf("cursor is empty")
	}

	// 尝试 Base64 解码
	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		// 如果解码失败，可能是未编码的格式（向后兼容）
		return parseCursorRaw(cursor)
	}

	return parseCursorRaw(string(decoded))
}

// parseCursorRaw 解析原始游标字符串（格式："timestamp_id"）
func parseCursorRaw(raw string) (*CursorData, error) {
	parts := strings.Split(raw, "_")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cursor format: %s", raw)
	}

	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp in cursor: %w", err)
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid id in cursor: %w", err)
	}

	return &CursorData{
		Timestamp: timestamp,
		ID:        id,
	}, nil
}

// GetTimeFromCursor 从游标中获取时间
func (c *CursorData) GetTime() time.Time {
	return time.Unix(c.Timestamp, 0)
}

// SimpleCursorEncode 简单编码游标（不使用 Base64，直接返回 "timestamp_id"）
// 用于内部存储或调试
func SimpleCursorEncode(timestamp int64, id int64) string {
	return fmt.Sprintf("%d_%d", timestamp, id)
}

// SimpleCursorDecode 简单解码游标（直接解析 "timestamp_id"）
func SimpleCursorDecode(cursor string) (timestamp int64, id int64, err error) {
	var data *CursorData
	data, err = parseCursorRaw(cursor)
	if err != nil {
		return 0, 0, err
	}
	return data.Timestamp, data.ID, nil
}
