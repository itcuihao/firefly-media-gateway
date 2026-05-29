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
  <title>Firefly Media Gateway - Control Center</title>
  
  <!-- Modern Typography from Google Fonts -->
  <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet" />
  <!-- Material Symbols Rounded -->
  <link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Rounded:opsz,wght,FILL,GRAD@24,400,0..1,0" rel="stylesheet" />

  <style>
    :root {
      /* Material Design 3 Dark Theme Palette (HSL Tailored) */
      --md-sys-color-primary: 185, 100%, 45%;            /* Vibrant Aqua/Cyan */
      --md-sys-color-on-primary: 185, 100%, 12%;
      --md-sys-color-primary-container: 185, 100%, 18%;
      --md-sys-color-on-primary-container: 185, 100%, 80%;
      
      --md-sys-color-secondary: 205, 30%, 65%;           /* Soft Steel Blue */
      --md-sys-color-on-secondary: 205, 35%, 15%;
      --md-sys-color-secondary-container: 205, 30%, 25%;
      
      --md-sys-color-background: 215, 28%, 8%;           /* Ultra deep midnight slate */
      --md-sys-color-on-background: 210, 20%, 92%;
      
      --md-sys-color-surface: 216, 24%, 12%;             /* Material surface level 1 */
      --md-sys-color-surface-container: 216, 24%, 15%;   /* Material surface level 2 */
      --md-sys-color-on-surface: 210, 20%, 92%;
      --md-sys-color-on-surface-variant: 210, 12%, 75%;
      
      --md-sys-color-outline: 210, 10%, 40%;
      --md-sys-color-outline-variant: 210, 12%, 24%;
      
      --md-sys-color-error: 0, 97%, 77%;
      --md-sys-color-on-error: 0, 100%, 15%;
      --md-sys-color-success: 145, 80%, 65%;
      --md-sys-color-on-success: 145, 100%, 12%;
      
      /* Elevations */
      --elevation-1: 0px 1px 3px 1px rgba(0, 0, 0, 0.15), 0px 1px 2px 0px rgba(0, 0, 0, 0.3);
      --elevation-2: 0px 2px 6px 2px rgba(0, 0, 0, 0.15), 0px 1px 2px 0px rgba(0, 0, 0, 0.3);
      --elevation-3: 0px 4px 8px 3px rgba(0, 0, 0, 0.15), 0px 1px 3px 0px rgba(0, 0, 0, 0.3);

      /* Transitions */
      --easing-standard: cubic-bezier(0.2, 0, 0, 1);
      --duration-standard: 0.3s;
    }

    * {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
      font-family: 'Outfit', sans-serif;
    }

    body {
      background: radial-gradient(circle at 50% 0%, hsl(185, 60%, 13%), hsl(var(--md-sys-color-background)) 65%);
      color: hsl(var(--md-sys-color-on-background));
      min-height: 100vh;
      overflow-x: hidden;
      display: flex;
    }

    /* Scrollbar Styling */
    ::-webkit-scrollbar {
      width: 8px;
      height: 8px;
    }
    ::-webkit-scrollbar-track {
      background: transparent;
    }
    ::-webkit-scrollbar-thumb {
      background: rgba(255, 255, 255, 0.12);
      border-radius: 8px;
    }
    ::-webkit-scrollbar-thumb:hover {
      background: rgba(255, 255, 255, 0.24);
    }

    /* Layout Wrapper */
    .app-layout {
      display: flex;
      width: 100%;
      min-height: 100vh;
    }

    /* Navigation Drawer (Sidebar) */
    .nav-drawer {
      width: 280px;
      background: hsl(var(--md-sys-color-surface));
      border-right: 1px solid rgba(255, 255, 255, 0.06);
      display: flex;
      flex-direction: column;
      padding: 24px 16px;
      position: fixed;
      top: 0;
      bottom: 0;
      left: 0;
      z-index: 100;
      transition: transform var(--duration-standard) var(--easing-standard);
    }

    .nav-brand {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 32px;
      padding-left: 12px;
    }

    .logo-wrapper {
      position: relative;
      display: inline-block;
      width: 42px;
      height: 42px;
    }

    .logo-img {
      width: 100%;
      height: 100%;
      object-fit: contain;
      filter: drop-shadow(0 0 8px rgba(0, 229, 255, 0.5));
      transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    }

    .logo-wrapper:hover .logo-img {
      transform: scale(1.1) rotate(6deg);
    }

    .logo-badge {
      position: absolute;
      top: -4px;
      right: -8px;
      background: linear-gradient(135deg, #fbbf24, #f59e0b);
      color: #030712;
      font-size: 9px;
      font-weight: 800;
      padding: 1px 4px;
      border-radius: 9999px;
      box-shadow: 0 0 6px rgba(245, 158, 11, 0.6);
      white-space: nowrap;
      pointer-events: none;
      animation: logoPulse 2.5s infinite;
      border: 1px solid rgba(3, 7, 18, 0.8);
      letter-spacing: 0.05em;
      line-height: 1;
    }

    @keyframes logoPulse {
      0%, 100% {
        transform: scale(1);
        box-shadow: 0 0 6px rgba(245, 158, 11, 0.6);
      }
      50% {
        transform: scale(1.08);
        box-shadow: 0 0 10px rgba(245, 158, 11, 0.9);
      }
    }

    .nav-brand h1 {
      font-size: 18px;
      font-weight: 700;
      letter-spacing: -0.2px;
      background: linear-gradient(135deg, hsl(var(--md-sys-color-primary)), #fff);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }

    .nav-menu {
      list-style: none;
      display: flex;
      flex-direction: column;
      gap: 4px;
      flex: 1;
    }

    .nav-item {
      display: flex;
      align-items: center;
      gap: 16px;
      padding: 14px 16px;
      color: hsl(var(--md-sys-color-on-surface-variant));
      text-decoration: none;
      border-radius: 100px;
      font-size: 14px;
      font-weight: 500;
      cursor: pointer;
      transition: all 0.2s ease;
    }

    .nav-item:hover {
      background: rgba(255, 255, 255, 0.05);
      color: hsl(var(--md-sys-color-on-surface));
    }

    .nav-item.active {
      background: hsl(var(--md-sys-color-primary-container));
      color: hsl(var(--md-sys-color-on-primary-container));
    }

    .nav-item .material-symbols-rounded {
      font-size: 22px;
    }

    /* Main Content Area */
    .main-wrapper {
      flex: 1;
      margin-left: 280px;
      display: flex;
      flex-direction: column;
      min-width: 0;
    }

    /* Top App Bar */
    .top-app-bar {
      height: 72px;
      background: rgba(13, 18, 22, 0.4);
      backdrop-filter: blur(20px);
      -webkit-backdrop-filter: blur(20px);
      border-bottom: 1px solid rgba(255, 255, 255, 0.05);
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0 32px;
      position: sticky;
      top: 0;
      z-index: 90;
    }

    .page-title {
      font-size: 20px;
      font-weight: 600;
      display: flex;
      align-items: center;
      gap: 12px;
    }

    .menu-toggle {
      display: none;
      background: none;
      border: none;
      color: inherit;
      cursor: pointer;
    }

    .global-actions {
      display: flex;
      align-items: center;
      gap: 12px;
    }

    /* Global Config Dropdown */
    .config-trigger {
      display: flex;
      align-items: center;
      gap: 8px;
      background: rgba(255, 255, 255, 0.05);
      border: 1px solid rgba(255, 255, 255, 0.08);
      border-radius: 100px;
      padding: 8px 16px;
      font-size: 13px;
      color: hsl(var(--md-sys-color-on-surface-variant));
      cursor: pointer;
      transition: all 0.2s;
    }

    .config-trigger:hover, .config-trigger.active {
      background: rgba(255, 255, 255, 0.1);
      border-color: rgba(255, 255, 255, 0.2);
      color: #fff;
    }

    .config-dropdown {
      position: absolute;
      top: 80px;
      right: 32px;
      width: 320px;
      background: hsl(var(--md-sys-color-surface-container));
      border: 1px solid rgba(255, 255, 255, 0.1);
      border-radius: 20px;
      box-shadow: var(--elevation-3);
      padding: 20px;
      display: none;
      flex-direction: column;
      gap: 16px;
      z-index: 120;
      animation: scaleIn 0.2s cubic-bezier(0.2, 0, 0, 1);
    }

    @keyframes scaleIn {
      from { transform: scale(0.95); opacity: 0; }
      to { transform: scale(1); opacity: 1; }
    }

    /* Content Body */
    .content-body {
      padding: 32px;
      flex: 1;
      display: flex;
      flex-direction: column;
    }

    .panel-view {
      display: none;
      flex-direction: column;
      gap: 24px;
      animation: fadeIn 0.4s var(--easing-standard);
    }

    .panel-view.active {
      display: flex;
    }

    @keyframes fadeIn {
      from { opacity: 0; transform: translateY(10px); }
      to { opacity: 1; transform: translateY(0); }
    }

    /* Typography & Core components */
    h2.section-title {
      font-size: 18px;
      font-weight: 600;
      margin-bottom: 8px;
      display: flex;
      align-items: center;
      gap: 8px;
    }

    /* Material Cards */
    .m3-card {
      background: rgba(19, 27, 32, 0.6);
      backdrop-filter: blur(12px);
      -webkit-backdrop-filter: blur(12px);
      border: 1px solid rgba(255, 255, 255, 0.05);
      border-radius: 24px;
      padding: 24px;
      transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    }

    .m3-card.hoverable:hover {
      background: rgba(24, 35, 41, 0.85);
      border-color: rgba(0, 229, 255, 0.2);
      box-shadow: 0 8px 24px -8px rgba(0, 0, 0, 0.5), 0 0 16px rgba(0, 229, 255, 0.08);
      transform: translateY(-2px);
    }

    /* Grid layouts */
    .m3-grid-2 {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
      gap: 24px;
    }

    .m3-grid-3 {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
      gap: 24px;
    }

    /* Input Styling */
    .form-field {
      display: flex;
      flex-direction: column;
      gap: 6px;
      margin-bottom: 16px;
    }

    .form-field label {
      font-size: 12px;
      font-weight: 600;
      color: hsl(var(--md-sys-color-primary));
      letter-spacing: 0.5px;
      text-transform: uppercase;
    }

    .input-wrapper {
      position: relative;
      display: flex;
      align-items: center;
    }

    .input-wrapper input, .input-wrapper select, .input-wrapper textarea {
      width: 100%;
      background: rgba(255, 255, 255, 0.04);
      border: 1px solid rgba(255, 255, 255, 0.1);
      border-radius: 12px;
      color: #fff;
      padding: 12px 16px;
      font-size: 14px;
      outline: none;
      transition: all 0.2s;
    }

    .input-wrapper input:focus, .input-wrapper select:focus, .input-wrapper textarea:focus {
      border-color: hsl(var(--md-sys-color-primary));
      background: rgba(255, 255, 255, 0.08);
      box-shadow: 0 0 0 3px rgba(0, 229, 255, 0.15);
    }

    .input-icon-btn {
      position: absolute;
      right: 12px;
      background: none;
      border: none;
      color: hsl(var(--md-sys-color-on-surface-variant));
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
    }

    /* Button systems */
    .m3-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      border-radius: 100px;
      padding: 12px 24px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      transition: all 0.2s cubic-bezier(0.2, 0, 0, 1);
      border: none;
      outline: none;
    }

    .m3-btn-primary {
      background: hsl(var(--md-sys-color-primary));
      color: hsl(var(--md-sys-color-on-primary));
    }

    .m3-btn-primary:hover {
      box-shadow: 0 4px 12px rgba(0, 229, 255, 0.3);
      filter: brightness(1.15);
      transform: translateY(-1px);
    }

    .m3-btn-secondary {
      background: rgba(255, 255, 255, 0.06);
      color: hsl(var(--md-sys-color-on-surface));
      border: 1px solid rgba(255, 255, 255, 0.1);
    }

    .m3-btn-secondary:hover {
      background: rgba(255, 255, 255, 0.12);
      border-color: rgba(255, 255, 255, 0.2);
    }

    .m3-btn-danger {
      background: rgba(255, 180, 171, 0.15);
      color: #ffb4ab;
      border: 1px solid rgba(255, 180, 171, 0.25);
    }

    .m3-btn-danger:hover {
      background: rgba(255, 180, 171, 0.25);
      border-color: rgba(255, 180, 171, 0.4);
    }

    .m3-btn-sm {
      padding: 8px 16px;
      font-size: 12px;
    }

    /* Stat Card Specifics */
    .stat-card {
      display: flex;
      align-items: center;
      gap: 20px;
    }

    .stat-icon {
      width: 56px;
      height: 56px;
      border-radius: 16px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 26px;
    }

    .stat-icon.primary {
      background: rgba(0, 229, 255, 0.1);
      color: hsl(var(--md-sys-color-primary));
    }

    .stat-icon.secondary {
      background: rgba(176, 235, 244, 0.1);
      color: hsl(var(--md-sys-color-secondary));
    }

    .stat-icon.success {
      background: rgba(133, 247, 176, 0.1);
      color: hsl(var(--md-sys-color-success));
    }

    .stat-info {
      display: flex;
      flex-direction: column;
    }

    .stat-val {
      font-size: 26px;
      font-weight: 700;
      color: #fff;
    }

    .stat-label {
      font-size: 13px;
      color: hsl(var(--md-sys-color-on-surface-variant));
    }

    /* Media Explorer filters */
    .media-filter-bar {
      display: flex;
      flex-wrap: wrap;
      gap: 16px;
      align-items: center;
      justify-content: space-between;
    }

    .filter-inputs {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
      align-items: center;
      flex: 1;
    }

    .filter-input {
      background: rgba(255, 255, 255, 0.05);
      border: 1px solid rgba(255, 255, 255, 0.08);
      border-radius: 12px;
      color: #fff;
      padding: 10px 14px;
      font-size: 13px;
      outline: none;
    }

    .filter-input:focus {
      border-color: hsl(var(--md-sys-color-primary));
    }

    .search-field {
      min-width: 240px;
      flex: 1;
    }

    .view-toggle-btns {
      display: flex;
      background: rgba(255, 255, 255, 0.04);
      padding: 4px;
      border-radius: 100px;
      border: 1px solid rgba(255, 255, 255, 0.08);
    }

    .view-toggle-btn {
      background: none;
      border: none;
      color: hsl(var(--md-sys-color-on-surface-variant));
      padding: 6px 12px;
      border-radius: 100px;
      cursor: pointer;
      display: flex;
      align-items: center;
      font-size: 13px;
      gap: 6px;
      transition: all 0.2s;
    }

    .view-toggle-btn.active {
      background: hsl(var(--md-sys-color-primary-container));
      color: hsl(var(--md-sys-color-on-primary-container));
    }

    /* Media Layouts (Grid/List) */
    .media-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
      gap: 20px;
    }

    .media-card {
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.06);
      border-radius: 20px;
      overflow: hidden;
      position: relative;
      transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
    }

    .media-card:hover {
      transform: translateY(-4px);
      border-color: rgba(0, 229, 255, 0.25);
      background: rgba(255, 255, 255, 0.06);
      box-shadow: var(--elevation-2);
    }

    .media-thumb {
      height: 140px;
      background: #0d1216;
      display: flex;
      align-items: center;
      justify-content: center;
      position: relative;
      overflow: hidden;
    }

    .media-thumb img, .media-thumb video {
      width: 100%;
      height: 100%;
      object-fit: cover;
    }

    .media-thumb .file-icon {
      font-size: 42px;
      color: hsl(var(--md-sys-color-primary));
    }

    .media-card-info {
      padding: 16px;
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    .media-id {
      font-size: 14px;
      font-weight: 600;
      color: #fff;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .media-meta-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      font-size: 11px;
      color: hsl(var(--md-sys-color-on-surface-variant));
    }

    .media-card-tags {
      display: flex;
      flex-wrap: wrap;
      gap: 4px;
    }

    .badge {
      font-size: 10px;
      font-weight: 600;
      padding: 2px 6px;
      border-radius: 100px;
      background: rgba(255, 255, 255, 0.08);
      color: hsl(var(--md-sys-color-on-surface-variant));
    }

    .badge-primary {
      background: rgba(0, 229, 255, 0.12);
      color: hsl(var(--md-sys-color-primary));
    }

    .badge-success {
      background: rgba(133, 247, 176, 0.12);
      color: hsl(var(--md-sys-color-success));
    }

    .badge-error {
      background: rgba(255, 180, 171, 0.12);
      color: #ffb4ab;
    }

    .media-actions {
      display: flex;
      gap: 6px;
      border-top: 1px solid rgba(255, 255, 255, 0.05);
      padding: 12px 16px;
      background: rgba(0, 0, 0, 0.1);
    }

    /* List table */
    .m3-table-wrapper {
      overflow-x: auto;
      border-radius: 16px;
      border: 1px solid rgba(255, 255, 255, 0.06);
    }

    .m3-table {
      width: 100%;
      border-collapse: collapse;
      text-align: left;
      font-size: 13px;
    }

    .m3-table th, .m3-table td {
      padding: 14px 18px;
      border-bottom: 1px solid rgba(255, 255, 255, 0.05);
    }

    .m3-table th {
      background: rgba(255, 255, 255, 0.03);
      font-weight: 600;
      text-transform: uppercase;
      font-size: 11px;
      color: hsl(var(--md-sys-color-primary));
      letter-spacing: 0.5px;
    }

    .m3-table tr:hover td {
      background: rgba(255, 255, 255, 0.02);
    }

    /* Floating Action Button */
    .fab {
      position: fixed;
      bottom: 32px;
      right: 32px;
      width: 64px;
      height: 64px;
      border-radius: 20px;
      background: hsl(var(--md-sys-color-primary));
      color: hsl(var(--md-sys-color-on-primary));
      display: flex;
      align-items: center;
      justify-content: center;
      box-shadow: var(--elevation-3);
      cursor: pointer;
      border: none;
      z-index: 85;
      transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
    }

    .fab:hover {
      transform: scale(1.08) rotate(90deg);
      background: #8bf0ff;
    }

    /* API Sandbox Design */
    .api-sandbox-layout {
      display: flex;
      gap: 24px;
      flex-wrap: wrap;
    }

    .api-params-col {
      flex: 1 1 350px;
    }

    .api-response-col {
      flex: 2 2 500px;
      display: flex;
      flex-direction: column;
    }

    .console-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 12px;
    }

    .status-pill {
      font-family: 'JetBrains Mono', monospace;
      font-size: 12px;
      font-weight: 600;
      padding: 4px 10px;
      border-radius: 6px;
      display: inline-flex;
      align-items: center;
      gap: 6px;
    }

    .console-output {
      flex: 1;
      min-height: 400px;
      background: #070b0e;
      border: 1px solid rgba(255, 255, 255, 0.08);
      border-radius: 16px;
      padding: 20px;
      color: #79c0ff;
      font-family: 'JetBrains Mono', monospace;
      font-size: 13px;
      line-height: 1.6;
      overflow: auto;
      white-space: pre-wrap;
    }

    /* Bot Verifier Cards */
    .tab-nav {
      display: flex;
      border-bottom: 1px solid rgba(255, 255, 255, 0.08);
      margin-bottom: 24px;
    }

    .tab-btn {
      background: none;
      border: none;
      color: hsl(var(--md-sys-color-on-surface-variant));
      padding: 12px 24px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      position: relative;
    }

    .tab-btn.active {
      color: hsl(var(--md-sys-color-primary));
    }

    .tab-btn.active::after {
      content: '';
      position: absolute;
      bottom: -1px;
      left: 0;
      right: 0;
      height: 3px;
      background: hsl(var(--md-sys-color-primary));
      border-top-left-radius: 3px;
      border-top-right-radius: 3px;
    }

    .bot-details-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: 16px;
      margin-top: 16px;
    }

    .bot-detail-item {
      background: rgba(255, 255, 255, 0.03);
      padding: 12px 16px;
      border-radius: 12px;
      border: 1px solid rgba(255, 255, 255, 0.05);
    }

    .bot-detail-label {
      font-size: 11px;
      color: hsl(var(--md-sys-color-on-surface-variant));
      text-transform: uppercase;
      margin-bottom: 4px;
    }

    .bot-detail-value {
      font-size: 14px;
      font-weight: 600;
      color: #fff;
      word-break: break-all;
    }

    .chat-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
      gap: 16px;
      margin-top: 20px;
    }

    .chat-card {
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid rgba(255, 255, 255, 0.06);
      border-radius: 16px;
      padding: 16px;
      display: flex;
      justify-content: space-between;
      align-items: center;
      transition: all 0.2s;
    }

    .chat-card:hover {
      background: rgba(255, 255, 255, 0.06);
      border-color: rgba(0, 229, 255, 0.15);
    }

    .chat-card-info {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }

    .chat-card-title {
      font-size: 14px;
      font-weight: 600;
      color: #fff;
    }

    .chat-card-id {
      font-family: 'JetBrains Mono', monospace;
      font-size: 12px;
      color: hsl(var(--md-sys-color-on-surface-variant));
    }

    /* Modal / Dialog Dialogs */
    .m3-dialog-overlay {
      position: fixed;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background: rgba(0, 0, 0, 0.7);
      backdrop-filter: blur(8px);
      -webkit-backdrop-filter: blur(8px);
      display: none;
      align-items: center;
      justify-content: center;
      z-index: 200;
      padding: 16px;
    }

    .m3-dialog-overlay.active {
      display: flex;
      animation: dialogBgIn 0.3s ease;
    }

    .m3-dialog {
      background: hsl(var(--md-sys-color-surface));
      border: 1px solid rgba(255, 255, 255, 0.1);
      border-radius: 28px;
      width: 100%;
      max-width: 500px;
      box-shadow: var(--elevation-3);
      overflow: hidden;
      animation: dialogIn 0.3s cubic-bezier(0.2, 0, 0, 1);
    }

    @keyframes dialogBgIn {
      from { opacity: 0; }
      to { opacity: 1; }
    }

    @keyframes dialogIn {
      from { transform: scale(0.9) translateY(20px); opacity: 0; }
      to { transform: scale(1) translateY(0); opacity: 1; }
    }

    .m3-dialog-header {
      padding: 24px 24px 16px 24px;
      display: flex;
      justify-content: space-between;
      align-items: center;
    }

    .m3-dialog-header h3 {
      font-size: 20px;
      font-weight: 600;
      color: #fff;
    }

    .m3-dialog-close {
      background: none;
      border: none;
      color: hsl(var(--md-sys-color-on-surface-variant));
      cursor: pointer;
      font-size: 24px;
    }

    .m3-dialog-body {
      padding: 0 24px 24px 24px;
    }

    .m3-dialog-footer {
      padding: 16px 24px 24px 24px;
      display: flex;
      justify-content: flex-end;
      gap: 12px;
      border-top: 1px solid rgba(255, 255, 255, 0.05);
    }

    /* Slide drawer for detail */
    .m3-sheet {
      position: fixed;
      top: 0;
      right: -420px;
      bottom: 0;
      width: 400px;
      background: hsl(var(--md-sys-color-surface));
      border-left: 1px solid rgba(255, 255, 255, 0.08);
      box-shadow: var(--elevation-3);
      z-index: 150;
      display: flex;
      flex-direction: column;
      transition: right var(--duration-standard) var(--easing-standard);
      padding: 32px 24px;
    }

    .m3-sheet.active {
      right: 0;
    }

    .m3-sheet-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 24px;
    }

    .m3-sheet-body {
      flex: 1;
      overflow-y: auto;
      display: flex;
      flex-direction: column;
      gap: 20px;
    }

    /* Toast system */
    .toast-container {
      position: fixed;
      bottom: 32px;
      left: 50%;
      transform: translateX(-50%);
      z-index: 1000;
      display: flex;
      flex-direction: column;
      gap: 8px;
      pointer-events: none;
    }

    .toast {
      background: hsl(var(--md-sys-color-surface-container));
      border: 1px solid rgba(255, 255, 255, 0.08);
      color: #fff;
      padding: 14px 24px;
      border-radius: 100px;
      font-size: 13px;
      font-weight: 500;
      box-shadow: var(--elevation-2);
      display: flex;
      align-items: center;
      gap: 10px;
      animation: toastIn 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
      pointer-events: auto;
    }

    .toast-success {
      border-color: rgba(133, 247, 176, 0.3);
      color: hsl(var(--md-sys-color-success));
    }

    .toast-error {
      border-color: rgba(255, 180, 171, 0.3);
      color: #ffb4ab;
    }

    @keyframes toastIn {
      from { transform: translateY(20px); opacity: 0; }
      to { transform: translateY(0); opacity: 1; }
    }

    /* Responsive adjustments */
    @media (max-width: 900px) {
      .nav-drawer {
        transform: translateX(-100%);
      }
      .nav-drawer.active {
        transform: translateX(0);
      }
      .main-wrapper {
        margin-left: 0;
      }
      .menu-toggle {
        display: block;
        margin-right: 16px;
      }
      .page-title {
        font-size: 18px;
      }
    }
  </style>
</head>
<body>
  <!-- Toast Messages Container -->
  <div class="toast-container" id="toastContainer"></div>

  <!-- Main M3 Layout -->
  <div class="app-layout">
    
    <!-- Navigation Drawer -->
    <nav class="nav-drawer" id="navDrawer">
      <div class="nav-brand">
        <div class="logo-wrapper">
          <img src="data:image/png;base64,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" alt="Firefly Logo" class="logo-img" />
          <span class="logo-badge">嘿嘿</span>
        </div>
        <h1>Firefly Gateway</h1>
      </div>
      <ul class="nav-menu">
        <li>
          <a class="nav-item active" onclick="switchTab('dashboard')">
            <span class="material-symbols-rounded">dashboard</span>
            <span>仪表盘总览</span>
          </a>
        </li>
        <li>
          <a class="nav-item" onclick="switchTab('explorer')">
            <span class="material-symbols-rounded">folder_open</span>
            <span>媒体库管理器</span>
          </a>
        </li>
        <li>
          <a class="nav-item" onclick="switchTab('verifier')">
            <span class="material-symbols-rounded">vpn_key</span>
            <span>机器人连通验证</span>
          </a>
        </li>
        <li>
          <a class="nav-item" onclick="switchTab('sandbox')">
            <span class="material-symbols-rounded">science</span>
            <span>API 联调沙盒</span>
          </a>
        </li>
      </ul>
    </nav>

    <!-- Content Workspace -->
    <div class="main-wrapper">
      
      <!-- Top App Bar Header -->
      <header class="top-app-bar">
        <div style="display: flex; align-items: center;">
          <button class="menu-toggle" onclick="toggleDrawer()">
            <span class="material-symbols-rounded" style="font-size: 28px;">menu</span>
          </button>
          <div class="page-title" id="pageTitle">仪表盘总览</div>
        </div>

        <div class="global-actions">
          <button class="config-trigger" id="configBtn" onclick="toggleConfigDropdown()">
            <span class="material-symbols-rounded" style="font-size: 18px;">settings</span>
            <span>网关配置</span>
          </button>
          
          <!-- Quick Config dropdown panel -->
          <div class="config-dropdown" id="configDropdown">
            <h3 style="font-size: 15px; font-weight: 600; margin-bottom: 8px; border-bottom: 1px solid rgba(255,255,255,0.08); padding-bottom: 6px;">全局连接配置</h3>
            <div class="form-field">
              <label>API 基础地址</label>
              <div class="input-wrapper">
                <input id="baseUrl" type="text" placeholder="http://localhost:8080" />
              </div>
            </div>
            <div class="form-field">
              <label>网关 Bearer Token</label>
              <div class="input-wrapper">
                <input id="authToken" type="password" placeholder="输入 API Token" />
                <button class="input-icon-btn" onclick="togglePasswordVisibility('authToken')">
                  <span class="material-symbols-rounded" id="authToken_eye">visibility</span>
                </button>
              </div>
            </div>
            <button class="m3-btn m3-btn-primary m3-btn-sm" onclick="saveGlobalConfig()">保存配置</button>
          </div>
        </div>
      </header>

      <!-- App Body Panel Switcher -->
      <main class="content-body">
        
        <!-- Tab 1: Dashboard Panel -->
        <div class="panel-view active" id="panel_dashboard">
          <div class="m3-grid-3">
            <div class="m3-card stat-card">
              <div class="stat-icon primary">
                <span class="material-symbols-rounded">cloud_upload</span>
              </div>
              <div class="stat-info">
                <span class="stat-val" id="stat_total_count">--</span>
                <span class="stat-label">库中文件总数</span>
              </div>
            </div>
            <div class="m3-card stat-card">
              <div class="stat-icon secondary">
                <span class="material-symbols-rounded">database</span>
              </div>
              <div class="stat-info">
                <span class="stat-val" id="stat_total_size">--</span>
                <span class="stat-label">总计占用容量</span>
              </div>
            </div>
            <div class="m3-card stat-card">
              <div class="stat-icon success">
                <span class="material-symbols-rounded">health_and_safety</span>
              </div>
              <div class="stat-info">
                <span class="stat-val" id="stat_health_status">获取中</span>
                <span class="stat-label">网关服务状态</span>
              </div>
            </div>
          </div>

          <div class="m3-grid-2">
            <!-- Storage Info -->
            <div class="m3-card">
              <h2 class="section-title">
                <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-primary));">donut_large</span>
                媒体资源构成
              </h2>
              <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 24px;">网关内保存的各类媒体文件占比及详细数据。</p>
              
              <div style="display: flex; flex-direction: column; gap: 18px;">
                <div>
                  <div style="display: flex; justify-content: space-between; font-size: 13px; margin-bottom: 6px;">
                    <span>图片类 (Images)</span>
                    <span id="chart_img_txt" style="font-weight: 600;">--</span>
                  </div>
                  <div style="height: 8px; background: rgba(255,255,255,0.05); border-radius: 10px; overflow: hidden;">
                    <div id="chart_img_bar" style="width: 0%; height: 100%; background: hsl(var(--md-sys-color-primary)); transition: width 1s;"></div>
                  </div>
                </div>

                <div>
                  <div style="display: flex; justify-content: space-between; font-size: 13px; margin-bottom: 6px;">
                    <span>视频类 (Videos)</span>
                    <span id="chart_vid_txt" style="font-weight: 600;">--</span>
                  </div>
                  <div style="height: 8px; background: rgba(255,255,255,0.05); border-radius: 10px; overflow: hidden;">
                    <div id="chart_vid_bar" style="width: 0%; height: 100%; background: hsl(var(--md-sys-color-secondary)); transition: width 1s;"></div>
                  </div>
                </div>

                <div>
                  <div style="display: flex; justify-content: space-between; font-size: 13px; margin-bottom: 6px;">
                    <span>其他分片/归档 (Others)</span>
                    <span id="chart_other_txt" style="font-weight: 600;">--</span>
                  </div>
                  <div style="height: 8px; background: rgba(255,255,255,0.05); border-radius: 10px; overflow: hidden;">
                    <div id="chart_other_bar" style="width: 0%; height: 100%; background: #9ca3af; transition: width 1s;"></div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Server Environment -->
            <div class="m3-card">
              <h2 class="section-title">
                <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-success));">info</span>
                系统环境配置
              </h2>
              <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 20px;">运行中的媒体网关后端核心环境参数。</p>
              
              <table style="width: 100%; font-size: 13px; border-collapse: collapse;">
                <tr style="border-bottom: 1px solid rgba(255,255,255,0.05);">
                  <td style="padding: 10px 0; color: hsl(var(--md-sys-color-on-surface-variant));">接口鉴权状态</td>
                  <td id="env_auth_state" style="padding: 10px 0; text-align: right; font-weight: 600; color: #fff;">加载中</td>
                </tr>
                <tr style="border-bottom: 1px solid rgba(255,255,255,0.05);">
                  <td style="padding: 10px 0; color: hsl(var(--md-sys-color-on-surface-variant));">响应时区</td>
                  <td style="padding: 10px 0; text-align: right; font-family: monospace; color: #fff;">UTC / Local</td>
                </tr>
                <tr>
                  <td style="padding: 10px 0; color: hsl(var(--md-sys-color-on-surface-variant));">服务器版本</td>
                  <td style="padding: 10px 0; text-align: right; font-family: monospace; font-weight: 600; color: hsl(var(--md-sys-color-primary));">v1.2.0-release</td>
                </tr>
              </table>
            </div>
          </div>
        </div>

        <!-- Tab 2: Media Explorer Panel -->
        <div class="panel-view" id="panel_explorer">
          <!-- Filters Card -->
          <div class="m3-card media-filter-bar">
            <div class="filter-inputs">
              <input class="filter-input search-field" id="searchKeyword" placeholder="搜索资源 ID 或 MIME 格式..." oninput="applyFilters()" />
              <select class="filter-input" id="filterProject" onchange="applyFilters()">
                <option value="">全部项目 (Projects)</option>
              </select>
              <select class="filter-input" id="filterUsage" onchange="applyFilters()">
                <option value="">全部用途 (Usages)</option>
              </select>
              <label style="display: flex; align-items: center; gap: 8px; font-size: 13px; cursor: pointer; user-select: none;">
                <input type="checkbox" id="showDeleted" onchange="applyFilters()" style="accent-color: hsl(var(--md-sys-color-primary));" />
                <span>显示已删除资源</span>
              </label>
            </div>

            <div style="display: flex; gap: 12px; align-items: center;">
              <div class="view-toggle-btns">
                <button class="view-toggle-btn active" id="btn_view_grid" onclick="setExplorerLayout('grid')">
                  <span class="material-symbols-rounded" style="font-size: 18px;">grid_view</span>
                  <span>网格</span>
                </button>
                <button class="view-toggle-btn" id="btn_view_list" onclick="setExplorerLayout('list')">
                  <span class="material-symbols-rounded" style="font-size: 18px;">format_list_bulleted</span>
                  <span>列表</span>
                </button>
              </div>

              <button class="m3-btn m3-btn-secondary m3-btn-sm" onclick="loadMediaAssets()">
                <span class="material-symbols-rounded" style="font-size: 16px;">refresh</span>
                <span>刷新</span>
              </button>
            </div>
          </div>

          <!-- Assets Render Area -->
          <div id="explorerContainerGrid" class="media-grid">
            <!-- Grid cards injected dynamically -->
          </div>

          <div id="explorerContainerList" class="m3-table-wrapper" style="display: none;">
            <table class="m3-table">
              <thead>
                <tr>
                  <th style="width: 60px;">预览</th>
                  <th>ID</th>
                  <th>类型 (MIME)</th>
                  <th>大小</th>
                  <th>项目/用途</th>
                  <th>状态</th>
                  <th style="text-align: right;">操作</th>
                </tr>
              </thead>
              <tbody id="explorerListBody">
                <!-- List rows injected dynamically -->
              </tbody>
            </table>
          </div>
          
          <div id="explorerEmptyState" style="display: none; text-align: center; padding: 80px 0; color: hsl(var(--md-sys-color-on-surface-variant));">
            <span class="material-symbols-rounded" style="font-size: 64px; color: rgba(255,255,255,0.08); margin-bottom: 16px;">folder_off</span>
            <p style="font-size: 15px;">未检索到符合条件的媒体资源文件</p>
          </div>
        </div>

        <!-- Tab 3: Bot Connection Verification Panel -->
        <div class="panel-view" id="panel_verifier">
          <div class="tab-nav">
            <button class="tab-btn active" id="btn_tg_tab" onclick="switchVerifierTab('tg')">Telegram 机器人验证</button>
            <button class="tab-btn" id="btn_discord_tab" onclick="switchVerifierTab('discord')">Discord 机器人验证</button>
          </div>

          <!-- TG Tab Content -->
          <div id="verifier_tg_content" style="display: flex; flex-direction: column; gap: 24px;">
            <div class="m3-card">
              <h2 class="section-title">
                <span class="material-symbols-rounded" style="color: #229ED9;">send</span>
                Telegram Bot 在线校验与 Group ID 获取
              </h2>
              <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 24px;">
                输入您想要调试的 Telegram 机器人 Token。我们将调用 <code>getMe</code> 接口校验其真实性，并可通过 <code>getUpdates</code> 拉取最新加入的群组或频道。
              </p>

              <div class="m3-grid-2">
                <div>
                  <div class="form-field">
                    <label>Telegram Bot Token</label>
                    <div class="input-wrapper">
                      <input id="tgBotTokenInput" type="password" placeholder="123456789:ABCDefGhIjKlMnOpQrStUvWxYz" />
                      <button class="input-icon-btn" onclick="togglePasswordVisibility('tgBotTokenInput')">
                        <span class="material-symbols-rounded" id="tgBotTokenInput_eye">visibility</span>
                      </button>
                    </div>
                    <span style="font-size: 11px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-top: 4px;">为空则默认加载后端的 <code>TELEGRAM_BOT_TOKEN</code> 环境变量</span>
                  </div>

                  <div style="display: flex; gap: 12px; margin-top: 24px;">
                    <button class="m3-btn m3-btn-primary" onclick="verifyTelegramBot()">
                      <span class="material-symbols-rounded">verified_user</span>
                      <span>测试机器人连通性</span>
                    </button>
                    <button class="m3-btn m3-btn-secondary" onclick="fetchTelegramChatIDsPost()">
                      <span class="material-symbols-rounded">chat_bubble</span>
                      <span>获取最近群组 ID</span>
                    </button>
                  </div>
                </div>

                <!-- Bot Info Output Card -->
                <div style="background: rgba(0,0,0,0.15); border-radius: 20px; padding: 20px; border: 1px dashed rgba(255,255,255,0.06); display: flex; flex-direction: column; justify-content: center;">
                  <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 16px;">
                    <span id="tg_status_icon" class="material-symbols-rounded" style="font-size: 28px; color: rgba(255,255,255,0.2);">help</span>
                    <div>
                      <h4 style="font-size: 15px; font-weight: 600;" id="tg_status_title">未连接测试</h4>
                      <p style="font-size: 12px; color: hsl(var(--md-sys-color-on-surface-variant));" id="tg_status_desc">请点击左侧按钮进行通讯测试</p>
                    </div>
                  </div>

                  <div class="bot-details-grid" id="tg_bot_details" style="display: none;">
                    <div class="bot-detail-item">
                      <div class="bot-detail-label">Bot ID</div>
                      <div class="bot-detail-value" id="tg_bot_id">--</div>
                    </div>
                    <div class="bot-detail-item">
                      <div class="bot-detail-label">账号昵称</div>
                      <div class="bot-detail-value" id="tg_bot_name">--</div>
                    </div>
                    <div class="bot-detail-item">
                      <div class="bot-detail-label">用户名</div>
                      <div class="bot-detail-value" id="tg_bot_username">--</div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- TG Chats list output -->
            <div class="m3-card" id="tg_chats_card" style="display: none;">
              <h3 style="font-size: 15px; font-weight: 600; margin-bottom: 12px; display: flex; align-items: center; gap: 8px;">
                <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-primary));">groups</span>
                检测到的最新互动群组 / 频道 (Chat IDs)
              </h3>
              <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 16px;">
                注意：Bot 只能拉取到最近 24 小时内有新消息 of 群组。请在群组中艾特 Bot 发送测试消息，然后再次点击刷新。
              </p>
              
              <div class="chat-grid" id="tg_chats_container">
                <!-- chats dynamically loaded -->
              </div>
            </div>
          </div>

          <!-- Discord Tab Content -->
          <div id="verifier_discord_content" style="display: none; flex-direction: column; gap: 24px;">
            <div class="m3-card">
              <h2 class="section-title">
                <span class="material-symbols-rounded" style="color: #5865F2;">forum</span>
                Discord Bot 鉴权有效性及服务器 (Guild) ID 获取
              </h2>
              <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 24px;">
                输入您创建的 Discord 机器人 token。我们将使用 <code>Authorization: Bot &lt;token&gt;</code> 调用 Discord API 校验状态，并能列出该 Bot 目前加入的所有服务器（Guilds）。
              </p>

              <div class="m3-grid-2">
                <div>
                  <div class="form-field">
                    <label>Discord Bot Token</label>
                    <div class="input-wrapper">
                      <input id="discordBotTokenInput" type="password" placeholder="MTIzNDU2Nzg5MD... (输入完整的 Discord Bot 秘钥)" />
                      <button class="input-icon-btn" onclick="togglePasswordVisibility('discordBotTokenInput')">
                        <span class="material-symbols-rounded" id="discordBotTokenInput_eye">visibility</span>
                      </button>
                    </div>
                  </div>

                  <div style="display: flex; gap: 12px; margin-top: 24px;">
                    <button class="m3-btn m3-btn-primary" onclick="verifyDiscordBot()">
                      <span class="material-symbols-rounded">verified_user</span>
                      <span>测试 Bot 鉴权</span>
                    </button>
                    <button class="m3-btn m3-btn-secondary" onclick="fetchDiscordGuilds()">
                      <span class="material-symbols-rounded">dns</span>
                      <span>拉取加入的服务器</span>
                    </button>
                  </div>
                </div>

                <!-- Bot Info Output Card -->
                <div style="background: rgba(0,0,0,0.15); border-radius: 20px; padding: 20px; border: 1px dashed rgba(255,255,255,0.06); display: flex; flex-direction: column; justify-content: center;">
                  <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 16px;">
                    <span id="discord_status_icon" class="material-symbols-rounded" style="font-size: 28px; color: rgba(255,255,255,0.2);">help</span>
                    <div>
                      <h4 style="font-size: 15px; font-weight: 600;" id="discord_status_title">未连接测试</h4>
                      <p style="font-size: 12px; color: hsl(var(--md-sys-color-on-surface-variant));" id="discord_status_desc">请输入 Token 并测试</p>
                    </div>
                  </div>

                  <div class="bot-details-grid" id="discord_bot_details" style="display: none;">
                    <div class="bot-detail-item">
                      <div class="bot-detail-label">应用 (ID)</div>
                      <div class="bot-detail-value" id="discord_bot_id">--</div>
                    </div>
                    <div class="bot-detail-item">
                      <div class="bot-detail-label">Bot 昵称</div>
                      <div class="bot-detail-value" id="discord_bot_name">--</div>
                    </div>
                    <div class="bot-detail-item">
                      <div class="bot-detail-label">用户名/标识</div>
                      <div class="bot-detail-value" id="discord_bot_tag">--</div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Discord Servers list output -->
            <div class="m3-card" id="discord_guilds_card" style="display: none;">
              <h3 style="font-size: 15px; font-weight: 600; margin-bottom: 12px; display: flex; align-items: center; gap: 8px;">
                <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-primary));">dns</span>
                Bot 加入的 Discord 服务器列表 (Guilds)
              </h3>
              <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 16px;">
                Bot 具有管理员或普通读取权限的服务器。点击复制 ID 用于设置 Discord 默认存储目标。
              </p>
              
              <div class="chat-grid" id="discord_guilds_container">
                <!-- guilds dynamically loaded -->
              </div>
            </div>
          </div>
        </div>

        <!-- Tab 4: API Sandbox Playground -->
        <div class="panel-view" id="panel_sandbox">
          <div class="api-sandbox-layout">
            <!-- Left panel: Form Controls -->
            <div class="m3-card api-params-col">
              <h2 class="section-title">
                <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-primary));">network_check</span>
                API 测试参数选择
              </h2>
              <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 20px;">选择一个接口端点，填充参数进行实时响应联调。</p>
              
              <div class="form-field">
                <label>选择调试 API</label>
                <div class="input-wrapper">
                  <select id="sandboxApiSelector" onchange="onSandboxApiChange()">
                    <option value="health">GET /api/v1/health (健康检查)</option>
                    <option value="list">GET /api/v1/media (文件列表)</option>
                    <option value="meta">GET /api/v1/media/{mediaId}/meta (媒体元数据)</option>
                    <option value="upload">POST /api/v1/media/upload (上传媒体文件)</option>
                    <option value="delete">DELETE /api/v1/media/{mediaId} (删除媒体文件)</option>
                    <option value="telegram_chats">POST /api/v1/provider/telegram/chat-ids (获取TG群组)</option>
                  </select>
                </div>
              </div>

              <!-- Dynamic parameters builder container -->
              <div id="sandboxParamsContainer" style="margin-top: 20px;">
                <!-- HTML parameters generated by JS -->
              </div>

              <div style="margin-top: 28px;">
                <button class="m3-btn m3-btn-primary" style="width: 100%;" onclick="runSandboxApi()">
                  <span class="material-symbols-rounded">play_arrow</span>
                  <span>发起 API 请求</span>
                </button>
              </div>
            </div>

            <!-- Right panel: Code Output console -->
            <div class="api-response-col">
              <div class="console-header">
                <h2 class="section-title" style="margin-bottom: 0;">
                  <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-secondary));">terminal</span>
                  调试响应控制台
                </h2>
                
                <div style="display: flex; gap: 10px; align-items: center;">
                  <div class="status-pill" id="sandboxStatusPill" style="display: none;"></div>
                  <button class="m3-btn m3-btn-secondary m3-btn-sm" onclick="copyConsoleOutput()">
                    <span class="material-symbols-rounded" style="font-size: 16px;">content_copy</span>
                    <span>复制响应</span>
                  </button>
                </div>
              </div>

              <div class="console-output" id="sandboxOutput">等待发送请求，联调数据将在此实时高亮渲染...</div>
            </div>
          </div>
        </div>

      </main>
    </div>
  </div>

  <!-- Upload FAB Action Panel Modal -->
  <button class="fab" id="uploadFab" onclick="openUploadDialog()">
    <span class="material-symbols-rounded" style="font-size: 32px;">add</span>
  </button>

  <!-- Dialog Modal: File Upload -->
  <div class="m3-dialog-overlay" id="uploadDialogOverlay">
    <div class="m3-dialog">
      <div class="m3-dialog-header">
        <h3>📤 上传新媒体文件</h3>
        <button class="m3-dialog-close" onclick="closeUploadDialog()">&times;</button>
      </div>
      <div class="m3-dialog-body">
        <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 20px;">
          文件最大支持限制：图片类最大 10MB (jpg/png/webp)，视频类最大 120MB (mp4/webm/mov)。
        </p>

        <div class="form-field">
          <label>所属项目 (Project)</label>
          <div class="input-wrapper">
            <input id="uploadProject" type="text" value="interactive-video" placeholder="如 myproject" />
          </div>
        </div>

        <div class="form-field">
          <label>使用场景 (Usage)</label>
          <div class="input-wrapper">
            <select id="uploadUsage">
              <option value="cover">cover (封面大图)</option>
              <option value="scene">scene (场景/正片)</option>
              <option value="avatar">avatar (头像/缩略图)</option>
            </select>
          </div>
        </div>

        <div class="form-field">
          <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; user-select: none;">
            <input type="checkbox" id="uploadIsMember" style="accent-color: hsl(var(--md-sys-color-primary));" />
            <span>是否会员专享内容 (is_member)</span>
          </label>
        </div>

        <div class="form-field" style="margin-top: 16px;">
          <label>选择媒体文件</label>
          <div class="input-wrapper">
            <input id="uploadFileInput" type="file" accept="image/*,video/*" />
          </div>
        </div>
      </div>
      <div class="m3-dialog-footer">
        <button class="m3-btn m3-btn-secondary" onclick="closeUploadDialog()">取消</button>
        <button class="m3-btn m3-btn-primary" onclick="submitUploadFile()">确认上传</button>
      </div>
    </div>
  </div>

  <!-- Detail Drawer Panel (Sheet) -->
  <div class="m3-sheet" id="detailSheet">
    <div class="m3-sheet-header">
      <h3 style="font-size: 18px; font-weight: 600; color: #fff;">📁 媒体资源元数据</h3>
      <button class="m3-dialog-close" onclick="closeDetailSheet()">&times;</button>
    </div>
    <div class="m3-sheet-body" id="detailSheetBody">
      <!-- Injected by JS -->
    </div>
    <div style="margin-top: 24px; display: flex; gap: 12px;">
      <button class="m3-btn m3-btn-primary m3-btn-sm" style="flex: 1;" id="detail_btn_open" onclick="">在新标签页打开</button>
      <button class="m3-btn m3-btn-secondary m3-btn-sm" style="flex: 1;" id="detail_btn_copy" onclick="">复制链接</button>
    </div>
  </div>

  <script>
    // State management
    var state = {
      currentPage: 'dashboard',
      layoutMode: 'grid',
      assets: [],
      projects: new Set(),
      usages: new Set(),
      verifierTab: 'tg',
      activeDetailAsset: null
    };

    // Initialize Page
    window.addEventListener('DOMContentLoaded', function() {
      // Set Default URL Configs
      var localOrigin = window.location.origin;
      document.getElementById('baseUrl').value = localStorage.getItem('media_gateway_url') || localOrigin;
      document.getElementById('authToken').value = localStorage.getItem('media_gateway_token') || '';

      // Initialize API params dropdown trigger
      onSandboxApiChange();

      // Trigger initial loaders
      checkServerHealth();
      loadMediaAssets();

      // Close popups on outer clicks
      window.addEventListener('click', function(e) {
        var configDropdown = document.getElementById('configDropdown');
        var configBtn = document.getElementById('configBtn');
        if (configDropdown.style.display === 'flex' && !configDropdown.contains(e.target) && !configBtn.contains(e.target)) {
          configDropdown.style.display = 'none';
          configBtn.classList.remove('active');
        }
      });
    });

    // Helper: Show Snackbar Notification
    function showToast(message, type) {
      if (!type) type = 'success';
      var container = document.getElementById('toastContainer');
      var toast = document.createElement('div');
      toast.className = 'toast toast-' + type;
      
      var icon = 'check_circle';
      if (type === 'error') icon = 'error';
      
      toast.innerHTML = '<span class="material-symbols-rounded" style="font-size: 18px;">' + icon + '</span><span>' + message + '</span>';
      
      container.appendChild(toast);
      setTimeout(function() {
        toast.style.opacity = '0';
        toast.style.transform = 'translateY(-20px)';
        setTimeout(function() { toast.remove(); }, 300);
      }, 3500);
    }

    // Toggle Drawer on Mobile
    function toggleDrawer() {
      var drawer = document.getElementById('navDrawer');
      drawer.classList.toggle('active');
    }

    // Switch Application Views
    function switchTab(tabId) {
      state.currentPage = tabId;
      
      // Update sidebar states
      document.querySelectorAll('.nav-item').forEach(function(item) { item.classList.remove('active'); });
      var activeItem = Array.from(document.querySelectorAll('.nav-item')).find(function(item) {
        return item.getAttribute('onclick').includes(tabId);
      });
      if (activeItem) activeItem.classList.add('active');

      // Update Top Title
      var titles = {
        dashboard: '仪表盘总览',
        explorer: '媒体库管理器',
        verifier: '机器人连通验证',
        sandbox: 'API 联调沙盒'
      };
      document.getElementById('pageTitle').innerText = titles[tabId];

      // Update Visibility
      document.querySelectorAll('.panel-view').forEach(function(panel) { panel.classList.remove('active'); });
      document.getElementById('panel_' + tabId).classList.add('active');

      // Hide mobile drawer if open
      document.getElementById('navDrawer').classList.remove('active');

      // Refresh data accordingly
      if (tabId === 'dashboard') {
        checkServerHealth();
        updateDashboardStats();
      } else if (tabId === 'explorer') {
        loadMediaAssets();
      }
    }

    // Toggle Password Visibility Input
    function togglePasswordVisibility(inputId) {
      var input = document.getElementById(inputId);
      var eye = document.getElementById(inputId + '_eye');
      if (input.type === 'password') {
        input.type = 'text';
        eye.innerText = 'visibility_off';
      } else {
        input.type = 'password';
        eye.innerText = 'visibility';
      }
    }

    // Save and load Global API configurations
    function toggleConfigDropdown() {
      var dropdown = document.getElementById('configDropdown');
      var btn = document.getElementById('configBtn');
      if (dropdown.style.display === 'flex') {
        dropdown.style.display = 'none';
        btn.classList.remove('active');
      } else {
        dropdown.style.display = 'flex';
        btn.classList.add('active');
      }
    }

    function saveGlobalConfig() {
      var url = document.getElementById('baseUrl').value.trim();
      var token = document.getElementById('authToken').value.trim();
      
      localStorage.setItem('media_gateway_url', url);
      localStorage.setItem('media_gateway_token', token);
      
      showToast('全局配置已成功保存！');
      toggleConfigDropdown();
      
      // Trigger reloading
      checkServerHealth();
      loadMediaAssets();
    }

    // Helper: Headers Builder
    function getAuthHeaders() {
      var token = localStorage.getItem('media_gateway_token') || document.getElementById('authToken').value.trim();
      return token ? { 'Authorization': 'Bearer ' + token } : {};
    }

    function getBaseUrl() {
      return (localStorage.getItem('media_gateway_url') || document.getElementById('baseUrl').value.trim()).replace(/\/$/, '');
    }

    // Formatter helpers
    function formatBytes(bytes) {
      if (!bytes || bytes === 0) return '0 B';
      var k = 1024;
      var sizes = ['B', 'KB', 'MB', 'GB'];
      var i = Math.floor(Math.log(bytes) / Math.log(k));
      return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    // Snipped formatting function
    function formatDate(dateStr) {
      if (!dateStr) return '--';
      try {
        var d = new Date(dateStr);
        return d.toLocaleString('zh-CN', { hour12: false });
      } catch (e) {
        return dateStr;
      }
    }

    // Server Health API Check
    async function checkServerHealth() {
      var valEl = document.getElementById('stat_health_status');
      var envAuthEl = document.getElementById('env_auth_state');
      
      valEl.innerText = '连接中...';
      valEl.style.color = '#fff';
      
      try {
        var res = await fetch(getBaseUrl() + '/api/v1/health');
        var data = await res.json();
        
        if (res.status === 200 && data.status === 'ok') {
          valEl.innerText = '在线运行';
          valEl.style.color = 'hsl(var(--md-sys-color-success))';
        } else {
          valEl.innerText = '异常 (HTTP ' + res.status + ')';
          valEl.style.color = 'var(--md-sys-color-error)';
        }
      } catch (e) {
        valEl.innerText = '断开/未运行';
        valEl.style.color = 'var(--md-sys-color-error)';
      }

      // Check Bearer Token check
      var testToken = localStorage.getItem('media_gateway_token') || document.getElementById('authToken').value.trim();
      envAuthEl.innerText = testToken ? '已配置 (Bearer)' : '未配置';
      envAuthEl.style.color = testToken ? 'hsl(var(--md-sys-color-primary))' : 'var(--md-sys-color-error)';
    }

    // Fetch and cache Media Library items
    async function loadMediaAssets() {
      try {
        var res = await fetch(getBaseUrl() + '/api/v1/media?limit=100', {
          headers: getAuthHeaders()
        });
        
        if (res.status === 401) {
          showToast('接口访问未授权，请配置正确的 Bearer Token', 'error');
          renderAssets([]);
          return;
        }

        if (res.status !== 200) {
          showToast('拉取资源失败: HTTP ' + res.status, 'error');
          renderAssets([]);
          return;
        }

        var data = await res.json();
        state.assets = data || [];
        
        // Extract project and usage metadata
        state.projects.clear();
        state.usages.clear();
        state.assets.forEach(function(asset) {
          if (asset.project) state.projects.add(asset.project);
          if (asset.usage) state.usages.add(asset.usage);
        });

        // Update Filters UI
        populateFilters();
        // Render List & Grid layout
        applyFilters();
        // Update Dashboard statistics
        updateDashboardStats();
      } catch (e) {
        showToast('请求异常: ' + e.message, 'error');
        renderAssets([]);
      }
    }

    function populateFilters() {
      var projFilter = document.getElementById('filterProject');
      var usageFilter = document.getElementById('filterUsage');

      var selectedProj = projFilter.value;
      var selectedUsage = usageFilter.value;

      projFilter.innerHTML = '<option value="">全部项目 (Projects)</option>';
      state.projects.forEach(function(p) {
        projFilter.innerHTML += '<option value="' + p + '">' + p + '</option>';
      });

      usageFilter.innerHTML = '<option value="">全部用途 (Usages)</option>';
      state.usages.forEach(function(u) {
        usageFilter.innerHTML += '<option value="' + u + '">' + u + '</option>';
      });

      // Restore values
      projFilter.value = selectedProj;
      usageFilter.value = selectedUsage;
    }

    // Filter Logic
    function applyFilters() {
      var keyword = document.getElementById('searchKeyword').value.trim().toLowerCase();
      var proj = document.getElementById('filterProject').value;
      var usage = document.getElementById('filterUsage').value;
      var showDel = document.getElementById('showDeleted').checked;

      var filtered = state.assets.filter(function(asset) {
        // Keyword check
        var matchesKeyword = !keyword || 
          asset.mediaId.toLowerCase().indexOf(keyword) !== -1 || 
          (asset.mimeType && asset.mimeType.toLowerCase().indexOf(keyword) !== -1) ||
          (asset.project && asset.project.toLowerCase().indexOf(keyword) !== -1);

        // Project check
        var matchesProj = !proj || asset.project === proj;
        // Usage check
        var matchesUsage = !usage || asset.usage === usage;
        // Status check
        var matchesStatus = showDel || asset.status === 'active';

        return matchesKeyword && matchesProj && matchesUsage && matchesStatus;
      });

      renderAssets(filtered);
    }

    // Toggle Grid vs List Layout in Media Explorer
    function setExplorerLayout(layout) {
      state.layoutMode = layout;
      document.getElementById('btn_view_grid').classList.toggle('active', layout === 'grid');
      document.getElementById('btn_view_list').classList.toggle('active', layout === 'list');

      document.getElementById('explorerContainerGrid').style.display = layout === 'grid' ? 'grid' : 'none';
      document.getElementById('explorerContainerList').style.display = layout === 'list' ? 'block' : 'none';
      
      applyFilters();
    }

    // Render Assets
    function renderAssets(assetsList) {
      var gridContainer = document.getElementById('explorerContainerGrid');
      var listBody = document.getElementById('explorerListBody');
      var emptyState = document.getElementById('explorerEmptyState');

      gridContainer.innerHTML = '';
      listBody.innerHTML = '';

      if (assetsList.length === 0) {
        gridContainer.style.display = 'none';
        document.getElementById('explorerContainerList').style.display = 'none';
        emptyState.style.display = 'block';
        return;
      }

      emptyState.style.display = 'none';
      if (state.layoutMode === 'grid') {
        gridContainer.style.display = 'grid';
      } else {
        document.getElementById('explorerContainerList').style.display = 'block';
      }

      assetsList.forEach(function(asset) {
        var isImage = asset.mimeType.startsWith('image/');
        var isVideo = asset.mimeType.startsWith('video/');
        var previewHtml = '<span class="material-symbols-rounded file-icon">description</span>';

        if (isImage && asset.status === 'active') {
          previewHtml = '<img src="' + asset.publicUrl + '" alt="preview" loading="lazy" />';
        } else if (isVideo && asset.status === 'active') {
          previewHtml = '<span class="material-symbols-rounded file-icon" style="color: hsl(var(--md-sys-color-secondary));">video_library</span>';
        }

        var sizeStr = formatBytes(asset.sizeBytes);
        var dateStr = formatDate(asset.createdAt);
        var statusBadge = asset.status === 'active' ? 
          '<span class="badge badge-success">活动</span>' : 
          '<span class="badge badge-error">已删除</span>';

        var chunkBadge = asset.isChunked ? '<span class="badge badge-primary" style="position: absolute; top: 8px; left: 8px;">分片上传</span>' : '';
        var deleteButton = asset.status === 'active' ? 
          '<button class="m3-btn m3-btn-danger m3-btn-sm" style="flex: 1; padding: 6px 0;" onclick="deleteAsset(\'' + asset.mediaId + '\')"><span class="material-symbols-rounded" style="font-size: 14px;">delete</span></button>' : '';

        // 1. Grid Rendering
        var card = document.createElement('div');
        card.className = 'media-card';
        card.innerHTML = 
          '<div class="media-thumb" onclick="openDetailSheet(\'' + asset.mediaId + '\')" style="cursor: pointer;">' +
            previewHtml + chunkBadge +
          '</div>' +
          '<div class="media-card-info">' +
            '<div class="media-id" title="' + asset.mediaId + '">' + asset.mediaId + '</div>' +
            '<div class="media-meta-row">' +
              '<span>' + sizeStr + '</span>' +
              '<span>' + dateStr.split(' ')[0] + '</span>' +
            '</div>' +
            '<div class="media-card-tags">' +
              '<span class="badge">' + asset.project + '</span>' +
              '<span class="badge">' + asset.usage + '</span>' +
              statusBadge +
            '</div>' +
          '</div>' +
          '<div class="media-actions">' +
            '<button class="m3-btn m3-btn-secondary m3-btn-sm" style="flex: 1; padding: 6px 0;" onclick="copyText(\'' + asset.publicUrl + '\')">' +
              '<span class="material-symbols-rounded" style="font-size: 14px;">link</span>' +
            '</button>' +
            '<button class="m3-btn m3-btn-secondary m3-btn-sm" style="flex: 1; padding: 6px 0;" onclick="openDetailSheet(\'' + asset.mediaId + '\')">' +
              '<span class="material-symbols-rounded" style="font-size: 14px;">visibility</span>' +
            '</button>' +
            deleteButton +
          '</div>';
        gridContainer.appendChild(card);

        // 2. List Rendering
        var listDeleteBtn = asset.status === 'active' ? '<button class="m3-btn m3-btn-danger m3-btn-sm" onclick="deleteAsset(\'' + asset.mediaId + '\')">删除</button>' : '';
        var row = document.createElement('tr');
        var listPreview = '';
        if (isImage && asset.status === 'active') {
          listPreview = '<img src="' + asset.publicUrl + '" style="width: 100%; height: 100%; object-fit: cover;" />';
        } else {
          listPreview = '<span class="material-symbols-rounded" style="font-size: 20px; color: hsl(var(--md-sys-color-primary));">' + (isVideo ? 'video_library' : 'description') + '</span>';
        }

        row.innerHTML = 
          '<td>' +
            '<div style="width: 40px; height: 40px; border-radius: 8px; background: rgba(0,0,0,0.2); display: flex; align-items: center; justify-content: center; overflow: hidden;">' +
              listPreview +
            '</div>' +
          '</td>' +
          '<td>' +
            '<span style="font-weight: 600; color: #fff; cursor: pointer;" onclick="openDetailSheet(\'' + asset.mediaId + '\')">' + asset.mediaId + '</span>' +
          '</td>' +
          '<td>' + asset.mimeType + '</td>' +
          '<td>' + sizeStr + '</td>' +
          '<td>' +
            '<span class="badge" style="margin-right: 4px;">' + asset.project + '</span>' +
            '<span class="badge">' + asset.usage + '</span>' +
          '</td>' +
          '<td>' + statusBadge + '</td>' +
          '<td style="text-align: right;">' +
            '<div style="display: inline-flex; gap: 6px;">' +
              '<button class="m3-btn m3-btn-secondary m3-btn-sm" onclick="copyText(\'' + asset.publicUrl + '\')">复制链接</button>' +
              '<button class="m3-btn m3-btn-secondary m3-btn-sm" onclick="openDetailSheet(\'' + asset.mediaId + '\')">详情</button>' +
              listDeleteBtn +
            '</div>' +
          '</td>';
        listBody.appendChild(row);
      });
    }

    // Dashboard Statistics Calculation
    function updateDashboardStats() {
      document.getElementById('stat_total_count').innerText = state.assets.length;
      
      var totalBytes = 0;
      var imgCount = 0;
      var imgBytes = 0;
      var vidCount = 0;
      var vidBytes = 0;
      var otherCount = 0;
      var otherBytes = 0;

      state.assets.forEach(function(asset) {
        totalBytes += asset.sizeBytes;
        
        if (asset.mimeType.startsWith('image/')) {
          imgCount++;
          imgBytes += asset.sizeBytes;
        } else if (asset.mimeType.startsWith('video/')) {
          vidCount++;
          vidBytes += asset.sizeBytes;
        } else {
          otherCount++;
          otherBytes += asset.sizeBytes;
        }
      });

      document.getElementById('stat_total_size').innerText = formatBytes(totalBytes);

      // Render chart ratios
      var renderBar = function(barId, txtId, count, bytes, totalCount) {
        var pct = totalCount > 0 ? (count / totalCount * 100).toFixed(0) : 0;
        document.getElementById(barId).style.width = pct + '%';
        document.getElementById(txtId).innerText = count + ' 个 (' + pct + '%) - ' + formatBytes(bytes);
      };

      renderBar('chart_img_bar', 'chart_img_txt', imgCount, imgBytes, state.assets.length);
      renderBar('chart_vid_bar', 'chart_vid_txt', vidCount, vidBytes, state.assets.length);
      renderBar('chart_other_bar', 'chart_other_txt', otherCount, otherBytes, state.assets.length);
    }

    // Open/Close dialog uploads
    function openUploadDialog() {
      document.getElementById('uploadDialogOverlay').classList.add('active');
    }

    function closeUploadDialog() {
      document.getElementById('uploadDialogOverlay').classList.remove('active');
      document.getElementById('uploadFileInput').value = '';
    }

    // Perform file uploads
    async function submitUploadFile() {
      var fileInput = document.getElementById('uploadFileInput');
      if (!fileInput.files || !fileInput.files[0]) {
        showToast('请选择需要上传的文件！', 'error');
        return;
      }

      var form = new FormData();
      form.append('file', fileInput.files[0]);
      form.append('project', document.getElementById('uploadProject').value.trim());
      form.append('usage', document.getElementById('uploadUsage').value);
      form.append('member', document.getElementById('uploadIsMember').checked ? 'true' : 'false');

      showToast('正在上传，请耐心等待...', 'success');
      closeUploadDialog();

      try {
        var res = await fetch(getBaseUrl() + '/api/v1/media/upload', {
          method: 'POST',
          headers: getAuthHeaders(),
          body: form
        });
        var data = await res.json();

        if (res.status === 201) {
          showToast('媒体文件上传成功！');
          loadMediaAssets();
        } else {
          showToast(data.error || '上传失败', 'error');
        }
      } catch (e) {
        showToast('上传异常: ' + e.message, 'error');
      }
    }

    // Show asset details panel (Drawer sheet)
    function openDetailSheet(mediaId) {
      var asset = state.assets.find(function(a) { return a.mediaId === mediaId; });
      if (!asset) return;

      state.activeDetailAsset = asset;
      var body = document.getElementById('detailSheetBody');

      var preview = '';
      if (asset.mimeType.startsWith('image/') && asset.status === 'active') {
        preview = '<div style="width: 100%; height: 180px; border-radius: 16px; overflow: hidden; background: #000; border: 1px solid rgba(255,255,255,0.08);"><img src="' + asset.publicUrl + '" style="width:100%; height:100%; object-fit:contain;" /></div>';
      } else if (asset.mimeType.startsWith('video/') && asset.status === 'active') {
        preview = '<video src="' + asset.publicUrl + '" controls style="width:100%; height:180px; border-radius: 16px; background: #000; border: 1px solid rgba(255,255,255,0.08); object-fit:contain;"></video>';
      }

      body.innerHTML = 
        preview +
        '<div style="display: flex; flex-direction: column; gap: 14px; font-size: 13px; margin-top: 12px;">' +
          '<div>' +
            '<div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 4px;">媒体 ID (Media ID)</div>' +
            '<div style="font-family: monospace; color:#fff; word-break:break-all; background: rgba(0,0,0,0.2); padding: 8px; border-radius: 8px;">' + asset.mediaId + '</div>' +
          '</div>' +
          '<div>' +
            '<div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 4px;">公共访问地址 (Public Link)</div>' +
            '<div style="font-family: monospace; color:#fff; word-break:break-all; background: rgba(0,0,0,0.2); padding: 8px; border-radius: 8px; font-size:11px;">' + asset.publicUrl + '</div>' +
          '</div>' +
          '<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">' +
            '<div>' +
              '<div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">大小</div>' +
              '<div style="color:#fff;">' + formatBytes(asset.sizeBytes) + '</div>' +
            '</div>' +
            '<div>' +
              '<div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">类型</div>' +
              '<div style="color:#fff; font-family: monospace;">' + asset.mimeType + '</div>' +
            '</div>' +
          '</div>' +
          '<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">' +
            '<div>' +
              '<div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">项目</div>' +
              '<div style="color:#fff;">' + asset.project + '</div>' +
            '</div>' +
            '<div>' +
              '<div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">用途</div>' +
              '<div style="color:#fff;">' + asset.usage + '</div>' +
            '</div>' +
          '</div>' +
          '<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">' +
            '<div>' +
              '<div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">分片上传</div>' +
              '<div style="color:#fff;">' + (asset.isChunked ? '是 (' + asset.chunkCount + '片)' : '否') + '</div>' +
            '</div>' +
            '<div>' +
              '<div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">所属 Provider</div>' +
              '<div style="color:#fff; text-transform: uppercase;">' + asset.provider + '</div>' +
            '</div>' +
          '</div>' +
          '<div>' +
            '<div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">SHA-256</div>' +
            '<div style="color:#fff; font-family: monospace; font-size:11px; word-break:break-all;">' + (asset.sha256 || '暂无') + '</div>' +
          '</div>' +
          '<div>' +
            '<div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">入库时间</div>' +
            '<div style="color:#fff;">' + formatDate(asset.createdAt) + '</div>' +
          '</div>' +
        '</div>';

      // Set action events
      document.getElementById('detail_btn_open').setAttribute('onclick', "window.open('" + asset.publicUrl + "', '_blank')");
      document.getElementById('detail_btn_copy').setAttribute('onclick', "copyText('" + asset.publicUrl + "')");

      document.getElementById('detailSheet').classList.add('active');
    }

    function closeDetailSheet() {
      document.getElementById('detailSheet').classList.remove('active');
      state.activeDetailAsset = null;
    }

    // Delete Media Asset
    async function deleteAsset(mediaId) {
      if (!confirm('确定要删除此媒体资源吗？\nID: ' + mediaId)) return;

      try {
        var res = await fetch(getBaseUrl() + '/api/v1/media/' + encodeURIComponent(mediaId), {
          method: 'DELETE',
          headers: getAuthHeaders()
        });
        var data = await res.json();

        if (res.status === 200) {
          showToast('媒体资源已成功标记删除！');
          loadMediaAssets();
          closeDetailSheet();
        } else {
          showToast(data.error || '删除失败', 'error');
        }
      } catch (e) {
        showToast('删除异常: ' + e.message, 'error');
      }
    }

    // Helper: copy to clipboard
    function copyText(txt) {
      navigator.clipboard.writeText(txt).then(function() {
        showToast('已成功复制到剪贴板！');
      }).catch(function(err) {
        showToast('复制失败，请手动选择复制', 'error');
      });
    }

    // --- Provider Verifier Panel Logic ---
    function switchVerifierTab(tab) {
      state.verifierTab = tab;
      document.getElementById('btn_tg_tab').classList.toggle('active', tab === 'tg');
      document.getElementById('btn_discord_tab').classList.toggle('active', tab === 'discord');

      document.getElementById('verifier_tg_content').style.display = tab === 'tg' ? 'flex' : 'none';
      document.getElementById('verifier_discord_content').style.display = tab === 'discord' ? 'flex' : 'none';
    }

    // TG Verify
    async function verifyTelegramBot() {
      var token = document.getElementById('tgBotTokenInput').value.trim();
      var statusIcon = document.getElementById('tg_status_icon');
      var statusTitle = document.getElementById('tg_status_title');
      var statusDesc = document.getElementById('tg_status_desc');
      var detailsEl = document.getElementById('tg_bot_details');

      statusTitle.innerText = '正在验证...';
      statusDesc.innerText = '请稍候...';
      statusIcon.style.color = 'rgba(255,255,255,0.2)';
      statusIcon.innerText = 'sync';
      statusIcon.style.animation = 'spin 2s linear infinite';
      detailsEl.style.display = 'none';

      try {
        var res = await fetch(getBaseUrl() + '/api/v1/provider/telegram/verify', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, getAuthHeaders()),
          body: JSON.stringify({ token: token })
        });
        var data = await res.json();
        
        statusIcon.style.animation = ''; // stop spin
        
        if (res.status === 200 && data.ok) {
          statusIcon.innerText = 'check_circle';
          statusIcon.style.color = 'hsl(var(--md-sys-color-success))';
          statusTitle.innerText = '验证成功！';
          statusTitle.style.color = 'hsl(var(--md-sys-color-success))';
          statusDesc.innerText = 'Telegram 机器人连接正常';
          
          document.getElementById('tg_bot_id').innerText = data.bot_info.id || '--';
          document.getElementById('tg_bot_name').innerText = data.bot_info.first_name || '--';
          document.getElementById('tg_bot_username').innerText = '@' + (data.bot_info.username || '--');
          
          detailsEl.style.display = 'grid';
          showToast('Telegram 机器人验证成功！');
        } else {
          statusIcon.innerText = 'cancel';
          statusIcon.style.color = 'var(--md-sys-color-error)';
          statusTitle.innerText = '验证失败';
          statusTitle.style.color = 'var(--md-sys-color-error)';
          statusDesc.innerText = data.error || '无效的 Token 或网络连接失败';
          showToast(data.error || '验证失败', 'error');
        }
      } catch (e) {
        statusIcon.style.animation = '';
        statusIcon.innerText = 'cancel';
        statusIcon.style.color = 'var(--md-sys-color-error)';
        statusTitle.innerText = '异常错误';
        statusDesc.innerText = e.message;
        showToast(e.message, 'error');
      }
    }

    // TG Get Chats
    async function fetchTelegramChatIDsPost() {
      var token = document.getElementById('tgBotTokenInput').value.trim();
      var container = document.getElementById('tg_chats_container');
      var cardEl = document.getElementById('tg_chats_card');

      cardEl.style.display = 'block';
      container.innerHTML = '<p style="grid-column: 1/-1; text-align:center; color:hsl(var(--md-sys-color-on-surface-variant));">正在拉取，请确保有群组最近发过消息...</p>';

      try {
        var res = await fetch(getBaseUrl() + '/api/v1/provider/telegram/chat-ids', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, getAuthHeaders()),
          body: JSON.stringify({ token: token })
        });
        var data = await res.json();

        if (res.status === 200) {
          container.innerHTML = '';
          if (!data || data.length === 0) {
            container.innerHTML = '<p style="grid-column: 1/-1; text-align:center; color:hsl(var(--md-sys-color-on-surface-variant)); padding: 20px 0;">未检测到最新交互。请先向 Bot 所在的群组发送消息，然后重试。</p>';
            return;
          }

          data.forEach(function(chat) {
            var chatCard = document.createElement('div');
            chatCard.className = 'chat-card';
            chatCard.innerHTML = 
              '<div class="chat-card-info">' +
                '<div class="chat-card-title">' + chat.title + '</div>' +
                '<div class="chat-card-id">ID: <code>' + chat.id + '</code></div>' +
                '<div style="font-size:11px; color:hsl(var(--md-sys-color-primary)); margin-top:2px; text-transform: capitalize;">类型: ' + chat.type + '</div>' +
              '</div>' +
              '<button class="m3-btn m3-btn-secondary m3-btn-sm" style="padding: 6px 12px;" onclick="copyText(\'' + chat.id + '\')">复制 ID</button>';
            container.appendChild(chatCard);
          });
          showToast('获取 Telegram 群组列表成功！');
        } else {
          container.innerHTML = '<p style="grid-column: 1/-1; text-align:center; color:var(--md-sys-color-error); padding: 20px 0;">获取失败: ' + (data.error || '未知错误') + '</p>';
          showToast(data.error || '获取失败', 'error');
        }
      } catch (e) {
        container.innerHTML = '<p style="grid-column: 1/-1; text-align:center; color:var(--md-sys-color-error); padding: 20px 0;">请求异常: ' + e.message + '</p>';
        showToast(e.message, 'error');
      }
    }

    // Discord Verify
    async function verifyDiscordBot() {
      var token = document.getElementById('discordBotTokenInput').value.trim();
      var statusIcon = document.getElementById('discord_status_icon');
      var statusTitle = document.getElementById('discord_status_title');
      var statusDesc = document.getElementById('discord_status_desc');
      var detailsEl = document.getElementById('discord_bot_details');

      if (!token) {
        showToast('请输入 Discord Bot Token', 'error');
        return;
      }

      statusTitle.innerText = '正在验证...';
      statusDesc.innerText = '请稍候...';
      statusIcon.style.color = 'rgba(255,255,255,0.2)';
      statusIcon.innerText = 'sync';
      statusIcon.style.animation = 'spin 2s linear infinite';
      detailsEl.style.display = 'none';

      try {
        var res = await fetch(getBaseUrl() + '/api/v1/provider/discord/verify', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, getAuthHeaders()),
          body: JSON.stringify({ token: token })
        });
        var data = await res.json();
        
        statusIcon.style.animation = ''; // stop spin
        
        if (res.status === 200 && data.ok) {
          statusIcon.innerText = 'check_circle';
          statusIcon.style.color = 'hsl(var(--md-sys-color-success))';
          statusTitle.innerText = '验证成功！';
          statusTitle.style.color = 'hsl(var(--md-sys-color-success))';
          statusDesc.innerText = 'Discord 机器人授权成功';
          
          document.getElementById('discord_bot_id').innerText = data.bot_info.id || '--';
          document.getElementById('discord_bot_name').innerText = data.bot_info.username || '--';
          document.getElementById('discord_bot_tag').innerText = data.bot_info.username + '#' + (data.bot_info.discriminator || '0000');
          
          detailsEl.style.display = 'grid';
          showToast('Discord 机器人鉴权验证成功！');
        } else {
          statusIcon.innerText = 'cancel';
          statusIcon.style.color = 'var(--md-sys-color-error)';
          statusTitle.innerText = '验证失败';
          statusTitle.style.color = 'var(--md-sys-color-error)';
          statusDesc.innerText = data.error || '无效的 Token 或网络连接失败';
          showToast(data.error || '验证失败', 'error');
        }
      } catch (e) {
        statusIcon.style.animation = '';
        statusIcon.innerText = 'cancel';
        statusIcon.style.color = 'var(--md-sys-color-error)';
        statusTitle.innerText = '异常错误';
        statusDesc.innerText = e.message;
        showToast(e.message, 'error');
      }
    }

    // Discord Get Guilds
    async function fetchDiscordGuilds() {
      var token = document.getElementById('discordBotTokenInput').value.trim();
      var container = document.getElementById('discord_guilds_container');
      var cardEl = document.getElementById('discord_guilds_card');

      if (!token) {
        showToast('请输入 Discord Bot Token', 'error');
        return;
      }

      cardEl.style.display = 'block';
      container.innerHTML = '<p style="grid-column: 1/-1; text-align:center; color:hsl(var(--md-sys-color-on-surface-variant));">正在拉取加入的服务器...</p>';

      try {
        var res = await fetch(getBaseUrl() + '/api/v1/provider/discord/guilds', {
          method: 'POST',
          headers: Object.assign({ 'Content-Type': 'application/json' }, getAuthHeaders()),
          body: JSON.stringify({ token: token })
        });
        var data = await res.json();

        if (res.status === 200) {
          container.innerHTML = '';
          if (!data || data.length === 0) {
            container.innerHTML = '<p style="grid-column: 1/-1; text-align:center; color:hsl(var(--md-sys-color-on-surface-variant)); padding: 20px 0;">该机器人目前未加入任何服务器，请先邀请机器人到您的 Discord 服务器中。</p>';
            return;
          }

          data.forEach(function(guild) {
            var guildCard = document.createElement('div');
            guildCard.className = 'chat-card';
            guildCard.innerHTML = 
              '<div class="chat-card-info">' +
                '<div class="chat-card-title">' + guild.name + '</div>' +
                '<div class="chat-card-id">Guild ID: <code>' + guild.id + '</code></div>' +
                '<div style="font-size:11px; color:hsl(var(--md-sys-color-secondary)); margin-top:2px;">权限权重: ' + (guild.permissions || '默认') + '</div>' +
              '</div>' +
              '<button class="m3-btn m3-btn-secondary m3-btn-sm" style="padding: 6px 12px;" onclick="copyText(\'' + guild.id + '\')">复制 ID</button>';
            container.appendChild(guildCard);
          });
          showToast('拉取 Discord 服务器成功！');
        } else {
          container.innerHTML = '<p style="grid-column: 1/-1; text-align:center; color:var(--md-sys-color-error); padding: 20px 0;">拉取失败: ' + (data.error || '未知错误') + '</p>';
          showToast(data.error || '拉取失败', 'error');
        }
      } catch (e) {
        container.innerHTML = '<p style="grid-column: 1/-1; text-align:center; color:var(--md-sys-color-error); padding: 20px 0;">请求异常: ' + e.message + '</p>';
        showToast(e.message, 'error');
      }
    }

    // --- API Sandbox Testing Playground ---
    function onSandboxApiChange() {
      var val = document.getElementById('sandboxApiSelector').value;
      var paramsContainer = document.getElementById('sandboxParamsContainer');
      paramsContainer.innerHTML = '';

      var buildInput = function(id, label, placeholder, type, val) {
        if (!type) type = 'text';
        if (!val) val = '';
        return '<div class="form-field">' +
            '<label>' + label + '</label>' +
            '<div class="input-wrapper">' +
              '<input id="sb_' + id + '" type="' + type + '" placeholder="' + placeholder + '" value="' + val + '" />' +
            '</div>' +
          '</div>';
      };

      if (val === 'list') {
        paramsContainer.innerHTML = 
          buildInput('limit', '每页条数 (limit)', '默认 20', 'number', '20') +
          buildInput('offset', '偏移起始 (offset)', '默认 0', 'number', '0');
      } else if (val === 'meta' || val === 'delete') {
        paramsContainer.innerHTML = 
          buildInput('mediaId', '媒体 ID (mediaId)', '请输入已存在的 mediaId', 'text');
      } else if (val === 'upload') {
        paramsContainer.innerHTML = 
          buildInput('project', '所属项目 (project)', '例如 default-proj', 'text', 'test-project') +
          buildInput('usage', '使用用途 (usage)', '例如 cover / avatar', 'text', 'cover') +
          '<div class="form-field">' +
            '<label>会员专享 (member)</label>' +
            '<div class="input-wrapper">' +
              '<select id="sb_isMember">' +
                '<option value="false">否 (false)</option>' +
                <option value="true">是 (true)</option> +
              '</select>' +
            '</div>' +
          '</div>' +
          '<div class="form-field">' +
            '<label>文件 (file)</label>' +
            '<div class="input-wrapper">' +
              '<input id="sb_file" type="file" />' +
            '</div>' +
          '</div>';
      } else if (val === 'telegram_chats') {
        paramsContainer.innerHTML = 
          buildInput('token', 'Telegram Bot Token', '若留空则使用默认配置', 'password');
      } else {
        paramsContainer.innerHTML = '<p style="font-size: 13px; color:hsl(var(--md-sys-color-on-surface-variant));">此接口无需配置额外参数</p>';
      }
    }

    async function runSandboxApi() {
      var apiType = document.getElementById('sandboxApiSelector').value;
      var outputConsole = document.getElementById('sandboxOutput');
      var statusPill = document.getElementById('sandboxStatusPill');

      outputConsole.innerHTML = '发送请求中，请稍后...';
      statusPill.style.display = 'none';

      var startTime = performance.now();

      var fetchUrl = '';
      var options = {
        headers: Object.assign({}, getAuthHeaders())
      };

      try {
        if (apiType === 'health') {
          fetchUrl = getBaseUrl() + '/api/v1/health';
        } else if (apiType === 'list') {
          var limit = document.getElementById('sb_limit').value || 20;
          var offset = document.getElementById('sb_offset').value || 0;
          fetchUrl = getBaseUrl() + '/api/v1/media?limit=' + limit + '&offset=' + offset;
        } else if (apiType === 'meta') {
          var mediaId = document.getElementById('sb_mediaId').value.trim();
          if (!mediaId) {
            showToast('请先输入 mediaId 参数！', 'error');
            outputConsole.innerHTML = '错误: 缺少 mediaId';
            return;
          }
          fetchUrl = getBaseUrl() + '/api/v1/media/' + encodeURIComponent(mediaId) + '/meta';
        } else if (apiType === 'delete') {
          var mediaId = document.getElementById('sb_mediaId').value.trim();
          if (!mediaId) {
            showToast('请先输入 mediaId 参数！', 'error');
            outputConsole.innerHTML = '错误: 缺少 mediaId';
            return;
          }
          fetchUrl = getBaseUrl() + '/api/v1/media/' + encodeURIComponent(mediaId);
          options.method = 'DELETE';
        } else if (apiType === 'telegram_chats') {
          var token = document.getElementById('sb_token').value.trim();
          fetchUrl = getBaseUrl() + '/api/v1/provider/telegram/chat-ids';
          options.method = 'POST';
          options.headers['Content-Type'] = 'application/json';
          options.body = JSON.stringify({ token: token });
        } else if (apiType === 'upload') {
          var project = document.getElementById('sb_project').value.trim();
          var usage = document.getElementById('sb_usage').value.trim();
          var isMember = document.getElementById('sb_isMember').value;
          var fileInput = document.getElementById('sb_file');

          if (!fileInput.files || !fileInput.files[0]) {
            showToast('请在参数中选择一个文件！', 'error');
            outputConsole.innerHTML = '错误: 请选择文件';
            return;
          }

          fetchUrl = getBaseUrl() + '/api/v1/media/upload';
          options.method = 'POST';
          var form = new FormData();
          form.append('file', fileInput.files[0]);
          form.append('project', project);
          form.append('usage', usage);
          form.append('member', isMember);
          options.body = form;
          // Delete Content-Type to let browser set boundary automatically
          delete options.headers['Content-Type'];
        }

        var response = await fetch(fetchUrl, options);
        var duration = (performance.now() - startTime).toFixed(0);

        // Update status badge
        statusPill.style.display = 'inline-flex';
        statusPill.innerText = 'HTTP ' + response.status + ' • ' + duration + 'ms';
        
        if (response.status >= 200 && response.status < 300) {
          statusPill.style.background = 'rgba(133, 247, 176, 0.15)';
          statusPill.style.color = 'hsl(var(--md-sys-color-success))';
          showToast('API 请求执行成功！');
        } else {
          statusPill.style.background = 'rgba(255, 180, 171, 0.15)';
          statusPill.style.color = '#ffb4ab';
          showToast('请求失败: HTTP ' + response.status, 'error');
        }

        var contentType = response.headers.get('Content-Type');
        var renderedBody = '';
        if (contentType && contentType.indexOf('application/json') !== -1) {
          var bodyJson = await response.json();
          renderedBody = JSON.stringify(bodyJson, null, 2);
        } else {
          renderedBody = await response.text();
        }

        outputConsole.innerText = renderedBody;
      } catch (err) {
        var duration = (performance.now() - startTime).toFixed(0);
        statusPill.style.display = 'inline-flex';
        statusPill.innerText = 'ERROR • ' + duration + 'ms';
        statusPill.style.background = 'rgba(255, 180, 171, 0.15)';
        statusPill.style.color = '#ffb4ab';
        
        outputConsole.innerText = '请求捕获异常: ' + err.message + '\n检查控制台报错或服务器网络配置。';
        showToast('请求链接错误', 'error');
      }
    }

    function copyConsoleOutput() {
      var txt = document.getElementById('sandboxOutput').innerText;
      copyText(txt);
    }
  </script>

  <style>
    @keyframes spin {
      from { transform: rotate(0deg); }
      to { transform: rotate(360deg); }
    }
  </style>
</body>
</html>`
