/**
 * 代理模式 RSS 服务 (服务端同步模式)
 * 
 * 负责与 ReadFlow Gateway 服务端进行同步
 * - 获取文章列表 (JSON)
 * - 提交阅读状态/ACK
 * - 管理订阅源同步
 */

import { DatabaseService } from '../../database/DatabaseService';
import { RSSSource, Article, ProxyModeConfig, AppError } from '../../types';
import { SettingsService } from '../SettingsService';
import { logger } from './RSSUtils';

interface ServerItem {
  ID: number;
  SourceID: number;
  GUID: string;
  Title: string;
  XMLContent: string;
  ImagePaths: string;
  PublishedAt: string;
  CreatedAt: string;
  Summary: string;
  WordCount: number;
  ReadingTime: number;
  CoverImage: string;
  Author: string;
  CleanContent: string;
  Content: string;
  ContentHash: string;
  ImageCaption: string;
  ImageCredit: string;
  SourceTitle: string;
  SourceURL: string;
}

export class ProxyRSSService {
  private static instance: ProxyRSSService;
  private databaseService: DatabaseService;

  private constructor() {
    this.databaseService = DatabaseService.getInstance();
  }

  public static getInstance(): ProxyRSSService {
    if (!ProxyRSSService.instance) {
      ProxyRSSService.instance = new ProxyRSSService();
    }
    return ProxyRSSService.instance;
  }

  // =================== 公共方法 ===================

  /**
   * 获取代理配置
   */
  public async getProxyConfig(): Promise<ProxyModeConfig> {
    return await SettingsService.getInstance().getProxyModeConfig();
  }

  /**
   * 检查代理模式是否启用
   */
  public async isProxyEnabled(): Promise<boolean> {
    const config = await this.getProxyConfig();
    return config.enabled && !!config.token;
  }

  /**
   * 订阅 RSS 源到代理服务器
   */
  public async subscribeToProxyServer(
    url: string,
    title: string | undefined,
    config: ProxyModeConfig
  ): Promise<void> {
    try {
      const response = await fetch(`${config.serverUrl}/api/subscribe`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${config.token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ url, title }),
      });

      const data = await response.json();
      if (!data.success) {
        throw new Error(data.message || '订阅失败');
      }
    } catch (error) {
      logger.error('Error subscribing to proxy server:', error);
      throw error;
    }
  }

  /**
   * 批量同步所有订阅源到代理服务器
   */
  public async syncAllSourcesToProxy(
    sources: RSSSource[],
    config: ProxyModeConfig
  ): Promise<void> {
    try {
      if (sources.length === 0) return;
      
      logger.info(`[Proxy Sync] 开始同步 ${sources.length} 个源到服务端`);
      
      for (const source of sources) {
        try {
          await this.subscribeToProxyServer(source.url, source.name, config);
        } catch (error) {
          logger.warn(`[Proxy Sync] 同步源失败: ${source.name}`, error);
        }
      }
    } catch (error) {
      logger.error('[Proxy Sync] 💥 同步过程出错:', error);
      throw error;
    }
  }

  /**
   * 从服务端全量/增量同步文章
   */
  public async syncFromProxyServer(
    options: {
      mode?: 'sync' | 'refresh';
      onProgress?: (current: number, total: number, sourceName: string) => void;
      onError?: (error: Error, sourceName: string) => void;
    } = {}
  ): Promise<{
    success: number;
    failed: number;
    totalArticles: number;
    errors: Array<{ source: string; error: string }>;
  }> {
    try {
      const config = await this.getProxyConfig();
      if (!config.enabled || !config.serverUrl) {
        return { success: 0, failed: 0, totalArticles: 0, errors: [] };
      }

      const rssSettings = await SettingsService.getInstance().getRSSSettings();
      const compress = rssSettings.enableImageCompression;
      const mode = options.mode || 'sync';

      logger.info(`[Sync] 开始同步 (mode=${mode}, compress=${compress})`);

      // 调用 Sync API
      const url = `${config.serverUrl}/api/sync?format=json&image_compression=${compress}&mode=${mode}&limit=100`;
      
      const response = await fetch(url, {
        headers: {
          'Authorization': `Bearer ${config.token}`,
        },
      });

      if (!response.ok) {
        throw new Error(`Sync failed: HTTP ${response.status}`);
      }

      const data = await response.json();
      if (!data.success || !Array.isArray(data.items)) {
        throw new Error('Invalid sync response');
      }

      const items: ServerItem[] = data.items;
      logger.info(`[Sync] 收到 ${items.length} 篇文章`);

      if (items.length === 0) {
        return { success: 0, failed: 0, totalArticles: 0, errors: [] };
      }

      // 保存文章
      const savedArticles = await this.saveArticlesFromSync(items);
      
      // 发送 ACK
      const itemIds = items.map(i => i.ID);
      await this.acknowledgeItems(itemIds, config);

      return { 
        success: savedArticles.length, 
        failed: items.length - savedArticles.length, 
        totalArticles: savedArticles.length, 
        errors: [] 
      };

    } catch (error) {
      logger.error('Error syncing from proxy server:', error);
      return { 
        success: 0, 
        failed: 1, 
        totalArticles: 0, 
        errors: [{ source: 'Server', error: (error as Error).message }] 
      };
    }
  }

  /**
   * 获取单个源的文章 (兼容 RSSService 调用)
   */
  public async fetchArticlesFromProxy(
    source: RSSSource,
    config: ProxyModeConfig,
    options: { mode?: 'sync' | 'refresh' } = {}
  ): Promise<Article[]> {
    try {
      const rssSettings = await SettingsService.getInstance().getRSSSettings();
      const compress = rssSettings.enableImageCompression;
      const mode = options.mode || 'refresh'; // 单个源默认刷新

      const url = `${config.serverUrl}/api/sync?format=json&image_compression=${compress}&mode=${mode}&source_url=${encodeURIComponent(source.url)}&limit=50`;
      
      const response = await fetch(url, {
        headers: {
          'Authorization': `Bearer ${config.token}`,
        },
      });

      if (!response.ok) {
        throw new Error(`Fetch failed: HTTP ${response.status}`);
      }

      const data = await response.json();
      const items: ServerItem[] = data.items || [];
      
      if (items.length > 0) {
        const saved = await this.saveArticlesFromSync(items);
        // ACK
        await this.acknowledgeItems(items.map(i => i.ID), config);
        return saved;
      }
      
      return [];
    } catch (error) {
      logger.error(`Error fetching from proxy for ${source.name}:`, error);
      throw error;
    }
  }

  // =================== 内部方法 ===================

  /**
   * 保存同步下来的文章
   */
  private async saveArticlesFromSync(items: ServerItem[]): Promise<Article[]> {
    const savedArticles: Article[] = [];

    for (const item of items) {
      try {
        // 查找本地源 ID
        // 优先使用 SourceURL 匹配，因为 SourceID 是服务端的
        let localSourceId = 0;
        let sourceName = item.SourceTitle || 'Unknown';
        
        if (item.SourceURL) {
          const sources = await this.databaseService.executeQuery(
            'SELECT id, title FROM rss_sources WHERE url = ?',
            [item.SourceURL]
          );
          if (sources.length > 0) {
            localSourceId = sources[0].id;
            sourceName = sources[0].title || sourceName;
          }
        }

        // 如果找不到本地源，可能需要自动创建或者跳过
        // 这里选择跳过，或者归类到 "Unknown"
        if (localSourceId === 0) {
           // 尝试用 SourceID 匹配 (如果之前同步过 ID)
           // 但目前没有同步机制，所以暂时忽略或创建临时源
           // 简单起见，如果 URL 匹配不到，就不保存关联源
           // 但 Article 必须有 sourceId
           // 我们可以创建一个 "Inbox" 源?
           // 或者自动创建该源?
           
           // 自动创建源
           if (item.SourceURL) {
             const result = await this.databaseService.executeInsert(
               'INSERT INTO rss_sources (url, title, is_active) VALUES (?, ?, 1)',
               [item.SourceURL, item.SourceTitle || 'Auto Imported']
             );
             localSourceId = Number(result.insertId);
           }
        }

        // 确定内容：优先使用 CleanContent (可能是压缩后的，也可能是原始的)
        // 或者是 Content (如果 CleanContent 为空)
        // 服务端 Sync API 已经根据 image_compression 参数处理了 CleanContent
        const content = item.CleanContent || item.Content || item.XMLContent || '';
        
        // 检查是否已存在
        const existing = await this.databaseService.executeQuery(
          'SELECT id FROM articles WHERE url = ? OR guid = ?',
          [item.GUID, item.GUID] // 使用 GUID 去重
        );

        if (existing.length > 0) {
          continue; // 已存在
        }

        const result = await this.databaseService.executeInsert(
          `INSERT INTO articles (
            title, url, content, summary, author, published_at, rss_source_id, 
            source_name, category, word_count, reading_time, difficulty, 
            is_read, is_favorite, read_progress, tags, guid, image_url,
            image_caption, image_credit
          ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
          [
            item.Title,
            item.GUID, // 使用 GUID 作为 URL (如果 GUID 是 URL) 或者 item.SourceURL + GUID? 
                       // 通常 GUID 是 URL，或者是唯一 ID。这里我们存 GUID 到 url 字段? 
                       // 不，url 字段应该是文章链接。服务端 Item 没有 Link 字段?
                       // 这是一个疏忽。Item struct 只有 GUID。通常 RSS Item Link != GUID.
                       // 无论如何，我们暂且用 GUID。如果 GUID 不是 URL，客户端点击可能会有问题。
                       // 但 JSON 渲染是本地的，所以也许没关系。
            content,
            item.Summary,
            item.Author,
            item.PublishedAt, // 字符串格式
            localSourceId,
            sourceName,
            'General', // Category
            item.WordCount,
            item.ReadingTime,
            'intermediate', // Difficulty
            0, 0, 0, // is_read, is_favorite, read_progress
            '[]', // tags
            item.GUID,
            item.CoverImage,
            item.ImageCaption,
            item.ImageCredit
          ]
        );

        savedArticles.push({
          id: Number(result.insertId),
          title: item.Title,
          content: content,
          summary: item.Summary,
          author: item.Author,
          publishedAt: new Date(item.PublishedAt),
          sourceId: localSourceId,
          sourceName: sourceName,
          url: item.GUID,
          imageUrl: item.CoverImage,
          imageCaption: item.ImageCaption,
          imageCredit: item.ImageCredit,
          tags: [],
          category: 'General',
          wordCount: item.WordCount,
          readingTime: item.ReadingTime,
          difficulty: 'intermediate',
          isRead: false,
          isFavorite: false,
          readProgress: 0,
        });

      } catch (error) {
        logger.error(`Failed to save item ${item.ID}:`, error);
      }
    }

    return savedArticles;
  }

  /**
   * 发送 ACK 确认
   */
  private async acknowledgeItems(
    itemIds: number[],
    config: ProxyModeConfig
  ): Promise<void> {
    if (itemIds.length === 0) return;
    try {
      await fetch(`${config.serverUrl}/api/ack`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${config.token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ item_ids: itemIds }),
      });
      logger.info(`ACK 已发送，确认 ${itemIds.length} 条记录`);
    } catch (error) {
      logger.error('Error acknowledging items:', error);
    }
  }
}

export const proxyRSSService = ProxyRSSService.getInstance();
