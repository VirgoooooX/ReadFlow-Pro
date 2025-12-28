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
	// 使用更健壮的连接字符串，显式设置模式和超时
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 验证连接是否可写
	var readonly string
	err = db.QueryRow("PRAGMA readonly").Scan(&readonly)
	if err != nil {
		// 如果 PRAGMA 不返回结果，通常意味着它是旧版本 SQLite 或不可写模式下的特殊表现
		// 我们可以尝试执行一个轻量级的写操作来最终确认
		log.Printf("[DEBUG] PRAGMA readonly failed or returned no rows: %v. Trying fallback check...", err)
		_, err = db.Exec("CREATE TABLE IF NOT EXISTS _write_test (id INTEGER PRIMARY KEY); DROP TABLE _write_test;")
		if err != nil {
			return nil, fmt.Errorf("database is NOT writable: %w", err)
		}
	} else if readonly == "1" || readonly == "true" {
		return nil, fmt.Errorf("database is opened in READ-ONLY mode, check file permissions")
	}

	// 配置连接池 - SQLite 优化
	db.SetMaxOpenConns(1)            // SQLite 单写限制
	db.SetMaxIdleConns(1)            // 保持一个空闲连接
	db.SetConnMaxLifetime(time.Hour) // 连接最长生命周期

	// 1. 首先确保基础表结构存在
	// 使用 CREATE TABLE IF NOT EXISTS 保证幂等性
	if _, err := db.Exec(Schema); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// 2. 然后再执行增量迁移逻辑
	// 此时所有基础表已经存在，migrate() 中的 ALTER TABLE 不会报错
	database := &DB{db}
	if err := database.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("Database initialized successfully")

	return database, nil
}

// migrate 处理简单的数据库迁移
func (db *DB) migrate() error {
	// 检查 sources 表是否存在 category 列
	if !db.columnExists("sources", "category") {
		log.Println("[Migration] Adding column 'category' to 'sources' table")
		if _, err := db.Exec("ALTER TABLE sources ADD COLUMN category TEXT DEFAULT 'Technology'"); err != nil {
			return err
		}
	}
	// 确保 category 索引存在
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_sources_category ON sources(category)"); err != nil {
		log.Printf("[Migration] Warning: Failed to create idx_sources_category: %v", err)
	}

	// 检查 sources 表是否存在 language 列
	if !db.columnExists("sources", "language") {
		log.Println("[Migration] Adding column 'language' to 'sources' table")
		if _, err := db.Exec("ALTER TABLE sources ADD COLUMN language TEXT DEFAULT 'en'"); err != nil {
			return err
		}
	}
	// 确保 language 索引存在
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_sources_language ON sources(language)"); err != nil {
		log.Printf("[Migration] Warning: Failed to create idx_sources_language: %v", err)
	}

	// 检查 items 表是否存在 category 列
	if !db.columnExists("items", "category") {
		log.Println("[Migration] Adding column 'category' to 'items' table")
		if _, err := db.Exec("ALTER TABLE items ADD COLUMN category TEXT"); err != nil {
			return err
		}
	}
	// 确保 category 索引存在
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_items_category ON items(category)"); err != nil {
		log.Printf("[Migration] Warning: Failed to create idx_items_category: %v", err)
	}

	// 检查 items 表是否存在 content 列
	if !db.columnExists("items", "content") {
		log.Println("[Migration] Adding column 'content' to 'items' table")
		if _, err := db.Exec("ALTER TABLE items ADD COLUMN content TEXT"); err != nil {
			return err
		}
	}

	// 检查 items 表是否存在 difficulty 列
	if !db.columnExists("items", "difficulty") {
		log.Println("[Migration] Adding column 'difficulty' to 'items' table")
		if _, err := db.Exec("ALTER TABLE items ADD COLUMN difficulty TEXT DEFAULT 'medium'"); err != nil {
			return err
		}
	}
	// 确保 difficulty 索引存在
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_items_difficulty ON items(difficulty)"); err != nil {
		log.Printf("[Migration] Warning: Failed to create idx_items_difficulty: %v", err)
	}

	// 检查 items 表是否存在 url 列
	if !db.columnExists("items", "url") {
		log.Println("[Migration] Adding column 'url' to 'items' table")
		if _, err := db.Exec("ALTER TABLE items ADD COLUMN url TEXT"); err != nil {
			return err
		}
	}
	// 确保 url 索引存在
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_items_url ON items(url)"); err != nil {
		log.Printf("[Migration] Warning: Failed to create idx_items_url: %v", err)
	}

	// 检查 items 表是否存在 tags 列
	if !db.columnExists("items", "tags") {
		log.Println("[Migration] Adding column 'tags' to 'items' table")
		if _, err := db.Exec("ALTER TABLE items ADD COLUMN tags TEXT"); err != nil {
			return err
		}
	}

	// 检查 items 表是否存在 image_caption 列
	if !db.columnExists("items", "image_caption") {
		log.Println("[Migration] Adding column 'image_caption' to 'items' table")
		if _, err := db.Exec("ALTER TABLE items ADD COLUMN image_caption TEXT"); err != nil {
			return err
		}
	}

	// 检查 items 表是否存在 image_credit 列
	if !db.columnExists("items", "image_credit") {
		log.Println("[Migration] Adding column 'image_credit' to 'items' table")
		if _, err := db.Exec("ALTER TABLE items ADD COLUMN image_credit TEXT"); err != nil {
			return err
		}
	}

	// 检查 items 表是否存在 image_primary_color 列
	if !db.columnExists("items", "image_primary_color") {
		log.Println("[Migration] Adding column 'image_primary_color' to 'items' table")
		if _, err := db.Exec("ALTER TABLE items ADD COLUMN image_primary_color TEXT"); err != nil {
			return err
		}
	}

	// 检查 user_deliveries 表
	if !db.columnExists("user_deliveries", "is_read") {
		log.Println("[Migration] Adding column 'is_read' to 'user_deliveries' table")
		if _, err := db.Exec("ALTER TABLE user_deliveries ADD COLUMN is_read BOOLEAN DEFAULT 0"); err != nil {
			return err
		}
	}
	// 确保 is_read 索引存在
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_deliveries_user_read ON user_deliveries(user_id, is_read)"); err != nil {
		log.Printf("[Migration] Warning: Failed to create idx_deliveries_user_read: %v", err)
	}
	if !db.columnExists("user_deliveries", "scroll_position") {
		log.Println("[Migration] Adding column 'scroll_position' to 'user_deliveries' table")
		if _, err := db.Exec("ALTER TABLE user_deliveries ADD COLUMN scroll_position INTEGER DEFAULT 0"); err != nil {
			return err
		}
	}
	if !db.columnExists("user_deliveries", "user_tags") {
		log.Println("[Migration] Adding column 'user_tags' to 'user_deliveries' table")
		if _, err := db.Exec("ALTER TABLE user_deliveries ADD COLUMN user_tags TEXT"); err != nil {
			return err
		}
	}
	if !db.columnExists("user_deliveries", "reading_time_spent") {
		log.Println("[Migration] Adding column 'reading_time_spent' to 'user_deliveries' table")
		if _, err := db.Exec("ALTER TABLE user_deliveries ADD COLUMN reading_time_spent INTEGER DEFAULT 0"); err != nil {
			return err
		}
	}
	if !db.columnExists("user_deliveries", "last_read_at") {
		log.Println("[Migration] Adding column 'last_read_at' to 'user_deliveries' table")
		if _, err := db.Exec("ALTER TABLE user_deliveries ADD COLUMN last_read_at DATETIME"); err != nil {
			return err
		}
	}

	// 检查 users 表
	if !db.columnExists("users", "email") {
		log.Println("[Migration] Adding column 'email' to 'users' table")
		if _, err := db.Exec("ALTER TABLE users ADD COLUMN email TEXT"); err != nil {
			return err
		}
		// 添加索引
		if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)"); err != nil {
			log.Printf("[Migration] Warning: Failed to create idx_users_email: %v", err)
		}
	}

	if !db.columnExists("users", "password_hash") {
		log.Println("[Migration] Adding column 'password_hash' to 'users' table")
		if _, err := db.Exec("ALTER TABLE users ADD COLUMN password_hash TEXT"); err != nil {
			return err
		}
	}

	return nil
}

// columnExists 检查表中是否存在指定列
func (db *DB) columnExists(tableName, columnName string) bool {
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			dataType  string
			notnull   int
			dfltValue interface{}
			pk        int
		)
		if err := rows.Scan(&cid, &name, &dataType, &notnull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == columnName {
			return true
		}
	}
	return false
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	return db.DB.Close()
}

// Models 数据模型定义

// User 用户
type User struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	Token        string
	CreatedAt    time.Time
	LastLoginAt  *time.Time
}

// UserPreference 用户偏好设置
type UserPreference struct {
	UserID                    int64  `json:"user_id"`
	ReadingSettings           string `json:"reading_settings"` // JSON string
	TranslationProvider       string `json:"translation_provider"`
	EnableAutoTranslation     bool   `json:"enable_auto_translation"`
	EnableTitleTranslation    bool   `json:"enable_title_translation"`
	MaxConcurrentTranslations int    `json:"max_concurrent_translations"`
	TranslationTimeout        int    `json:"translation_timeout"`
	DefaultCategory           string `json:"default_category"`
	EnableNotifications       bool   `json:"enable_notifications"`
	ProxyModeEnabled          bool   `json:"proxy_mode_enabled"`
	ProxyServerURL            string `json:"proxy_server_url"`
	ProxyToken                string `json:"proxy_token"`
	CreatedAt                 int64  `json:"created_at"`
	UpdatedAt                 int64  `json:"updated_at"`
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
	ID          int64      `json:"ID"`
	SourceID    int64      `json:"SourceID"`
	GUID        string     `json:"GUID"`
	Title       string     `json:"Title"`
	XMLContent  string     `json:"XMLContent"`
	ImagePaths  string     `json:"ImagePaths"`
	PublishedAt *time.Time `json:"PublishedAt"`
	CreatedAt   time.Time  `json:"CreatedAt"`
	// Quest 3: 新增字段
	Summary           string `json:"Summary"`
	WordCount         int    `json:"WordCount"`
	ReadingTime       int    `json:"ReadingTime"`
	CoverImage        string `json:"CoverImage"`
	Author            string `json:"Author"`
	CleanContent      string `json:"CleanContent"`
	Content           string `json:"Content"` // Original content
	ContentHash       string `json:"ContentHash"`
	ImageCaption      string `json:"ImageCaption"`      // Added
	ImageCredit       string `json:"ImageCredit"`       // Added
	ImagePrimaryColor string `json:"ImagePrimaryColor"` // Added
	SourceTitle       string `json:"SourceTitle"`       // Added for sync
	SourceURL         string `json:"SourceURL"`         // Added for sync
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
	Summary           string
	WordCount         int
	ReadingTime       int
	CoverImage        string
	Author            string
	CleanContent      string
	Content           string // Original content
	ContentHash       string
	ImageCaption      string // Added
	ImageCredit       string // Added
	ImagePrimaryColor string // Added
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
