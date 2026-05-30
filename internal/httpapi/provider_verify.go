package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type verifyRequest struct {
	Token string `json:"token"`
}

func (s *Server) handleTelegramVerify(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		s.writeError(w, r, http.StatusBadRequest, "invalid request body", err)
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		token = s.telegramBotToken
	}
	if token == "" {
		s.writeError(w, r, http.StatusBadRequest, "telegram bot token is required", nil)
		return
	}

	u := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to call telegram getMe", err)
		return
	}
	defer resp.Body.Close()

	var tgResp struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to decode telegram response", err)
		return
	}

	if !tgResp.OK {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": tgResp.Description,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"bot_info": tgResp.Result,
	})
}

func (s *Server) handleTelegramChatIDsPost(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		s.writeError(w, r, http.StatusBadRequest, "invalid request body", err)
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		token = s.telegramBotToken
	}
	if token == "" {
		s.writeError(w, r, http.StatusBadRequest, "telegram bot token is required", nil)
		return
	}

	u := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to call telegram getUpdates", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		s.writeError(w, r, resp.StatusCode, fmt.Sprintf("telegram api returned status %d: %s", resp.StatusCode, string(b)), nil)
		return
	}

	var tgResp struct {
		OK     bool `json:"ok"`
		Result []struct {
			UpdateID int `json:"update_id"`
			Message  *struct {
				Chat struct {
					ID        int64  `json:"id"`
					Title     string `json:"title"`
					Type      string `json:"type"`
					Username  string `json:"username"`
					FirstName string `json:"first_name"`
					LastName  string `json:"last_name"`
				} `json:"chat"`
			} `json:"message"`
			ChannelPost *struct {
				Chat struct {
					ID        int64  `json:"id"`
					Title     string `json:"title"`
					Type      string `json:"type"`
					Username  string `json:"username"`
					FirstName string `json:"first_name"`
					LastName  string `json:"last_name"`
				} `json:"chat"`
			} `json:"channel_post"`
			MyChatMember *struct {
				Chat struct {
					ID        int64  `json:"id"`
					Title     string `json:"title"`
					Type      string `json:"type"`
					Username  string `json:"username"`
					FirstName string `json:"first_name"`
					LastName  string `json:"last_name"`
				} `json:"chat"`
			} `json:"my_chat_member"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to decode telegram response", err)
		return
	}

	type ChatInfo struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Type  string `json:"type"`
	}

	uniqueChats := make(map[int64]ChatInfo)
	for _, res := range tgResp.Result {
		var chat *struct {
			ID        int64  `json:"id"`
			Title     string `json:"title"`
			Type      string `json:"type"`
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		}
		if res.Message != nil {
			chat = &res.Message.Chat
		} else if res.ChannelPost != nil {
			chat = &res.ChannelPost.Chat
		} else if res.MyChatMember != nil {
			chat = &res.MyChatMember.Chat
		}

		if chat != nil {
			title := chat.Title
			if title == "" {
				if chat.Username != "" {
					title = "@" + chat.Username
				} else if chat.FirstName != "" {
					title = chat.FirstName
					if chat.LastName != "" {
						title += " " + chat.LastName
					}
				} else {
					title = fmt.Sprintf("Chat %d", chat.ID)
				}
			}
			uniqueChats[chat.ID] = ChatInfo{
				ID:    chat.ID,
				Title: title,
				Type:  chat.Type,
			}
		}
	}

	var list []ChatInfo = make([]ChatInfo, 0)
	for _, info := range uniqueChats {
		list = append(list, info)
	}

	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleDiscordVerify(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		s.writeError(w, r, http.StatusBadRequest, "invalid request body", err)
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		s.writeError(w, r, http.StatusBadRequest, "discord bot token is required", nil)
		return
	}

	reqUrl := "https://discord.com/api/v10/users/@me"
	discReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, reqUrl, nil)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to create discord request", err)
		return
	}
	discReq.Header.Set("Authorization", "Bot "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(discReq)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to call discord api", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		writeJSON(w, resp.StatusCode, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("discord api returned status %d: %s", resp.StatusCode, string(b)),
		})
		return
	}

	var discResp any
	if err := json.NewDecoder(resp.Body).Decode(&discResp); err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to decode discord response", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"bot_info": discResp,
	})
}

func (s *Server) handleDiscordGuilds(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		s.writeError(w, r, http.StatusBadRequest, "invalid request body", err)
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		s.writeError(w, r, http.StatusBadRequest, "discord bot token is required", nil)
		return
	}

	reqUrl := "https://discord.com/api/v10/users/@me/guilds"
	discReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, reqUrl, nil)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to create discord request", err)
		return
	}
	discReq.Header.Set("Authorization", "Bot "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(discReq)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to call discord api", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		writeJSON(w, resp.StatusCode, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("discord api returned status %d: %s", resp.StatusCode, string(b)),
		})
		return
	}

	var guilds []any
	if err := json.NewDecoder(resp.Body).Decode(&guilds); err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to decode discord guilds response", err)
		return
	}

	writeJSON(w, http.StatusOK, guilds)
}

type workerVerifyRequest struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func (s *Server) handleWorkerVerify(w http.ResponseWriter, r *http.Request) {
	var req workerVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		s.writeError(w, r, http.StatusBadRequest, "invalid request body", err)
		return
	}

	url := strings.TrimSpace(req.URL)
	if url == "" {
		url = s.workerBaseURL
	}
	url = strings.TrimSpace(url)
	if url != "" && !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		token = s.workerAuthToken
	}

	if url == "" {
		s.writeError(w, r, http.StatusBadRequest, "Worker URL is required", nil)
		return
	}

	// Clean trailing slash
	url = strings.TrimRight(url, "/")

	// Ping the Worker's get endpoint to test connection
	pingURL := fmt.Sprintf("%s/get?file_id=test_ping_conn", url)
	client := &http.Client{Timeout: 10 * time.Second}
	
	workerReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, pingURL, nil)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "failed to create worker request", err)
		return
	}

	if token != "" {
		workerReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(workerReq)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("无法连接到 Cloudflare Worker: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	// If token is invalid, Worker returns 401 Unauthorized.
	// Otherwise, it might return 400 (missing param/invalid ID) or 404 (not found).
	// Therefore, any status other than 401 indicates valid authorization/connectivity.
	if resp.StatusCode == http.StatusUnauthorized {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "鉴权失败: Worker Token 无效 (HTTP 401)",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"info": map[string]any{
			"status_code": resp.StatusCode,
			"url":         url,
		},
	})
}
