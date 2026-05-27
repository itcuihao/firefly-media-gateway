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
  <title>Media Gateway Debug UI</title>
  <style>
    :root { --bg:#f6f8fb; --card:#ffffff; --text:#1f2937; --muted:#6b7280; --line:#d1d5db; --accent:#0f766e; }
    body { margin:0; font-family: ui-sans-serif, -apple-system, Segoe UI, Helvetica, Arial; background:linear-gradient(180deg,#eef2ff,#f6f8fb); color:var(--text); }
    .wrap { max-width: 980px; margin: 24px auto; padding: 0 16px; }
    .card { background:var(--card); border:1px solid var(--line); border-radius:12px; padding:16px; margin-bottom:16px; }
    h1,h2 { margin:0 0 12px 0; }
    h1 { font-size:22px; }
    h2 { font-size:18px; }
    .row { display:flex; gap:12px; flex-wrap:wrap; }
    .field { display:flex; flex-direction:column; gap:6px; min-width:220px; flex:1; }
    label { color:var(--muted); font-size:13px; }
    input, select, button, textarea { border:1px solid var(--line); border-radius:8px; padding:10px; font-size:14px; }
    textarea { min-height: 180px; width:100%; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
    button { background:var(--accent); color:#fff; border:none; cursor:pointer; }
    button.secondary { background:#475569; }
    .hint { color:var(--muted); font-size:13px; }
  </style>
</head>
<body>
  <div class="wrap">
    <h1>Media Gateway Debug UI</h1>
    <p class="hint">用于联调上传、访问、元数据查询、删除。</p>

    <div class="card">
      <h2>全局配置</h2>
      <div class="row">
        <div class="field">
          <label>Base URL</label>
          <input id="baseUrl" value="" placeholder="http://localhost:8080" />
        </div>
        <div class="field">
          <label>Bearer Token</label>
          <input id="token" type="password" placeholder="MEDIA_GATEWAY_TOKEN" />
        </div>
      </div>
    </div>

    <div class="card">
      <h2>上传</h2>
      <div class="row">
        <div class="field"><label>Project</label><input id="project" value="interactive-video" /></div>
        <div class="field"><label>Usage</label><select id="usage"><option value="cover">cover</option><option value="scene">scene</option></select></div>
      </div>
      <div class="row">
        <div class="field"><label>File</label><input id="file" type="file" /></div>
      </div>
      <div class="row">
        <button onclick="upload()">上传</button>
      </div>
    </div>

    <div class="card">
      <h2>按 mediaId 操作</h2>
      <div class="row">
        <div class="field"><label>mediaId</label><input id="mediaId" placeholder="uuid" /></div>
      </div>
      <div class="row">
        <button class="secondary" onclick="openMedia()">访问媒体（新标签）</button>
        <button onclick="getMeta()">查询 Meta</button>
        <button onclick="deleteMedia()">删除媒体</button>
      </div>
    </div>

    <div class="card">
      <h2>响应</h2>
      <textarea id="output" readonly></textarea>
    </div>
  </div>

  <script>
    (function initBaseUrl() {
      const local = window.location.origin;
      document.getElementById('baseUrl').value = local;
    })();

    function authHeader() {
      const token = document.getElementById('token').value.trim();
      return token ? { 'Authorization': 'Bearer ' + token } : {};
    }

    function setOutput(obj) {
      const el = document.getElementById('output');
      el.value = typeof obj === 'string' ? obj : JSON.stringify(obj, null, 2);
    }

    function base() {
      return document.getElementById('baseUrl').value.trim().replace(/\/$/, '');
    }

    async function upload() {
      try {
        const fileInput = document.getElementById('file');
        if (!fileInput.files || !fileInput.files[0]) {
          setOutput('请先选择文件');
          return;
        }
        const form = new FormData();
        form.append('file', fileInput.files[0]);
        form.append('project', document.getElementById('project').value.trim());
        form.append('usage', document.getElementById('usage').value);

        const res = await fetch(base() + '/api/v1/media/upload', {
          method: 'POST',
          headers: authHeader(),
          body: form
        });
        const text = await res.text();
        let parsed;
        try { parsed = JSON.parse(text); } catch { parsed = text; }
        if (parsed && parsed.mediaId) {
          document.getElementById('mediaId').value = parsed.mediaId;
        }
        setOutput({ status: res.status, body: parsed });
      } catch (e) {
        setOutput(String(e));
      }
    }

    async function getMeta() {
      try {
        const id = document.getElementById('mediaId').value.trim();
        const res = await fetch(base() + '/api/v1/media/' + encodeURIComponent(id) + '/meta', {
          headers: authHeader()
        });
        const text = await res.text();
        let parsed;
        try { parsed = JSON.parse(text); } catch { parsed = text; }
        setOutput({ status: res.status, body: parsed });
      } catch (e) {
        setOutput(String(e));
      }
    }

    async function deleteMedia() {
      try {
        const id = document.getElementById('mediaId').value.trim();
        const res = await fetch(base() + '/api/v1/media/' + encodeURIComponent(id), {
          method: 'DELETE',
          headers: authHeader()
        });
        const text = await res.text();
        let parsed;
        try { parsed = JSON.parse(text); } catch { parsed = text; }
        setOutput({ status: res.status, body: parsed });
      } catch (e) {
        setOutput(String(e));
      }
    }

    function openMedia() {
      const id = document.getElementById('mediaId').value.trim();
      if (!id) {
        setOutput('请先输入 mediaId');
        return;
      }
      window.open(base() + '/api/v1/media/' + encodeURIComponent(id), '_blank');
    }
  </script>
</body>
</html>`
