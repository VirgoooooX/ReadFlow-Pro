package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/readflow/gateway/internal/config"
	"github.com/readflow/gateway/internal/db"
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

// LoginResponse 登录响应
type LoginResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	UserID  int64  `json:"user_id,omitempty"`
	Message string `json:"message,omitempty"`
}

// Claims JWT 声明
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Login 用户登录
func (a *AuthService) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, LoginResponse{
			Success: false,
			Message: "无效的请求参数",
		})
		return
	}

	// 验证全局密码
	if req.Password != a.config.ServerPassword {
		c.JSON(http.StatusUnauthorized, LoginResponse{
			Success: false,
			Message: "密码错误",
		})
		return
	}

	// 创建或获取用户
	user, err := a.db.CreateOrGetUser(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Success: false,
			Message: "服务器错误",
		})
		return
	}

	// 生成 JWT Token
	token, err := a.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Success: false,
			Message: "生成 Token 失败",
		})
		return
	}

	// 更新用户 Token
	if err := a.db.UpdateUserToken(user.ID, token); err != nil {
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Success: false,
			Message: "更新 Token 失败",
		})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Success: true,
		Token:   token,
		UserID:  user.ID,
	})
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
