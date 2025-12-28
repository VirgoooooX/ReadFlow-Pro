import AsyncStorage from '@react-native-async-storage/async-storage';
import AvatarStorageService from './AvatarStorageService';
import { logger } from './rss/RSSUtils';
import { SettingsService } from './SettingsService';

export interface User {
  id: string;
  username: string;
  email: string;
  avatar?: string;
  bio?: string;
  phone?: string;
  location?: string;
  createdAt: string;
  lastLoginAt: string;
}

export interface LoginCredentials {
  email: string;
  password: string;
}

export interface RegisterData {
  username: string;
  email: string;
  password: string;
}

export interface AuthResponse {
  success: boolean;
  user?: User;
  token?: string;
  message?: string;
}

export class AuthService {
  private static instance: AuthService;
  private currentUser: User | null = null;
  private authToken: string | null = null;
  private initialized = false;

  private constructor() {
  }

  public static getInstance(): AuthService {
    if (!AuthService.instance) {
      AuthService.instance = new AuthService();
    }
    return AuthService.instance;
  }

  public isInitialized(): boolean {
    return this.initialized;
  }

  /**
   * 初始化认证服务，检查本地存储的登录状态
   */
  public async initialize(): Promise<void> {
    try {
      const token = await AsyncStorage.getItem('auth_token');
      const userStr = await AsyncStorage.getItem('current_user');

      if (token && userStr) {
        this.authToken = token;
        this.currentUser = JSON.parse(userStr);

        if (this.currentUser) {
          // 验证token是否仍然有效
          // TODO: Call server to validate token
          const isValid = true; 
          if (isValid) {
            // 加载用户头像
            const avatarPath = await AvatarStorageService.getAvatarPath(this.currentUser.id);
            if (avatarPath && avatarPath !== this.currentUser.avatar) {
              this.currentUser.avatar = avatarPath;
              await AsyncStorage.setItem('current_user', JSON.stringify(this.currentUser));
            }
          } else {
            await this.logout();
          }
        }
      }
    } catch (error) {
      logger.error('初始化认证服务失败:', error);
      await this.logout();
    } finally {
      this.initialized = true;
    }
  }

  private async getApiUrl(): Promise<string> {
      const config = await SettingsService.getInstance().getProxyModeConfig();
      // Default to the address requested by user if not set
      return config.serverUrl || 'http://192.168.31.27:8080';
  }

  /**
   * 用户登录
   */
  public async login(credentials: LoginCredentials): Promise<AuthResponse> {
    try {
      const apiUrl = await this.getApiUrl();
      // Use email as username for now as server only has username
      const username = credentials.email.includes('@') ? credentials.email.split('@')[0] : credentials.email;

      const response = await fetch(`${apiUrl}/api/auth/login`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          username: credentials.email, // Passing full email/username string as username
          password: credentials.password,
        }),
      });

      logger.info(`[AuthService] 登录响应状态: ${response.status}`);

      if (!response.ok) {
          const errorText = await response.text();
          logger.error(`[AuthService] 登录失败 (HTTP ${response.status}): ${errorText}`);
          return {
              success: false,
              message: `登录失败: HTTP ${response.status}`
          };
      }

      const data = await response.json();
      logger.info(`[AuthService] 登录结果:`, { success: data.success, user_id: data.user_id });

      if (data.success && data.token) {
        const user: User = {
            id: data.user_id ? data.user_id.toString() : '0',
            username: credentials.email,
            email: credentials.email,
            createdAt: new Date().toISOString(),
            lastLoginAt: new Date().toISOString(),
        };

        this.currentUser = user;
        this.authToken = data.token;

        // 保存到本地存储
        await AsyncStorage.setItem('auth_token', data.token);
        await AsyncStorage.setItem('current_user', JSON.stringify(user));

        // Update Proxy Config to enable RSS sync
        await SettingsService.getInstance().saveProxyModeConfig({
            enabled: true,
            serverUrl: apiUrl,
            serverPassword: '', 
            token: data.token,
            userId: data.user_id
        });

        return {
            success: true,
            user,
            token: data.token,
            message: '登录成功'
        };
      } else {
          return {
              success: false,
              message: data.message || '登录失败'
          };
      }
    } catch (error) {
      logger.error('登录失败:', error);
      return {
        success: false,
        message: '登录过程中出现错误，请检查网络连接'
      };
    }
  }

  /**
   * 用户注册
   */
  public async register(data: RegisterData): Promise<AuthResponse> {
    try {
      const apiUrl = await this.getApiUrl();
      logger.info(`[AuthService] 正在注册用户: ${data.username} 到 ${apiUrl}`);
      const response = await fetch(`${apiUrl}/api/auth/register`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          username: data.username,
          password: data.password,
          email: data.email
        }),
      });

      logger.info(`[AuthService] 注册响应状态: ${response.status}`);
      
      if (!response.ok) {
        const errorText = await response.text();
        logger.error(`[AuthService] 注册失败 (HTTP ${response.status}): ${errorText}`);
        return {
          success: false,
          message: `注册失败: HTTP ${response.status}`
        };
      }

      const result = await response.json();
      logger.info(`[AuthService] 注册结果:`, result);
      
      return {
          success: result.success,
          message: result.message || (result.success ? '注册成功' : '注册失败')
      };
    } catch (error) {
      logger.error('注册失败:', error);
      return {
        success: false,
        message: '注册过程中出现错误，请检查网络连接'
      };
    }
  }

  /**
   * 用户登出
   */
  public async logout(): Promise<void> {
    try {
      // 清除本地数据
      this.currentUser = null;
      this.authToken = null;

      await AsyncStorage.multiRemove(['auth_token', 'current_user']);
      
      // Clear proxy config token
      const config = await SettingsService.getInstance().getProxyModeConfig();
      if (config.token) {
          await SettingsService.getInstance().saveProxyModeConfig({
              ...config,
              token: undefined,
              userId: undefined
          });
      }
    } catch (error) {
      logger.error('登出失败:', error);
    }
  }

  /**
   * 更新用户信息
   */
  public async updateProfile(updates: Partial<User>): Promise<AuthResponse> {
      if (!this.currentUser) return { success: false, message: 'Not logged in' };
      
      this.currentUser = { ...this.currentUser, ...updates };
      await AsyncStorage.setItem('current_user', JSON.stringify(this.currentUser));
      
      return { success: true, user: this.currentUser };
  }

  /**
   * 同步用户配置到服务端
   */
  public async syncUserProfile(settings: {
      readingSettings?: any;
      translationProvider?: string;
      enableAutoTranslation?: boolean;
      enableTitleTranslation?: boolean;
      maxConcurrentTranslations?: number;
      translationTimeout?: number;
      defaultCategory?: string;
      enableNotifications?: boolean;
      proxyModeEnabled?: boolean;
      proxyServerUrl?: string;
      proxyToken?: string;
  }): Promise<void> {
      if (!this.currentUser || !this.authToken) return;
      
      try {
          const apiUrl = await this.getApiUrl();
          // Convert camelCase to snake_case for server
          const payload: any = {};
          if (settings.readingSettings) payload.reading_settings = JSON.stringify(settings.readingSettings);
          if (settings.translationProvider) payload.translation_provider = settings.translationProvider;
          if (settings.enableAutoTranslation !== undefined) payload.enable_auto_translation = settings.enableAutoTranslation;
          if (settings.enableTitleTranslation !== undefined) payload.enable_title_translation = settings.enableTitleTranslation;
          if (settings.maxConcurrentTranslations) payload.max_concurrent_translations = settings.maxConcurrentTranslations;
          if (settings.translationTimeout) payload.translation_timeout = settings.translationTimeout;
          if (settings.defaultCategory) payload.default_category = settings.defaultCategory;
          if (settings.enableNotifications !== undefined) payload.enable_notifications = settings.enableNotifications;
          if (settings.proxyModeEnabled !== undefined) payload.proxy_mode_enabled = settings.proxyModeEnabled;
          if (settings.proxyServerUrl) payload.proxy_server_url = settings.proxyServerUrl;
          if (settings.proxyToken) payload.proxy_token = settings.proxyToken;

          const response = await fetch(`${apiUrl}/api/user/profile`, {
              method: 'POST',
              headers: {
                  'Content-Type': 'application/json',
                  'Authorization': `Bearer ${this.authToken}`
              },
              body: JSON.stringify(payload)
          });
          
          if (!response.ok) {
              logger.warn(`[AuthService] Failed to sync profile: ${response.status}`);
          } else {
              logger.info(`[AuthService] Profile synced successfully`);
          }
      } catch (error) {
          logger.error(`[AuthService] Error syncing profile:`, error);
      }
  }

  /**
   * 修改密码
   */
  public async changePassword(oldPassword: string, newPassword: string): Promise<AuthResponse> {
      // TODO: Implement server side change password
      return { success: false, message: '修改密码功能暂未开放' };
  }

  /**
   * 获取当前用户
   */
  public getCurrentUser(): User | null {
    return this.currentUser;
  }

  /**
   * 获取认证token
   */
  public getAuthToken(): string | null {
    return this.authToken;
  }

  /**
   * 检查是否已登录
   */
  public isAuthenticated(): boolean {
    return this.currentUser !== null && this.authToken !== null;
  }
}

export default AuthService.getInstance();
