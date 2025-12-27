package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB 数据库连接实例
type DB struct {
	*sql.DB
}

// New 创建新的数据库连接
func New(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 配置连接池 - SQLite 优化
	db.SetMaxOpenConns(1)            // SQLite 单写限制
	db.SetMaxIdleConns(1)            // 保持一个空闲连接
	db.SetConnMaxLifetime(time.Hour) // 连接最长生命周期

	// 启用 WAL 模式和优化配置
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000;"); err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// 初始化数据库表结构
	if _, err := db.Exec(Schema); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Println("Database initialized successfully")

	return &DB{db}, nil
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	return db.DB.Close()
}

// Models 数据模型定义

// User 用户
type User struct {
	ID          int64
	Username    string
	Token       string
	CreatedAt   time.Time
	LastLoginAt *time.Time
}

// Source 订阅源
type Source struct {
	ID            int64
	URL           string
	Title         string
	Description   string
	LastFetchTime *time.Time
	FetchInterval int
	IsActive      bool
	ErrorCount    int
	LastError     string
	CreatedAt     time.Time
}

// Subscription 订阅关系
type Subscription struct {
	UserID       int64
	SourceID     int64
	SubscribedAt time.Time
}

// Item 文章
type Item struct {
	ID          int64
	SourceID    int64
	GUID        string
	Title       string
	XMLContent  string
	ImagePaths  string
	PublishedAt *time.Time
	CreatedAt   time.Time
	// Quest 3: 新增字段
	Summary      string
	WordCount    int
	ReadingTime  int
	CoverImage   string
	Author       string
	CleanContent string
	Content      string // Original content
	ContentHash  string
	ImageCaption string // Added
	ImageCredit  string // Added
	SourceTitle  string // Added for sync
	SourceURL    string // Added for sync
}

// UserArticle 用户视角的文章（包含源信息与投递状态）
type UserArticle struct {
	ID          int64
	SourceID    int64
	GUID        string
	Title       string
	XMLContent  string
	ImagePaths  string
	PublishedAt *time.Time
	CreatedAt   time.Time
	SourceTitle string
	SourceURL   string
	Status      int
	// Quest 3: 新增字段
	Summary      string
	WordCount    int
	ReadingTime  int
	CoverImage   string
	Author       string
	CleanContent string
	Content      string // Original content
	ContentHash  string
	ImageCaption string // Added
	ImageCredit  string // Added
	// Quest 5: 阅读状态字段
	IsFavorite   bool
	ReadProgress int
	ReadAt       *time.Time
	UpdatedAt    time.Time
}

// UserDelivery 用户投递状态
type UserDelivery struct {
	UserID      int64
	ItemID      int64
	Status      int
	DeliveredAt time.Time
	// Quest 5: 阅读状态字段
	IsFavorite   bool
	ReadProgress int
	ReadAt       *time.Time
	UpdatedAt    time.Time
}

// Vocabulary 生词
type Vocabulary struct {
	// 主键和基础信息
	ID          string // 单词ID（字符串主Key）
	UserID      int64  // 用户ID
	Word        string // 单词
	Definition  string // 定义（JSON字符串）
	Translation string // 翻译
	Example     string // 例句
	Context     string // 上下文

	// 来源信息
	SourceArticleID    string // 来源文章ID（客户端一致）
	SourceArticleTitle string // 来源文章标题
	ArticleID          int64  // 文章ID（数字形式，备用）

	// 学习进度
	ReviewCount  int64 // 复习次数
	CorrectCount int64 // 正确次数
	LastReviewAt int64 // 最后复习时间（Unix时间戳）
	NextReviewAt int64 // 下次复习时间（Unix时间戳）
	MasteryLevel int64 // 掌握程度（0-5）

	// 分类和备注
	Difficulty string // 难度（easy/medium/hard）
	Tags       string // 标签（JSON数组）
	Notes      string // 备注

	// 系统时间戳（都是Unix时间戳）
	AddedAt   int64 // 添加时间
	CreatedAt int64 // 创建时间
	UpdatedAt int64 // 更新时间
	IsDeleted bool  // 软删除标记
}
