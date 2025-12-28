package api

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/readflow/gateway/internal/config"
	"github.com/readflow/gateway/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// AuthService 认证服务
type AuthService struct {
	db     *db.DB
	config *config.Config
}

// NewAuthService 创建认证服务
func NewAuthService(database *db.DB, cfg *config.Config) *AuthService {
	return &AuthService{
		db:     database,
		config: cfg,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Password string `json:"password" binding:"required"`
	Username string `json:"username" binding:"required"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email"` // 可选
}

// LoginResponse 登录响应
type LoginResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	UserID  int64  `json:"user_id,omitempty"`
	Message string `json:"message,omitempty"`
}

// UpdateProfileRequest 更新用户资料请求
type UpdateProfileRequest struct {
	ReadingSettings           *string `json:"reading_settings"`
	TranslationProvider       *string `json:"translation_provider"`
	EnableAutoTranslation     *bool   `json:"enable_auto_translation"`
	EnableTitleTranslation    *bool   `json:"enable_title_translation"`
	MaxConcurrentTranslations *int    `json:"max_concurrent_translations"`
	TranslationTimeout        *int    `json:"translation_timeout"`
	DefaultCategory           *string `json:"default_category"`
	EnableNotifications       *bool   `json:"enable_notifications"`
	ProxyModeEnabled          *bool   `json:"proxy_mode_enabled"`
	ProxyServerURL            *string `json:"proxy_server_url"`
	ProxyToken                *string `json:"proxy_token"`
}

// Claims JWT 声明
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Register 用户注册
func (a *AuthService) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[AUTH] Invalid register request: %v", err)
		c.JSON(http.StatusBadRequest, LoginResponse{
			Success: false,
			Message: "无效的请求参数",
		})
		return
	}

	log.Printf("[AUTH] Registering user: username=%s, email=%s", req.Username, req.Email)

	// 哈希密码
	hashedPassword, err := a.HashPassword(req.Password)
	if err != nil {
		log.Printf("[AUTH] Password hashing failed: %v", err)
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Success: false,
			Message: "密码处理失败",
		})
		return
	}

	// 创建用户
	user, err := a.db.CreateUser(req.Username, req.Email, hashedPassword)
	if err != nil {
		log.Printf("[AUTH] Create user failed: %v", err)
		c.JSON(http.StatusBadRequest, LoginResponse{
			Success: false,
			Message: "用户名或邮箱已存在",
		})
		return
	}

	log.Printf("[AUTH] User registered successfully: id=%d", user.ID)

	c.JSON(http.StatusOK, LoginResponse{
		Success: true,
		UserID:  user.ID,
		Message: "注册成功",
	})
}

// Login 用户登录
func (a *AuthService) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[AUTH] Invalid login request: %v", err)
		c.JSON(http.StatusBadRequest, LoginResponse{
			Success: false,
			Message: "无效的请求参数",
		})
		return
	}

	log.Printf("[AUTH] Login attempt for: %s", req.Username)

	// 尝试通过用户名获取用户
	user, err := a.db.GetUserByUsername(req.Username)
	if err != nil {
		// 如果用户名找不到，尝试通过邮箱获取
		log.Printf("[AUTH] User not found by username, trying email: %s", req.Username)
		user, err = a.db.GetUserByEmail(req.Username)
		if err != nil {
			log.Printf("[AUTH] User not found by username or email: %s", req.Username)
			c.JSON(http.StatusUnauthorized, LoginResponse{
				Success: false,
				Message: "用户名或密码错误",
			})
			return
		}
	}

	// 验证密码
	if !a.CheckPasswordHash(req.Password, user.PasswordHash) {
		log.Printf("[AUTH] Password mismatch for user: %s", user.Username)
		// 兼容旧的全局密码模式 (如果用户没有设置密码哈希，且输入的是全局密码)
		if user.PasswordHash == "" && req.Password == a.config.ServerPassword {
			log.Printf("[AUTH] Legacy password matched for user: %s", user.Username)
		} else {
			c.JSON(http.StatusUnauthorized, LoginResponse{
				Success: false,
				Message: "用户名或密码错误",
			})
			return
		}
	}

	// 生成 JWT Token
	token, err := a.GenerateToken(user.ID, user.Username)
	if err != nil {
		log.Printf("[AUTH] Token generation failed: %v", err)
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Success: false,
			Message: "生成 Token 失败",
		})
		return
	}

	// 更新用户 Token
	if err := a.db.UpdateUserToken(user.ID, token); err != nil {
		log.Printf("[AUTH] Update token failed: %v", err)
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Success: false,
			Message: "更新 Token 失败",
		})
		return
	}

	log.Printf("[AUTH] User logged in successfully: id=%d, username=%s", user.ID, user.Username)

	c.JSON(http.StatusOK, LoginResponse{
		Success: true,
		Token:   token,
		UserID:  user.ID,
	})
}

// UpdateProfile 更新用户资料
func (a *AuthService) UpdateProfile(c *gin.Context) {
	userID, err := GetCurrentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未授权",
		})
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	// 获取当前配置，如果不存在则使用默认值
	pref, err := a.db.GetUserPreferences(userID)
	if err != nil {
		// 假设不存在，使用默认值
		pref = &db.UserPreference{
			UserID: userID,
		}
	}

	// 更新字段
	if req.ReadingSettings != nil {
		pref.ReadingSettings = *req.ReadingSettings
	}
	if req.TranslationProvider != nil {
		pref.TranslationProvider = *req.TranslationProvider
	}
	if req.EnableAutoTranslation != nil {
		pref.EnableAutoTranslation = *req.EnableAutoTranslation
	}
	if req.EnableTitleTranslation != nil {
		pref.EnableTitleTranslation = *req.EnableTitleTranslation
	}
	if req.MaxConcurrentTranslations != nil {
		pref.MaxConcurrentTranslations = *req.MaxConcurrentTranslations
	}
	if req.TranslationTimeout != nil {
		pref.TranslationTimeout = *req.TranslationTimeout
	}
	if req.DefaultCategory != nil {
		pref.DefaultCategory = *req.DefaultCategory
	}
	if req.EnableNotifications != nil {
		pref.EnableNotifications = *req.EnableNotifications
	}
	if req.ProxyModeEnabled != nil {
		pref.ProxyModeEnabled = *req.ProxyModeEnabled
	}
	if req.ProxyServerURL != nil {
		pref.ProxyServerURL = *req.ProxyServerURL
	}
	if req.ProxyToken != nil {
		pref.ProxyToken = *req.ProxyToken
	}

	if err := a.db.UpsertUserPreferences(pref); err != nil {
		log.Printf("[AUTH] Failed to update user preferences: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "更新配置失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "配置已更新",
	})
}

// HashPassword 生成密码哈希
func (a *AuthService) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPasswordHash 验证密码哈希
func (a *AuthService) CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken 生成 JWT Token
func (a *AuthService) GenerateToken(userID int64, username string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)), // 30 天有效期
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.config.JWTSecret))
}

// ValidateToken 验证 Token
func (a *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// AuthMiddleware 认证中间件
func (a *AuthService) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "缺少认证信息",
			})
			c.Abort()
			return
		}

		// 提取 Token (Bearer <token>)
		tokenString := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}

		// 验证 Token
		claims, err := a.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "无效的认证信息",
			})
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

		c.Next()
	}
}

// GetCurrentUserID 从上下文获取当前用户 ID
func GetCurrentUserID(c *gin.Context) (int64, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, fmt.Errorf("user not found in context")
	}
	return userID.(int64), nil
}
