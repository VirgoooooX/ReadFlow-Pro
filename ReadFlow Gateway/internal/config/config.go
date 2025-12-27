package config

import (
	"os"
	"strconv"
)

// Config 应用配置
type Config struct {
	// 数据库配置
	DBPath string

	// 静态文件目录
	StaticDir string

	// RSS 抓取间隔（秒）
	FetchInterval int

	// 图片处理配置
	ImageMaxWidth  int
	ImageQuality   int
	ImageConcurrent int

	// 服务器配置
	ServerPort     string
	ServerPassword string

	// JWT 配置
	JWTSecret string

	// 日志级别
	LogLevel string
}

// Load 从环境变量加载配置
func Load() *Config {
	return &Config{
		DBPath:          getEnv("DB_PATH", "/app/data/readflow.db"),
		StaticDir:       getEnv("STATIC_DIR", "/app/static"),
		FetchInterval:   getEnvInt("FETCH_INTERVAL", 900),
		ImageMaxWidth:   getEnvInt("IMAGE_MAX_WIDTH", 1080),
		ImageQuality:    getEnvInt("IMAGE_QUALITY", 75),
		ImageConcurrent: getEnvInt("IMAGE_CONCURRENT", 2),
		ServerPort:      getEnv("SERVER_PORT", "8080"),
		ServerPassword:  getEnv("SERVER_PASSWORD", "change_me_in_production"),
		JWTSecret:       getEnv("JWT_SECRET", "your_jwt_secret_key_change_in_production"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
	}
}

// getEnv 获取环境变量，如果不存在则使用默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvInt 获取整数类型的环境变量
func getEnvInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
