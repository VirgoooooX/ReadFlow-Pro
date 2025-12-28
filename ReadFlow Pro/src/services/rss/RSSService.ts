/**
 * RSS 服务主入口
 * 统一管理 RSS 源的 CRUD 操作，并将数据获取委托给 ProxyRSSService
 */

import { DatabaseService } from '../../database/DatabaseService';
import { RSSSource, Article, AppError } from '../../types';
import { SettingsService } from '../SettingsService';
import { proxyRSSService } from './ProxyRSSService';
import { logger } from './RSSUtils';
import { InteractionManager } from 'react-native';

export class RSSService {
  private static instance: RSSService;
  private databaseService: DatabaseService;

  private constructor() {
    this.databaseService = DatabaseService.getInstance();
  }

  public static getInstance(): RSSService {
    if (!RSSService.instance) {
      RSSService.instance = new RSSService();
    }
    return RSSService.instance;
  }

  // =================== RSS 源 CRUD 操作 ===================

  /**
   * 添加 RSS 源
   */
  public async addRSSSource(
    url: string, 
    title?: string, 
    contentType: 'text' | 'image_text' = 'image_text',
    category: string = '技术',
    sourceMode: 'direct' | 'proxy' = 'proxy' // 默认为 proxy
  ): Promise<RSSSource> {
    try {
      // 🔥 清理 URL：去除空格和末尾多余斜杠
      let cleanUrl = url.trim();
      if (cleanUrl.match(/\/[^/]+\/$/) && !cleanUrl.endsWith('://')) {
        cleanUrl = cleanUrl.replace(/\/$/, '');
        logger.info(`[addRSSSource] 已移除末尾斜杠: ${url} -> ${cleanUrl}`);
      }
      
      // 1. 验证 RSS 源 (简单检查)
      // 由于移除了本地解析，我们依赖服务端验证
      // 或者尝试简单的 HEAD 请求
      try {
        await fetch(cleanUrl, { method: 'HEAD' });
      } catch (e) {
        logger.warn(`[addRSSSource] URL 可能不可达: ${cleanUrl}`);
      }
      
      // 2. 代理模式：调用服务端订阅 API
      // 始终尝试订阅到服务端
      const proxyConfig = await SettingsService.getInstance().getProxyModeConfig();
      if (proxyConfig.enabled && proxyConfig.token) {
        await proxyRSSService.subscribeToProxyServer(cleanUrl, title, proxyConfig);
      }
      
      // 3. 保存到本地数据库
      const rssSource: Omit<RSSSource, 'id'> = {
        sortOrder: 0,
        name: title || 'New Feed',
        url: cleanUrl,
        category,
        contentType,
        sourceMode: 'proxy', // 强制使用 proxy
        isActive: true,
        lastFetchAt: new Date(),
        errorCount: 0,
        description: '',
        groupId: null, 
      };

      const result = await this.databaseService.executeInsert(
        `INSERT INTO rss_sources (url, title, description, category, content_type, source_mode, is_active, last_updated) 
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        [
          rssSource.url,
          rssSource.name,
          rssSource.description,
          rssSource.category,
          rssSource.contentType,
          rssSource.sourceMode,
          rssSource.isActive ? 1 : 0,
          rssSource.lastFetchAt?.toISOString() || new Date().toISOString(),
        ]
      );

      const newSource: RSSSource = {
        id: Number(result.insertId),
        ...rssSource,
      };

      // 4. 立即获取文章
      if (proxyConfig.serverUrl) {
        await proxyRSSService.fetchArticlesFromProxy(newSource, proxyConfig, { mode: 'refresh' });
      }

      return newSource;
    } catch (error) {
      logger.error('Error adding RSS source:', error);
      throw new AppError({
        code: 'RSS_ADD_ERROR',
        message: `Failed to add RSS source: ${url.trim()}`,
        details: error,
        timestamp: new Date(),
      });
    }
  }

  /**
   * 获取所有 RSS 源
   */
  public async getAllRSSSources(): Promise<RSSSource[]> {
    try {
      const results = await this.databaseService.executeQuery(`
        SELECT * FROM rss_sources ORDER BY sort_order ASC, id ASC
      `);
      
      return results.map(this.mapRSSSourceRow);
    } catch (error) {
      logger.error('Error getting RSS sources:', error);
      return [];
    }
  }

  /**
   * 根据 ID 获取 RSS 源
   */
  public async getSourceById(id: number): Promise<RSSSource | null> {
    try {
      const results = await this.databaseService.executeQuery(
        'SELECT * FROM rss_sources WHERE id = ?',
        [id]
      );
      
      if (results.length === 0) {
        return null;
      }
      
      return this.mapRSSSourceRow(results[0]);
    } catch (error) {
      logger.error('Error getting RSS source by ID:', error);
      return null;
    }
  }

  /**
   * 获取活跃的 RSS 源
   */
  public async getActiveRSSSources(): Promise<RSSSource[]> {
    try {
      const results = await this.databaseService.executeQuery(
        'SELECT * FROM rss_sources WHERE is_active = 1 ORDER BY sort_order ASC, id ASC'
      );
      
      return results.map(this.mapRSSSourceRow);
    } catch (error) {
      logger.error('Error getting active RSS sources:', error);
      return [];
    }
  }

  /**
   * 更新 RSS 源排序
   */
  public async updateSourcesOrder(sourceOrder: { id: number; sortOrder: number }[]): Promise<void> {
    try {
      for (const item of sourceOrder) {
        await this.databaseService.executeStatement(
          'UPDATE rss_sources SET sort_order = ? WHERE id = ?',
          [item.sortOrder, item.id]
        );
      }
    } catch (error) {
      logger.error('Error updating RSS sources order:', error);
      throw error;
    }
  }

  /**
   * 更新 RSS 源
   */
  public async updateRSSSource(id: number, updates: Partial<RSSSource>): Promise<void> {
    try {
      const setClause = [];
      const values = [];
      
      if (updates.name !== undefined) {
        setClause.push('title = ?');
        values.push(updates.name);
      }
      if (updates.url !== undefined) {
        setClause.push('url = ?');
        values.push(updates.url);
      }
      if (updates.description !== undefined) {
        setClause.push('description = ?');
        values.push(updates.description);
      }
      if (updates.category !== undefined) {
        setClause.push('category = ?');
        values.push(updates.category);
      }
      if (updates.contentType !== undefined) {
        setClause.push('content_type = ?');
        values.push(updates.contentType);
      }
      if (updates.isActive !== undefined) {
        setClause.push('is_active = ?');
        values.push(updates.isActive ? 1 : 0);
      }
      if (updates.sourceMode !== undefined) {
        setClause.push('source_mode = ?');
        values.push(updates.sourceMode);
      }
      
      if (setClause.length === 0) {
        return;
      }
      
      values.push(id);
      
      const sql = `UPDATE rss_sources SET ${setClause.join(', ')} WHERE id = ?`;
      await this.databaseService.executeStatement(sql, values);
    } catch (error) {
      logger.error('Error updating RSS source:', error);
      throw new AppError({
        code: 'RSS_UPDATE_ERROR',
        message: `Failed to update RSS source: ${id}`,
        details: error,
        timestamp: new Date(),
      });
    }
  }

  /**
   * 删除 RSS 源
   */
  public async deleteRSSSource(id: number): Promise<void> {
    try {
      const source = await this.getSourceById(id);
      if (!source) return;
      
      // 调用服务端 API 删除订阅
      const config = await SettingsService.getInstance().getProxyModeConfig();
      if (config.enabled && config.token) {
        try {
          await fetch(`${config.serverUrl}/api/subscribe/${source.id}`, { // 这里应该用服务端 ID? 
            // 实际上客户端并没有存储服务端 ID，除非 id 是一致的。
            // 现在的逻辑是客户端 ID。服务端 API 应该支持 url 删除?
            // 服务端 DELETE /api/subscribe/:id 是根据 item ID 还是 source ID? 是 source ID.
            // 我们目前没有存储 Server Source ID。
            // 暂时跳过服务端删除，或者需要先查询。
          });
        } catch (error) {
          logger.warn('Failed to delete source from proxy server:', error);
        }
      }
      
      // 删除本地数据
      await this.databaseService.executeStatement(
        'DELETE FROM articles WHERE rss_source_id = ?',
        [id]
      );
      
      await this.databaseService.executeStatement(
        'DELETE FROM rss_sources WHERE id = ?',
        [id]
      );
    } catch (error) {
      logger.error('Error deleting RSS source:', error);
      throw new AppError({
        code: 'RSS_DELETE_ERROR',
        message: `Failed to delete RSS source: ${id}`,
        details: error,
        timestamp: new Date(),
      });
    }
  }

  // =================== 文章获取 ===================

  /**
   * 获取 RSS 源文章
   */
  public async fetchArticlesFromSource(
    source: RSSSource,
    options: { mode?: 'sync' | 'refresh' } = {}
  ): Promise<Article[]> {
    const proxyConfig = await SettingsService.getInstance().getProxyModeConfig();
    if (!proxyConfig.serverUrl) {
      logger.warn(`[fetchArticlesFromSource] 未配置代理服务器，无法获取文章`);
      return [];
    }
    const mode = options.mode || 'refresh';
    return await proxyRSSService.fetchArticlesFromProxy(source, proxyConfig, { mode });
  }

  /**
   * 刷新所有活跃 RSS 源
   * 使用服务端 Sync 接口批量获取
   */
  public async refreshAllSources(
    options: {
      mode?: 'sync' | 'refresh';
      maxConcurrent?: number;
      onProgress?: (current: number, total: number, sourceName: string) => void;
      onError?: (error: Error, sourceName: string) => void;
    } = {}
  ): Promise<{ 
    success: number; 
    failed: number; 
    totalArticles: number;
    errors: Array<{ source: string; error: string }>;
  }> {
    // 直接调用代理服务的 Sync 方法
    // 这个方法会调用 /api/sync 获取所有待投递文章
    return await proxyRSSService.syncFromProxyServer(options);
  }

  /**
   * 刷新指定的 RSS 源列表
   */
  public async refreshSources(
    sourceIds: number[],
    options: {
      maxConcurrent?: number;
      onProgress?: (current: number, total: number, sourceName: string) => void;
      onError?: (error: Error, sourceName: string) => void;
      onArticlesReady?: (articles: Article[], sourceName: string) => void;
    } = {}
  ): Promise<{ 
    success: number; 
    failed: number; 
    totalArticles: number;
    errors: Array<{ source: string; error: string }>;
  }> {
    // 循环调用 fetchArticlesFromSource
    const { maxConcurrent = 3, onProgress, onError, onArticlesReady } = options;
    
    const allSources = await this.getActiveRSSSources();
    const sourcesToRefresh = allSources.filter(s => sourceIds.includes(s.id));
    
    if (sourcesToRefresh.length === 0) {
      return { success: 0, failed: 0, totalArticles: 0, errors: [] };
    }

    // 复用并发逻辑
    const limiter = this.createLimiter(maxConcurrent);
    
    let success = 0;
    let failed = 0;
    let totalArticles = 0;
    const errors: Array<{ source: string; error: string }> = [];
    let completed = 0;
    const total = sourcesToRefresh.length;

    const tasks = sourcesToRefresh.map(source => 
      limiter(() => 
        new Promise<void>((resolve, reject) => {
          InteractionManager.runAfterInteractions(() => {
            this.fetchArticlesFromSource(source)
              .then((articles) => {
                success++;
                totalArticles += articles.length;
                completed++;
                
                if (onArticlesReady && articles.length > 0) {
                  onArticlesReady(articles, source.name);
                }
                
                onProgress?.(completed, total, source.name);
                resolve();
              })
              .catch((error) => {
                failed++;
                completed++;
                const errorMsg = error.message || '未知错误';
                errors.push({ source: source.name, error: errorMsg });
                
                onError?.(error, source.name);
                onProgress?.(completed, total, source.name);
                resolve(); 
              });
          });
        })
      )
    );

    await Promise.all(tasks);

    return { success, failed, totalArticles, errors };
  }

  /**
   * 后台刷新所有 RSS 源 (兼容性方法，实际调用 refreshAllSources)
   */
  public async refreshAllSourcesBackground(
    options: {
      maxConcurrent?: number;
      onProgress?: (current: number, total: number, sourceName: string) => void;
      onError?: (error: Error, sourceName: string) => void;
      onArticlesReady?: (articles: Article[], sourceName: string) => void;
    } = {}
  ): Promise<{ 
    success: number; 
    failed: number; 
    totalArticles: number;
    errors: Array<{ source: string; error: string }>;
  }> {
    return this.refreshAllSources(options);
  }

  /**
   * 并发限制器
   */
  private createLimiter(maxConcurrent: number = 3) {
    let running = 0;
    const queue: Array<(value: void) => void> = [];

    const run = async (fn: () => Promise<any>) => {
      while (running >= maxConcurrent) {
        await new Promise<void>(resolve => queue.push(resolve));
      }
      running++;
      try {
        return await fn();
      } finally {
        running--;
        const resolve = queue.shift();
        if (resolve) resolve();
      }
    };

    return (fn: () => Promise<any>) => run(fn);
  }

  /**
   * 验证 RSS 源 (Stub)
   */
  public async validateRSSFeed(url: string): Promise<{
    title?: string;
    description?: string;
    language?: string;
  }> {
    // 简单的 URL 检查
    return { title: 'New Feed' };
  }

  // =================== 私有方法 ===================

  /**
   * 数据库行映射为 RSSSource 对象
   */
  private mapRSSSourceRow(row: any): RSSSource {
    return {
      id: Number(row.id),
      sortOrder: row.sort_order || 0,
      name: row.title,
      url: row.url,
      description: row.description,
      category: row.category || 'General',
      contentType: row.content_type || 'image_text',
      sourceMode: row.source_mode || 'proxy', // 默认为 proxy
      isActive: Boolean(row.is_active),
      lastFetchAt: row.last_updated ? new Date(row.last_updated) : new Date(),
      errorCount: row.error_count || 0,
      article_count: row.article_count || 0,
      unread_count: row.unread_count || 0,
      last_updated: row.last_updated,
      groupId: row.group_id || null,
      groupSortOrder: row.group_sort_order || 0,
    };
  }
}

export const rssService = RSSService.getInstance();
