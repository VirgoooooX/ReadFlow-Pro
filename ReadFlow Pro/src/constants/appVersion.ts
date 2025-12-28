// 应用版本信息
// 此文件由构建脚本自动更新，请勿手动修改

export const APP_VERSION = {
  version: '0.0.1',
  buildNumber: 1,
  updateTime: '2025-12-27',
  changelog: [
    '重构RSS架构为服务端处理并添加图片压缩选项',
    '将RSS处理逻辑迁移到服务端，移除客户端LocalRSSService',
    '新增图片压缩功能，可在设置中配置',
    '更新服务端同步接口支持图片压缩参数',
    '重构ProxyRSSService以适配服务端JSON数据',
    '更新Dockerfile优化构建流程',
    '修改数据库结构支持原始内容和压缩内容',
    'initial gitignore',
  ],
};

export const APP_INFO = {
  name: 'ReadFlow Pro',
  description: '一款专注英语阅读学习的应用',
};
