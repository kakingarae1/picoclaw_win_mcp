// Package teams implements a Microsoft Teams channel for PicoClaw.
// It receives messages via Bot Framework webhook and replies using the
// Bot Connector REST API.
package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Enabled      bool     `yaml:"enabled"`
	AppID        string   `yaml:"app_id"`
	AllowedUsers []string `yaml:"allowed_users"`
}

type Activity struct {
	Type         string    `json:"type"`
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	ServiceURL   string    `json:"serviceUrl"`
	ChannelID    string    `json:"channelId"`
	From         Entity    `json:"from"`
	Conversation Entity    `json:"conversation"`
	Recipient    Entity    `json:"recipient"`
	Text         string    `json:"text"`
	ReplyToID    string    `json:"replyToId,omitempty"`
}

type Entity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Handler struct {
	cfg    Config
	appPwd string
	chatFn func(ctx context.Context, userID, msg string) (string, error)
	tokenCache struct {
		sync.Mutex
		token   string
		expires time.Time
	}
}

func New(cfg Config, appPassword string, chatFn func(ctx context.Context, userID, msg string) (string, error)) *Handler {
	return &Handler{cfg: cfg, appPwd: appPassword, chatFn: chatFn}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	if err := h.verifyToken(r.Header.Get("Authorization")); err != nil {
		fmt.Printf("[teams] auth failed: %v\n", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var activity Activity
	if err := json.Unmarshal(body, &activity); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if activity.Type != "message" || strings.TrimSpace(activity.Text) == "" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if len(h.cfg.AllowedUsers) > 0 && !h.isAllowed(activity.From.ID, activity.From.Name) {
		fmt.Printf("[teams] rejected user: %s (%s)\n", activity.From.Name, activity.From.ID)
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		text := strings.TrimSpace(activity.Text)
		fmt.Printf("[teams] message from %s: %s\n", activity.From.Name, text)
		resp, err := h.chatFn(ctx, activity.From.ID, text)
		if err != nil {
			resp = fmt.Sprintf("Sorry, I encountered an error: %v", err)
		}
		if err := h.reply(ctx, activity, resp); err != nil {
			fmt.Printf("[teams] reply failed: %v\n", err)
		}
	}()
}

func (h *Handler) reply(ctx context.Context, incoming Activity, text string) error {
	token, err := h.getToken(ctx)
	if err != nil {
		return fmt.Errorf("getting token: %w", err)
	}
	reply := map[string]interface{}{
		"type": "message", "from": incoming.Recipient,
		"conversation": incoming.Conversation, "recipient": incoming.From,
		"text": text, "replyToId": incoming.ID,
	}
	body, _ := json.Marshal(reply)
	url := fmt.Sprintf("%sv3/conversations/%s/activities/%s",
		incoming.ServiceURL, incoming.Conversation.ID, incoming.ID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reply HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (h *Handler) getToken(ctx context.Context) (string, error) {
	h.tokenCache.Lock()
	defer h.tokenCache.Unlock()
	if h.tokenCache.token != "" && time.Now().Before(h.tokenCache.expires) {
		return h.tokenCache.token, nil
	}
	body := strings.NewReader(fmt.Sprintf(
		"grant_type=client_credentials&client_id=%s&client_secret=%s&scope=https%%3A%%2F%%2Fapi.botframework.com%%2F.default",
		h.cfg.AppID, h.appPwd,
	))
	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://login.microsoftonline.com/botframework.com/oauth2/v2.0/token", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("empty token from Microsoft")
	}
	h.tokenCache.token = result.AccessToken
	h.tokenCache.expires = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return result.AccessToken, nil
}

func (h *Handler) verifyToken(authHeader string) error {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return fmt.Errorf("missing Bearer token")
	}
	if len(strings.TrimPrefix(authHeader, "Bearer ")) < 10 {
		return fmt.Errorf("token too short")
	}
	return nil
}

func (h *Handler) isAllowed(id, name string) bool {
	idL, nameL := strings.ToLower(id), strings.ToLower(name)
	for _, u := range h.cfg.AllowedUsers {
		uL := strings.ToLower(u)
		if uL == idL || uL == nameL || strings.Contains(nameL, uL) {
			return true
		}
	}
	return false
}
