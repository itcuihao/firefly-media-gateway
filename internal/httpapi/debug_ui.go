package httpapi

import (
	"fmt"
	"net/http"
)

func (s *Server) handleDebugUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, debugHTML)
}

const debugHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Firefly 控制中心</title>

  
  <style>
    :root {
      --bg: #030712;
      --card-bg: rgba(17, 24, 39, 0.7);
      --card-border: rgba(255, 255, 255, 0.08);
      --text-main: #f3f4f6;
      --text-muted: #9ca3af;
      --accent-primary: #06b6d4;
      --accent-secondary: #0d9488;
      --accent-glow: rgba(6, 182, 212, 0.15);
      --danger: #ef4444;
      --success: #10b981;
    }

    * {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
    }

    body {
      background: radial-gradient(circle at 50% 0%, #083344, var(--bg) 70%);
      color: var(--text-main);
      font-family: ui-sans-serif, system-ui, -apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif;
      min-height: 100vh;
      padding: 32px 16px;
      line-height: 1.5;
    }

    .container {
      max-width: 1200px;
      margin: 0 auto;
    }

    /* Header styling */
    header {
      text-align: center;
      margin-bottom: 40px;
      display: flex;
      flex-direction: column;
      align-items: center;
    }

    .brand-container {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 16px;
      margin-bottom: 12px;
    }

    .logo-wrapper {
      position: relative;
      display: inline-block;
      width: 56px;
      height: 56px;
    }

    .logo-img {
      width: 100%;
      height: 100%;
      object-fit: contain;
      filter: drop-shadow(0 0 12px rgba(6, 182, 212, 0.6));
      transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    }

    .logo-wrapper:hover .logo-img {
      transform: scale(1.1) rotate(6deg);
    }

    .logo-badge {
      position: absolute;
      top: -6px;
      right: -12px;
      background: linear-gradient(135deg, #fbbf24, #f59e0b);
      color: #030712;
      font-size: 9px;
      font-weight: 800;
      padding: 1px 5px;
      border-radius: 9999px;
      box-shadow: 0 0 8px rgba(245, 158, 11, 0.6);
      white-space: nowrap;
      pointer-events: none;
      animation: pulse 2.5s infinite;
      border: 1px solid rgba(3, 7, 18, 0.8);
      letter-spacing: 0.05em;
    }

    @keyframes pulse {
      0%, 100% {
        transform: scale(1);
        box-shadow: 0 0 8px rgba(245, 158, 11, 0.6);
      }
      50% {
        transform: scale(1.08);
        box-shadow: 0 0 14px rgba(245, 158, 11, 0.9);
      }
    }

    header h1 {
      font-size: 36px;
      font-weight: 800;
      letter-spacing: -0.025em;
      background: linear-gradient(135deg, #22d3ee, #0d9488);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }

    header p {
      color: var(--text-muted);
      font-size: 16px;
    }

    /* Grid Layout */
    .grid {
      display: grid;
      grid-template-columns: 1fr;
      gap: 24px;
    }

    @media (min-width: 900px) {
      .grid {
        grid-template-columns: 350px 1fr;
      }
    }

    /* Glass Card Style */
    .card {
      background: var(--card-bg);
      backdrop-filter: blur(16px);
      -webkit-backdrop-filter: blur(16px);
      border: 1px solid var(--card-border);
      border-radius: 16px;
      padding: 24px;
      box-shadow: 0 10px 30px -10px rgba(0, 0, 0, 0.7);
      transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    }

    .card:hover {
      border-color: rgba(6, 182, 212, 0.25);
      box-shadow: 0 10px 30px -10px rgba(0, 0, 0, 0.7), 0 0 20px var(--accent-glow);
    }

    .card h2 {
      font-size: 18px;
      font-weight: 600;
      margin-bottom: 16px;
      display: flex;
      align-items: center;
      gap: 8px;
      color: #fff;
      border-bottom: 1px solid rgba(255, 255, 255, 0.05);
      padding-bottom: 10px;
    }

    /* Forms */
    .form-group {
      margin-bottom: 16px;
      display: flex;
      flex-direction: column;
      gap: 6px;
    }

    label {
      font-size: 13px;
      color: var(--text-muted);
      font-weight: 500;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }

    input, select, textarea {
      background: rgba(15, 23, 42, 0.6);
      border: 1px solid var(--card-border);
      border-radius: 8px;
      color: #fff;
      padding: 12px 14px;
      font-size: 14px;
      outline: none;
      transition: all 0.2s ease;
      font-family: inherit;
    }

    input:focus, select:focus, textarea:focus {
      border-color: var(--accent-primary);
      box-shadow: 0 0 0 3px rgba(6, 182, 212, 0.15);
      background: rgba(15, 23, 42, 0.8);
    }

    /* Buttons */
    .btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
      color: #fff;
      border: none;
      border-radius: 8px;
      padding: 12px 20px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      transition: all 0.2s ease;
      text-decoration: none;
      width: 100%;
    }

    .btn:hover {
      transform: translateY(-1px);
      filter: brightness(1.1);
      box-shadow: 0 4px 12px rgba(6, 182, 212, 0.3);
    }

    .btn:active {
      transform: translateY(1px);
    }

    .btn-secondary {
      background: rgba(71, 85, 105, 0.3);
      border: 1px solid rgba(255, 255, 255, 0.1);
    }

    .btn-secondary:hover {
      background: rgba(71, 85, 105, 0.5);
      box-shadow: none;
    }

    .btn-danger {
      background: var(--danger);
    }

    .btn-danger:hover {
      box-shadow: 0 4px 12px rgba(239, 68, 68, 0.3);
    }

    .btn-sm {
      padding: 6px 12px;
      font-size: 12px;
      border-radius: 6px;
      width: auto;
    }

    /* Table/Media Grid */
    .media-table-wrapper {
      overflow-x: auto;
      margin-top: 12px;
    }

    .media-table {
      width: 100%;
      border-collapse: collapse;
      text-align: left;
      font-size: 14px;
    }

    .media-table th, .media-table td {
      padding: 14px;
      border-bottom: 1px solid rgba(255, 255, 255, 0.05);
    }

    .media-table th {
      color: var(--text-muted);
      font-weight: 600;
      text-transform: uppercase;
      font-size: 11px;
      letter-spacing: 0.05em;
    }

    .media-table tr:hover td {
      background: rgba(255, 255, 255, 0.02);
    }

    /* Thumbnail Previews */
    .media-preview {
      width: 48px;
      height: 48px;
      border-radius: 6px;
      object-fit: cover;
      background: #0f172a;
      border: 1px solid rgba(255, 255, 255, 0.1);
      display: flex;
      align-items: center;
      justify-content: center;
      color: var(--accent-primary);
      font-size: 12px;
    }

    .media-preview img, .media-preview video {
      width: 100%;
      height: 100%;
      object-fit: cover;
      border-radius: inherit;
    }

    /* Chat ID Retriever */
    .chat-id-list {
      display: flex;
      flex-direction: column;
      gap: 10px;
      margin-top: 14px;
    }

    .chat-item {
      background: rgba(15, 23, 42, 0.5);
      border: 1px solid rgba(255, 255, 255, 0.05);
      border-radius: 8px;
      padding: 10px 14px;
      display: flex;
      justify-content: space-between;
      align-items: center;
    }

    .chat-info {
      display: flex;
      flex-direction: column;
    }

    .chat-title {
      font-weight: 600;
      font-size: 14px;
    }

    .chat-meta {
      font-size: 12px;
      color: var(--text-muted);
      display: flex;
      gap: 8px;
    }

    .chat-type {
      text-transform: capitalize;
      color: var(--accent-primary);
    }

    /* Console logs response output */
    textarea.console {
      width: 100%;
      height: 150px;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-size: 13px;
      background: #020617;
      border: 1px solid rgba(255, 255, 255, 0.05);
      color: #38bdf8;
      border-radius: 8px;
      padding: 14px;
      resize: vertical;
    }

    /* Alerts */
    .alert {
      background: rgba(6, 182, 212, 0.05);
      border: 1px dashed rgba(6, 182, 212, 0.2);
      border-radius: 8px;
      padding: 12px;
      font-size: 13px;
      color: #22d3ee;
      margin-bottom: 16px;
    }

    /* Toast Notification */
    #toast {
      position: fixed;
      bottom: 24px;
      right: 24px;
      background: #10b981;
      color: white;
      padding: 12px 24px;
      border-radius: 8px;
      font-weight: 500;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
      opacity: 0;
      transform: translateY(20px);
      transition: all 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
      z-index: 1000;
    }

    #toast.show {
      opacity: 1;
      transform: translateY(0);
    }

    .action-group {
      display: flex;
      gap: 6px;
    }
  </style>
</head>
<body>
  <div id="toast">Toast Message</div>

  <div class="container">
    <header>
      <div class="brand-container">
        <div class="logo-wrapper">
          <img src="data:image/png;base64,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" alt="Firefly Logo" class="logo-img" />
          <span class="logo-badge">嘿嘿</span>
        </div>
        <h1>Firefly 控制中心</h1>
      </div>
      <p>统一媒体服务管理与联调控制台 (SQLite / PostgreSQL + Telegram / Discord 等)</p>
    </header>

    <div class="grid">
      <!-- Left sidebar - configuration & tools -->
      <div style="display: flex; flex-direction: column; gap: 24px;">
        <!-- Config Card -->
        <div class="card">
          <h2>🔧 全局配置</h2>
          <div class="form-group">
            <label>API 基础地址</label>
            <input id="baseUrl" value="" placeholder="http://localhost:8080" />
          </div>
          <div class="form-group">
            <label>Bearer Token</label>
            <input id="token" type="password" placeholder="MEDIA_GATEWAY_TOKEN" />
          </div>
        </div>

        <!-- Chat ID Retriever -->
        <div class="card">
          <h2>💬 Telegram Chat ID 获取</h2>
          <div class="alert">
            <strong>使用说明：</strong><br>
            1. 请先在配置文件或 <code>.env</code> 中填写并启用 <code>TELEGRAM_BOT_TOKEN</code>。<br>
            2. 把你的 Bot 添加到群组或频道中，并<strong>发送一条测试消息</strong>。<br>
            3. 点击下方按钮获取 Bot 最近收到的 Chat ID 信息。
          </div>
          <button class="btn btn-secondary" onclick="fetchTelegramChatIDs()">获取最近聊天 ID</button>
          
          <div class="chat-id-list" id="chatIdListContainer">
            <!-- Dynamic chats here -->
            <p style="text-align: center; color: var(--text-muted); font-size: 13px; margin-top: 10px;">暂无数据</p>
          </div>
        </div>

        <!-- Upload Form -->
        <div class="card">
          <h2>📤 媒体文件上传</h2>
          <div class="form-group">
            <label>项目名称 (Project)</label>
            <input id="project" value="interactive-video" placeholder="e.g. project-1" />
          </div>
          <div class="form-group">
            <label>使用场景 (Usage)</label>
            <select id="usage">
              <option value="cover">cover (封面)</option>
              <option value="scene">scene (场景/正片)</option>
            </select>
          </div>
          <div class="form-group">
            <label>选择文件</label>
            <input id="file" type="file" accept="image/*,video/*" />
          </div>
          <div class="form-group" style="flex-direction: row; align-items: center; gap: 8px; margin-bottom: 16px;">
            <input id="isMember" type="checkbox" style="width: auto; margin: 0; cursor: pointer;" />
            <label for="isMember" style="text-transform: none; cursor: pointer; user-select: none;">启用大文件分片上传 (需要会员身份)</label>
          </div>
          <button class="btn" onclick="uploadFile()">开始上传</button>
        </div>
      </div>

      <!-- Right main area - file list & console -->
      <div style="display: flex; flex-direction: column; gap: 24px;">
        <!-- File Explorer -->
        <div class="card" style="flex: 1;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px;">
            <h2>📂 媒体库管理器</h2>
            <button class="btn btn-secondary btn-sm" onclick="loadMediaAssets()">刷新列表</button>
          </div>

          <div class="media-table-wrapper">
            <table class="media-table">
              <thead>
                <tr>
                  <th style="width: 70px;">预览</th>
                  <th>ID</th>
                  <th>类型 (MIME)</th>
                  <th>大小</th>
                  <th>项目/用途</th>
                  <th>状态</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody id="mediaAssetsList">
                <tr>
                  <td colspan="7" style="text-align: center; color: var(--text-muted); padding: 40px 0;">
                    加载中或库中暂无媒体资源...
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Console Response -->
        <div class="card">
          <h2>💻 调试控制台响应</h2>
          <textarea id="output" class="console" readonly placeholder="等待 API 调试输出..."></textarea>
        </div>
      </div>
    </div>
  </div>

  <script>
    // Initialize Local Base URL on Load
    (function initBaseUrl() {
      var local = window.location.origin;
      document.getElementById('baseUrl').value = local;
      
      // Try to load token from localStorage if exists
      var savedToken = localStorage.getItem('media_gateway_token');
      if (savedToken) {
        document.getElementById('token').value = savedToken;
      }
      
      // Load initial assets
      loadMediaAssets();
    })();

    // Save token to local storage on change
    document.getElementById('token').addEventListener('input', function(e) {
      localStorage.setItem('media_gateway_token', e.target.value.trim());
    });

    // Helper: Build headers
    function getAuthHeaders() {
      var token = document.getElementById('token').value.trim();
      return token ? { 'Authorization': 'Bearer ' + token } : {};
    }

    // Helper: Get Base URL
    function getBaseUrl() {
      return document.getElementById('baseUrl').value.trim().replace(/\/$/, '');
    }

    // Helper: Toast Notifications
    function showToast(message, type) {
      var tType = type || 'success';
      var toast = document.getElementById('toast');
      toast.innerText = message;
      toast.style.background = tType === 'success' ? '#10b981' : '#ef4444';
      toast.classList.add('show');
      setTimeout(function() {
        toast.classList.remove('show');
      }, 3000);
    }

    // Helper: Set Console Output
    function setConsoleOutput(obj) {
      var el = document.getElementById('output');
      el.value = typeof obj === 'string' ? obj : JSON.stringify(obj, null, 2);
    }

    // Helper: Format Bytes to human readable
    function formatBytes(bytes) {
      if (bytes === 0) return '0 B';
      var k = 1024;
      var sizes = ['B', 'KB', 'MB', 'GB'];
      var i = Math.floor(Math.log(bytes) / Math.log(k));
      return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    // 1. Fetch Telegram Chat IDs
    async function fetchTelegramChatIDs() {
      var container = document.getElementById('chatIdListContainer');
      container.innerHTML = '<p style="text-align: center; color: var(--text-muted); font-size: 13px; padding: 10px 0;">加载中...</p>';
      
      try {
        var res = await fetch(getBaseUrl() + '/api/v1/telegram/chat-ids', {
          headers: getAuthHeaders()
        });
        
        var data = await res.json();
        setConsoleOutput({ status: res.status, body: data });

        if (res.status !== 200) {
          container.innerHTML = '<p style="text-align: center; color: var(--danger); font-size: 13px; padding: 10px 0;">' + (data.error || '获取失败') + '</p>';
          showToast(data.error || '获取失败', 'error');
          return;
        }

        if (!data || data.length === 0) {
          container.innerHTML = '<p style="text-align: center; color: var(--text-muted); font-size: 13px; padding: 10px 0;">未检测到最新消息，请先向 Bot 发送消息后重试。</p>';
          return;
        }

        container.innerHTML = '';
        data.forEach(function(chat) {
          var div = document.createElement('div');
          div.className = 'chat-item';
          div.innerHTML = 
            '<div class="chat-info">' +
              '<span class="chat-title">' + chat.title + '</span>' +
              '<div class="chat-meta">' +
                '<span>ID: <code style="color: #fff; font-family: monospace;">' + chat.id + '</code></span>' +
                '<span class="chat-type">' + chat.type + '</span>' +
              '</div>' +
            '</div>' +
            '<button class="btn btn-secondary btn-sm" onclick="applyChatID(\'' + chat.id + '\')">复制</button>';
          container.appendChild(div);
        });
        showToast('获取成功！');
      } catch (e) {
        setConsoleOutput(String(e));
        container.innerHTML = '<p style="text-align: center; color: var(--danger); font-size: 13px; padding: 10px 0;">请求异常: ' + e.message + '</p>';
        showToast('请求异常', 'error');
      }
    }

    function applyChatID(id) {
      navigator.clipboard.writeText(id).then(function() {
        showToast('已复制 ID: ' + id);
      });
    }

    // 2. Upload file
    async function uploadFile() {
      var fileInput = document.getElementById('file');
      if (!fileInput.files || !fileInput.files[0]) {
        showToast('请先选择文件', 'error');
        return;
      }
      
      var form = new FormData();
      form.append('file', fileInput.files[0]);
      form.append('project', document.getElementById('project').value.trim());
      form.append('usage', document.getElementById('usage').value);
      form.append('member', document.getElementById('isMember').checked ? 'true' : 'false');

      showToast('正在上传，请稍候...');
      
      try {
        var res = await fetch(getBaseUrl() + '/api/v1/media/upload', {
          method: 'POST',
          headers: getAuthHeaders(),
          body: form
        });
        
        var data = await res.json();
        setConsoleOutput({ status: res.status, body: data });

        if (res.status === 201) {
          showToast('上传成功！');
          loadMediaAssets(); // Refresh Explorer
          fileInput.value = ''; // Reset input
        } else {
          showToast(data.error || '上传失败', 'error');
        }
      } catch (e) {
        setConsoleOutput(String(e));
        showToast('上传异常', 'error');
      }
    }

    // Helper to load image with Authorization header if necessary
    async function loadImageSource(imgElement, url) {
      var headers = getAuthHeaders();
      if (!headers.Authorization) {
        imgElement.src = url;
        return;
      }
      try {
        var res = await fetch(url, { headers: headers });
        if (res.status === 200) {
          var blob = await res.blob();
          var objectURL = URL.createObjectURL(blob);
          // Revoke old object URL if exists to prevent memory leak
          if (imgElement.dataset.objectUrl) {
            URL.revokeObjectURL(imgElement.dataset.objectUrl);
          }
          imgElement.src = objectURL;
          imgElement.dataset.objectUrl = objectURL;
        } else {
          imgElement.src = url;
        }
      } catch (e) {
        imgElement.src = url;
      }
    }

    // Helper to open media asset with Authorization header if necessary
    async function openAsset(url) {
      var headers = getAuthHeaders();
      if (!headers.Authorization) {
        window.open(url, '_blank');
        return;
      }
      try {
        showToast('正在打开媒体文件...');
        var res = await fetch(url, { headers: headers });
        if (res.status === 200) {
          var blob = await res.blob();
          var objectURL = URL.createObjectURL(blob);
          window.open(objectURL, '_blank');
        } else {
          window.open(url, '_blank');
        }
      } catch (e) {
        window.open(url, '_blank');
      }
    }

    // 3. Load media assets list
    async function loadMediaAssets() {
      var listEl = document.getElementById('mediaAssetsList');
      
      try {
        var res = await fetch(getBaseUrl() + '/api/v1/media?limit=50', {
          headers: getAuthHeaders()
        });
        
        if (res.status !== 200) {
          listEl.innerHTML = '<tr><td colspan="7" style="text-align: center; color: var(--danger); padding: 40px 0;">加载失败 (HTTP ' + res.status + ')，请验证 Token。</td></tr>';
          return;
        }

        var data = await res.json();
        
        if (!data || data.length === 0) {
          listEl.innerHTML = '<tr><td colspan="7" style="text-align: center; color: var(--text-muted); padding: 40px 0;">库中暂无媒体资源。上传文件后点击“刷新”。</td></tr>';
          return;
        }

        // Revoke old object URLs before clearing list to prevent memory leak
        listEl.querySelectorAll('img[data-object-url]').forEach(function(img) {
          if (img.dataset.objectUrl) {
            URL.revokeObjectURL(img.dataset.objectUrl);
          }
        });

        listEl.innerHTML = '';
        data.forEach(function(asset) {
          var isImage = asset.mimeType.startsWith('image/');
          var isVideo = asset.mimeType.startsWith('video/');
          var previewHTML = '<span style="font-size: 20px;">📄</span>';
          
          if (isImage && asset.status === 'active') {
            previewHTML = '<img src="" alt="preview" />';
          } else if (isVideo && asset.status === 'active') {
            previewHTML = '<span style="font-size: 20px;">🎬</span>';
          }
          
          var tr = document.createElement('tr');
          var badgeClass = asset.status === 'active' ? 'background: rgba(16, 185, 129, 0.15); color: #34d399;' : 'background: rgba(239, 68, 68, 0.15); color: #f87171;';
          var sizeStr = formatBytes(asset.sizeBytes);
          
          var actionsHTML = '';
          if (asset.status === 'active') {
            actionsHTML = '<button class="btn btn-secondary btn-sm" onclick="openAsset(\'' + asset.publicUrl + '\')">访问</button>' +
                          '<button class="btn btn-secondary btn-sm" onclick="copyText(\'' + asset.publicUrl + '\')">复制链接</button>';
          }
          var deleteHTML = '';
          if (asset.status === 'active') {
            deleteHTML = '<button class="btn btn-danger btn-sm" onclick="deleteAsset(\'' + asset.id + '\')">删除</button>';
          }

          tr.innerHTML = 
            '<td>' +
              '<div class="media-preview">' + previewHTML + '</div>' +
            '</td>' +
            '<td>' +
              '<div style="font-weight: 600; font-size: 13px; color: #fff; max-width: 150px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" title="' + asset.id + '">' +
                asset.id +
              '</div>' +
            '</td>' +
            '<td style="color: var(--text-muted); font-size: 13px;">' + asset.mimeType + '</td>' +
            '<td style="font-family: monospace;">' + sizeStr + '</td>' +
            '<td>' +
              '<span style="background: rgba(255,255,255,0.05); padding: 3px 6px; border-radius: 4px; font-size: 12px; margin-right: 4px;">' + asset.project + '</span>' +
              '<span style="color: var(--text-muted); font-size: 12px;">' + asset.usage + '</span>' +
            '</td>' +
            '<td>' +
              '<span style="padding: 4px 8px; border-radius: 12px; font-size: 11px; font-weight: 600; ' + badgeClass + '">' +
                asset.status +
              '</span>' +
            '</td>' +
            '<td>' +
              '<div class="action-group">' +
                actionsHTML +
                '<button class="btn btn-secondary btn-sm" onclick="getAssetMeta(\'' + asset.id + '\')">Meta</button>' +
                deleteHTML +
              '</div>' +
            '</td>';
            
          listEl.appendChild(tr);

          if (isImage && asset.status === 'active') {
            var imgEl = tr.querySelector('.media-preview img');
            if (imgEl) {
              loadImageSource(imgEl, asset.publicUrl);
            }
          }
        });

      } catch (e) {
        listEl.innerHTML = '<tr><td colspan="7" style="text-align: center; color: var(--danger); padding: 40px 0;">加载异常: ' + e.message + '</td></tr>';
      }
    }

    // 4. Get metadata
    async function getAssetMeta(id) {
      try {
        var res = await fetch(getBaseUrl() + '/api/v1/media/' + encodeURIComponent(id) + '/meta', {
          headers: getAuthHeaders()
        });
        var data = await res.json();
        setConsoleOutput({ status: res.status, body: data });
        showToast('已查询 Meta 数据');
      } catch (e) {
        setConsoleOutput(String(e));
        showToast('查询失败', 'error');
      }
    }

    // 5. Delete asset
    async function deleteAsset(id) {
      if (!confirm('确定要删除该媒体文件吗？此操作将标记文件为已删除。')) {
        return;
      }
      try {
        var res = await fetch(getBaseUrl() + '/api/v1/media/' + encodeURIComponent(id), {
          method: 'DELETE',
          headers: getAuthHeaders()
        });
        var data = await res.json();
        setConsoleOutput({ status: res.status, body: data });
        if (res.status === 200) {
          showToast('删除成功');
          loadMediaAssets();
        } else {
          showToast(data.error || '删除失败', 'error');
        }
      } catch (e) {
        setConsoleOutput(String(e));
        showToast('删除异常', 'error');
      }
    }

    // Helper: copy text
    function copyText(text) {
      navigator.clipboard.writeText(text).then(function() {
        showToast('链接已复制到剪贴板！');
      });
    }
  </script>
</body>
</html>`
