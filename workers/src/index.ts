/**
 * Firefly Media Gateway - Cloudflare Workers 版本
 *
 * 功能：接收文件上传，转发到 Telegram，返回元数据
 * 元数据由业务方自行存储
 */

import { Router } from 'itty-router';

const router = Router();

// 环境变量声明（wrangler.toml 中配置）
interface Env {
  // 默认 bot（兼容单 bot 配置）
  TELEGRAM_BOT_TOKEN?: string;
  TELEGRAM_CHAT_ID?: string;
  // 多 bot 配置（JSON 格式，优先级高于默认）
  TELEGRAM_BOTS_CONFIG?: string;  // '{"bot1":{"token":"...","default_group":"group1"},"bot2":{...}}'
  AUTH_TOKEN?: string;             // 可选：Bearer token 鉴权
  MAX_UPLOAD_SIZE?: string;        // 可选：最大上传大小（字节）
  PRIVATE_RULES?: string;          // 可选：私有规则列表（英文逗号分隔，支持 "*"、"all" 或特定 project/usage）
}

// Bot 配置
interface BotConfig {
  token: string;
  default_group?: string;
}

// 上传响应
interface UploadResponse {
  success: boolean;
  provider: 'telegram';
  bot_name: string;
  group_id: string;
  file_id: string;
  file_unique_id?: string;
  file_url: string;
  mime_type: string;
  file_size: number;
  timestamp: string;
}

interface DeleteFileRequest {
  bot_name?: string;
  group_id?: string;
  message_id: number;  // Telegram 消息 ID，用于删除
}

interface GetFileResponse {
  success: boolean;
  file_id: string;
  bot_name: string;
  stream_url: string;
  tg_url?: string;  // debug 模式返回
  mime_type: string;
  file_size: number;
}

interface ErrorResponse {
  success: false;
  error: string;
  code: string;
}

// Delete 响应
interface DeleteFileResponse {
  success: boolean;
  bot_name: string;
  message_id: number;
  deleted: boolean;
}

// MIME 类型白名单
const ALLOWED_MIME_TYPES = [
  'image/jpeg',
  'image/png',
  'image/webp',
  'video/mp4',
  'video/webm',
  'video/quicktime',
];

// 最大文件大小（Telegram 限制 50MB）
const DEFAULT_MAX_SIZE = 50 * 1024 * 1024;

/**
 * 鉴权中间件
 */
function auth(env: Env): boolean {
  // 未配置 AUTH_TOKEN 则跳过鉴权
  if (!env.AUTH_TOKEN) return true;
  // 在 handleRequest 中检查 Authorization header
  return true; // 实际检查在 request handler 中
}

/**
 * 验证 MIME 类型
 */
function isValidMimeType(mimeType: string): boolean {
  return ALLOWED_MIME_TYPES.includes(mimeType.toLowerCase());
}

/**
 * 验证文件大小
 */
function isValidFileSize(size: number, maxSize: number): boolean {
  return size > 0 && size <= maxSize;
}

/**
 * 解析 Bot 配置
 */
function parseBotConfig(env: Env): Map<string, BotConfig> {
  const configs = new Map<string, BotConfig>();

  // 解析多 bot 配置
  if (env.TELEGRAM_BOTS_CONFIG) {
    try {
      const parsed = JSON.parse(env.TELEGRAM_BOTS_CONFIG) as Record<string, { token?: string; default_group?: string }>;
      for (const [name, cfg] of Object.entries(parsed)) {
        if (cfg.token) {
          configs.set(name, {
            token: cfg.token,
            default_group: cfg.default_group,
          });
        }
      }
    } catch (e) {
      console.error('Failed to parse TELEGRAM_BOTS_CONFIG:', e);
    }
  }

  // 兼容单 bot 配置（作为 "default" bot）
  if (env.TELEGRAM_BOT_TOKEN && configs.size === 0) {
    configs.set('default', {
      token: env.TELEGRAM_BOT_TOKEN,
      default_group: env.TELEGRAM_CHAT_ID,
    });
  }

  return configs;
}

/**
 * 获取 Bot 配置
 */
function getBotConfig(botConfigs: Map<string, BotConfig>, botName: string): BotConfig | null {
  // 支持不指定 bot 名称时使用第一个可用的
  if (!botName && botConfigs.size > 0) {
    const first = botConfigs.keys().next().value;
    return first ? (botConfigs.get(first) || null) : null;
  }
  return botConfigs.get(botName) || null;
}

/**
 * 主上传处理
 */
async function handleUpload(request: Request, env: Env): Promise<Response> {
  // 鉴权检查
  if (env.AUTH_TOKEN) {
    const authHeader = request.headers.get('Authorization');
    if (!authHeader || !authHeader.startsWith('Bearer ')) {
      return error('Unauthorized', 'UNAUTHORIZED', 401);
    }
    const token = authHeader.slice(7);
    if (token !== env.AUTH_TOKEN) {
      return error('Invalid auth token', 'FORBIDDEN', 403);
    }
  }

  // 解析 bot 配置
  const botConfigs = parseBotConfig(env);
  if (botConfigs.size === 0) {
    return error('No bot configured', 'NO_BOT_CONFIG', 500);
  }

  // 解析表单数据
  const formData = await request.formData();
  const file = formData.get('file') as File | null;

  // 获取 bot 名称（优先 header，其次 form）
  let botName = request.headers.get('X-Bot-Name') || formData.get('bot') as string || '';
  if (!botName && botConfigs.size === 1) {
    botName = botConfigs.keys().next().value || '';
  }

  // 获取 group ID（优先 form，其次 bot 配置的默认值）
  const groupId = (formData.get('group') as string) || undefined;

  if (!file) {
    return error('No file provided', 'NO_FILE', 400);
  }

  const maxSize = env.MAX_UPLOAD_SIZE
    ? parseInt(env.MAX_UPLOAD_SIZE, 10)
    : DEFAULT_MAX_SIZE;

  // 验证文件
  if (!isValidMimeType(file.type)) {
    return error(`Invalid MIME type: ${file.type}`, 'INVALID_MIME', 400);
  }

  if (!isValidFileSize(file.size, maxSize)) {
    return error(`File size exceeds ${maxSize} bytes`, 'FILE_TOO_LARGE', 413);
  }

  // 获取 bot 配置
  const botConfig = getBotConfig(botConfigs, botName);
  if (!botConfig) {
    return error(`Bot not found: ${botName}`, 'BOT_NOT_FOUND', 404);
  }

  // 确定目标 group
  const targetGroup = groupId || botConfig.default_group;
  if (!targetGroup) {
    return error('Group ID not specified and no default configured', 'NO_GROUP', 400);
  }

  // 转发到 Telegram
  try {
    const tgFormData = new FormData();
    tgFormData.append('chat_id', targetGroup);
    tgFormData.append('document', file);

    const tgResponse = await fetch(
      `https://api.telegram.org/bot${botConfig.token}/sendDocument`,
      {
        method: 'POST',
        body: tgFormData,
      }
    );

    const tgData = await tgResponse.json() as any;

    if (!tgData.ok) {
      return error(
        `Telegram API error: ${tgData.description || 'Unknown'}`,
        'TG_API_ERROR',
        502
      );
    }

    const doc = tgData.result.document;

    // 构造返回
    const response: UploadResponse = {
      success: true,
      provider: 'telegram',
      bot_name: botName,
      group_id: targetGroup,
      file_id: doc.file_id,
      file_unique_id: doc.file_unique_id,
      file_url: '', // 稍后填充
      mime_type: doc.mime_type,
      file_size: doc.file_size,
      timestamp: new Date().toISOString(),
    };

    // 获取 file_url（需要额外调用 getFile）
    try {
      const fileResponse = await fetch(
        `https://api.telegram.org/bot${botConfig.token}/getFile?file_id=${doc.file_id}`
      );
      const fileData = await fileResponse.json() as any;
      if (fileData.ok && fileData.result.file_path) {
        response.file_url = `https://api.telegram.org/file/bot${botConfig.token}/${fileData.result.file_path}`;
      }
    } catch {
      // getFile 失败不影响上传成功
    }

    return jsonResponse(response, 201);

  } catch (e) {
    return error(
      `Upload failed: ${e instanceof Error ? e.message : 'Unknown error'}`,
      'UPLOAD_FAILED',
      500
    );
  }
}

/**
 * 删除文件（通过删除 Telegram 消息）
 */
async function handleDeleteFile(request: Request, env: Env): Promise<Response> {
  // 鉴权检查
  if (env.AUTH_TOKEN) {
    const authHeader = request.headers.get('Authorization');
    if (!authHeader || !authHeader.startsWith('Bearer ')) {
      return error('Unauthorized', 'UNAUTHORIZED', 401);
    }
    const token = authHeader.slice(7);
    if (token !== env.AUTH_TOKEN) {
      return error('Invalid auth token', 'FORBIDDEN', 403);
    }
  }

  // 解析 bot 配置
  const botConfigs = parseBotConfig(env);
  if (botConfigs.size === 0) {
    return error('No bot configured', 'NO_BOT_CONFIG', 500);
  }

  // 解析参数（支持 query 或 body）
  let params: URLSearchParams;
  const contentType = request.headers.get('Content-Type');
  if (contentType && contentType.includes('application/json')) {
    const body = await request.json() as Record<string, string | number>;
    params = new URLSearchParams(Object.entries(body).flatMap(([k, v]) => v ? [[k, String(v)]] : []));
  } else {
    params = new URL(request.url).searchParams;
  }

  const messageIdStr = params.get('message_id');
  if (!messageIdStr) {
    return error('message_id is required', 'MISSING_MESSAGE_ID', 400);
  }
  const messageId = parseInt(messageIdStr, 10);
  if (isNaN(messageId)) {
    return error('Invalid message_id', 'INVALID_MESSAGE_ID', 400);
  }

  const groupId = params.get('group_id');
  if (!groupId) {
    return error('group_id is required', 'MISSING_GROUP_ID', 400);
  }

  const botName = params.get('bot_name') || '';
  const botConfig = getBotConfig(botConfigs, botName);
  if (!botConfig) {
    return error(`Bot not found: ${botName}`, 'BOT_NOT_FOUND', 404);
  }

  try {
    // 调用 Telegram deleteMessage API
    const response = await fetch(
      `https://api.telegram.org/bot${botConfig.token}/deleteMessage?chat_id=${groupId}&message_id=${messageId}`
    );
    const data = await response.json() as any;

    const resp: DeleteFileResponse = {
      success: true,
      bot_name: botName,
      message_id: messageId,
      deleted: data.ok,
    };

    return jsonResponse(data.ok ? resp : error(data.description || 'Delete failed', 'DELETE_FAILED', 502));
  } catch (e) {
    return error(
      `Delete file failed: ${e instanceof Error ? e.message : 'Unknown error'}`,
      'DELETE_FAILED',
      500
    );
  }
}

/**
 * 查找能访问指定文件的 bot（自动匹配）
 */
async function findBotForFile(
  fileId: string,
  botConfigs: Map<string, BotConfig>
): Promise<{ botName: string; botConfig: BotConfig } | null> {
  for (const [botName, cfg] of botConfigs) {
    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 5000); // 5秒超时

      const resp = await fetch(
        `https://api.telegram.org/bot${cfg.token}/getFile?file_id=${fileId}`,
        { signal: controller.signal }
      );
      clearTimeout(timeoutId);

      const data = await resp.json() as any;
      if (data.ok) {
        return { botName, botConfig: cfg };
      }
    } catch {
      // 继续尝试下一个 bot
      continue;
    }
  }
  return null;
}

/**
 * 获取文件访问 URL
 */
async function handleGetFile(request: Request, env: Env): Promise<Response> {
  const url = new URL(request.url);
  const fileId = url.searchParams.get('file_id');
  const debugMode = url.searchParams.get('debug') === 'true';

  if (!fileId) {
    return error('file_id is required', 'MISSING_FILE_ID', 400);
  }

  // 鉴权检查（通过 PRIVATE_RULES 统一判定）
  let isPrivate = false;
  if (env.AUTH_TOKEN) {
    const rulesStr = (env.PRIVATE_RULES || '').trim();
    if (rulesStr) {
      const rules = rulesStr.split(',').map(s => s.trim()).filter(Boolean);
      if (rules.includes('*') || rules.includes('all')) {
        isPrivate = true;
      } else {
        const project = url.searchParams.get('project') || request.headers.get('X-Project') || '';
        const usage = url.searchParams.get('usage') || request.headers.get('X-Usage') || '';
        if (rules.includes(project) || rules.includes(usage)) {
          isPrivate = true;
        }
      }
    }
  }

  if (isPrivate && env.AUTH_TOKEN) {
    let authorized = false;
    const authHeader = request.headers.get('Authorization');
    if (authHeader && authHeader.startsWith('Bearer ')) {
      const token = authHeader.slice(7);
      if (token === env.AUTH_TOKEN) {
        authorized = true;
      }
    }

    if (!authorized) {
      const sig = url.searchParams.get('token_sig');
      const expiresStr = url.searchParams.get('expires');
      if (sig && expiresStr) {
        const expires = parseInt(expiresStr, 10);
        if (!isNaN(expires) && Date.now() / 1000 <= expires) {
          authorized = await verifySignature(fileId, expires, sig, env.AUTH_TOKEN);
        }
      }
    }

    if (!authorized) {
      return error('Unauthorized', 'UNAUTHORIZED', 401);
    }
  }

  if (!fileId) {
    return error('file_id is required', 'MISSING_FILE_ID', 400);
  }

  const botConfigs = parseBotConfig(env);
  if (botConfigs.size === 0) {
    return error('No bot configured', 'NO_BOT_CONFIG', 500);
  }

  let botName: string | null = null;
  let botConfig: BotConfig | null = null;

  // 1. 优先从 Header 获取 bot_name
  const headerBotName = request.headers.get('X-Bot-Name');
  if (headerBotName) {
    botName = headerBotName;
    botConfig = getBotConfig(botConfigs, botName);
    if (!botConfig) {
      return error(`Bot not found: ${botName}`, 'BOT_NOT_FOUND', 404);
    }
  }

  // 2. 如果 Header 没指定，自动匹配
  if (!botConfig) {
    const match = await findBotForFile(fileId, botConfigs);
    if (!match) {
      return error('No bot can access this file', 'FILE_NOT_ACCESSIBLE', 404);
    }
    botName = match.botName;
    botConfig = match.botConfig;
  }

  try {
    const getFileResp = await fetch(
      `https://api.telegram.org/bot${botConfig.token}/getFile?file_id=${fileId}`
    );
    const getFileData = await getFileResp.json() as any;

    if (!getFileData.ok) {
      return error(
        `Telegram API error: ${getFileData.description || 'Unknown'}`,
        'TG_API_ERROR',
        502
      );
    }

    const filePath = getFileData.result.file_path;
    const mimeType = getFileData.result.mime_type || 'application/octet-stream';
    const fileSize = getFileData.result.file_size || 0;

    const resp: GetFileResponse = {
      success: true,
      file_id: fileId,
      bot_name: botName || '',
      stream_url: `${url.protocol}//${url.host}/stream?file_id=${fileId}`,
      mime_type: mimeType,
      file_size: fileSize,
    };

    // Debug 模式：额外返回 TG URL（包含 token）
    if (debugMode) {
      resp.tg_url = `https://api.telegram.org/file/bot${botConfig.token}/${filePath}`;
    }

    return jsonResponse(resp);
  } catch (e) {
    return error(
      `Get file failed: ${e instanceof Error ? e.message : 'Unknown error'}`,
      'GET_FILE_FAILED',
      500
    );
  }
}

/**
 * 流式文件下载（代理 Telegram 文件，支持 Range 请求）
 */
async function handleStreamFile(request: Request, env: Env): Promise<Response> {
  const url = new URL(request.url);
  const fileId = url.searchParams.get('file_id');

  if (!fileId) {
    return error('file_id is required', 'MISSING_FILE_ID', 400);
  }

  // 鉴权检查（通过 PRIVATE_RULES 统一判定）
  let isPrivate = false;
  if (env.AUTH_TOKEN) {
    const rulesStr = (env.PRIVATE_RULES || '').trim();
    if (rulesStr) {
      const rules = rulesStr.split(',').map(s => s.trim()).filter(Boolean);
      if (rules.includes('*') || rules.includes('all')) {
        isPrivate = true;
      } else {
        const project = url.searchParams.get('project') || request.headers.get('X-Project') || '';
        const usage = url.searchParams.get('usage') || request.headers.get('X-Usage') || '';
        if (rules.includes(project) || rules.includes(usage)) {
          isPrivate = true;
        }
      }
    }
  }

  if (isPrivate && env.AUTH_TOKEN) {
    let authorized = false;
    const authHeader = request.headers.get('Authorization');
    if (authHeader && authHeader.startsWith('Bearer ')) {
      const token = authHeader.slice(7);
      if (token === env.AUTH_TOKEN) {
        authorized = true;
      }
    }

    if (!authorized) {
      const sig = url.searchParams.get('token_sig');
      const expiresStr = url.searchParams.get('expires');
      if (sig && expiresStr) {
        const expires = parseInt(expiresStr, 10);
        if (!isNaN(expires) && Date.now() / 1000 <= expires) {
          authorized = await verifySignature(fileId, expires, sig, env.AUTH_TOKEN);
        }
      }
    }

    if (!authorized) {
      return error('Unauthorized', 'UNAUTHORIZED', 401);
    }
  }

  if (!fileId) {
    return error('file_id is required', 'MISSING_FILE_ID', 400);
  }

  const botConfigs = parseBotConfig(env);
  if (botConfigs.size === 0) {
    return error('No bot configured', 'NO_BOT_CONFIG', 500);
  }

  let botName: string | null = null;
  let botConfig: BotConfig | null = null;

  // 1. 优先从 Header 获取 bot_name
  const headerBotName = request.headers.get('X-Bot-Name');
  if (headerBotName) {
    botName = headerBotName;
    botConfig = getBotConfig(botConfigs, botName);
    if (!botConfig) {
      return error(`Bot not found: ${botName}`, 'BOT_NOT_FOUND', 404);
    }
  }

  // 2. 如果 Header 没指定，自动匹配
  if (!botConfig) {
    const match = await findBotForFile(fileId, botConfigs);
    if (!match) {
      return error('No bot can access this file', 'FILE_NOT_ACCESSIBLE', 404);
    }
    botName = match.botName;
    botConfig = match.botConfig;
  }

  try {
    // 获取 Telegram 文件信息
    const getFileResp = await fetch(
      `https://api.telegram.org/bot${botConfig.token}/getFile?file_id=${fileId}`
    );
    const getFileData = await getFileResp.json() as any;

    if (!getFileData.ok) {
      return error(
        `Telegram API error: ${getFileData.description || 'Unknown'}`,
        'TG_API_ERROR',
        502
      );
    }

    const filePath = getFileData.result.file_path;
    const tgFileUrl = `https://api.telegram.org/file/bot${botConfig.token}/${filePath}`;
    const mimeType = getFileData.result.mime_type || 'application/octet-stream';
    const fileSize = getFileData.result.file_size || 0;

    // 流式返回文件
    const headers = new Headers();

    // 转发 Range 头（支持视频进度条）
    const rangeHeader = request.headers.get('Range');
    if (rangeHeader) {
      headers.set('Range', rangeHeader);
    }

    // 获取文件
    const fileResp = await fetch(tgFileUrl, {
      headers: headers,
    });

    // 构建响应
    const responseHeaders = new Headers();
    responseHeaders.set('Content-Type', mimeType);
    responseHeaders.set('Content-Length', String(fileSize));
    responseHeaders.set('Accept-Ranges', 'bytes');
    responseHeaders.set('Cache-Control', 'public, max-age=86400');
    responseHeaders.set('Access-Control-Allow-Origin', '*');
    responseHeaders.set('X-Served-By-Bot', botName || '');

    const ext = extByMIME(mimeType);
    const filename = `${fileId}${ext}`;
    responseHeaders.set('Content-Disposition', `inline; filename="${filename}"`);

    // 返回状态码（Range 请求返回 206）
    const status = rangeHeader && fileResp.status === 206 ? 206 : fileResp.status;

    return new Response(fileResp.body, {
      status,
      headers: responseHeaders,
    });

  } catch (e) {
    return error(
      `Stream file failed: ${e instanceof Error ? e.message : 'Unknown error'}`,
      'STREAM_FAILED',
      500
    );
  }
}

/**
 * 健康检查
 */
function handleHealthCheck(): Response {
  return jsonResponse({
    success: true,
    service: 'firefly-media-gateway',
    version: '1.0.0',
    timestamp: new Date().toISOString(),
  });
}

/**
 * 返回 JSON 响应
 */
function jsonResponse(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data, null, 2), {
    status,
    headers: {
      'Content-Type': 'application/json',
      'Access-Control-Allow-Origin': '*',
    },
  });
}

/**
 * 返回错误响应
 */
function error(message: string, code: string, status = 400): Response {
  const err: ErrorResponse = {
    success: false,
    error: message,
    code,
  };
  return jsonResponse(err, status);
}

/**
 * 校验签名
 */
async function verifySignature(fileId: string, expires: number, sig: string, authToken: string): Promise<boolean> {
  try {
    const encoder = new TextEncoder();
    const keyData = encoder.encode(authToken);
    const data = encoder.encode(`${fileId}:${expires}`);

    const key = await crypto.subtle.importKey(
      'raw',
      keyData,
      { name: 'HMAC', hash: 'SHA-256' },
      false,
      ['verify']
    );

    // Convert hex sig to Uint8Array
    const sigBytes = new Uint8Array(sig.match(/.{1,2}/g)!.map(byte => parseInt(byte, 16)));

    return await crypto.subtle.verify(
      'HMAC',
      key,
      sigBytes,
      data
    );
  } catch (e) {
    return false;
  }
}

/**
 * OPTIONS 预检请求
 */
function handleOptions(): Response {
  return new Response(null, {
    status: 204,
    headers: {
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Methods': 'POST, GET, OPTIONS',
      'Access-Control-Allow-Headers': 'Content-Type, Authorization, X-Bot-Name',
      'Access-Control-Max-Age': '86400',
    },
  });
}

// 路由定义
router
  .options('*', () => handleOptions())
  .get('/', () => handleHealthCheck())
  .get('/health', () => handleHealthCheck())
  .post('/upload', (request, env) => handleUpload(request, env))
  .get('/get', (request, env) => handleGetFile(request, env))
  .get('/stream', (request, env) => handleStreamFile(request, env))
  .delete('/delete', (request, env) => handleDeleteFile(request, env))
  .post('/delete', (request, env) => handleDeleteFile(request, env));

// 导出 fetch handler
export default {
  fetch: (request: Request, env: Env, ctx: ExecutionContext) =>
    router.handle(request, env, ctx).catch((e) =>
      error(
        `Internal error: ${e instanceof Error ? e.message : 'Unknown'}`,
        'INTERNAL_ERROR',
        500
      )
    ),
};

function extByMIME(mimeType: string): string {
  switch (mimeType.toLowerCase()) {
    case 'image/jpeg': return '.jpg';
    case 'image/png': return '.png';
    case 'image/webp': return '.webp';
    case 'video/mp4': return '.mp4';
    case 'video/webm': return '.webm';
    case 'video/quicktime': return '.mov';
    default: return '';
  }
}
