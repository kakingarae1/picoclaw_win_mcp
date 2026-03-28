package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Sender struct{ webhookURL string }

func NewSender(webhookURL string) *Sender { return &Sender{webhookURL: webhookURL} }

func (s *Sender) Send(ctx context.Context, text string) error {
	payload := map[string]interface{}{
		"type": "message",
		"attachments": []map[string]interface{}{{
			"contentType": "application/vnd.microsoft.card.adaptive",
			"content": map[string]interface{}{
				"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
				"type": "AdaptiveCard", "version": "1.4",
				"body": []map[string]interface{}{{"type": "TextBlock", "text": text, "wrap": true}},
			},
		}},
	}
	return s.post(ctx, payload)
}

func (s *Sender) SendCard(ctx context.Context, title, text string, facts map[string]string) error {
	var fi []map[string]string
	for k, v := range facts {
		fi = append(fi, map[string]string{"title": k, "value": v})
	}
	payload := map[string]interface{}{
		"type": "message",
		"attachments": []map[string]interface{}{{
			"contentType": "application/vnd.microsoft.card.adaptive",
			"content": map[string]interface{}{
				"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
				"type": "AdaptiveCard", "version": "1.4",
				"body": []map[string]interface{}{
					{"type": "TextBlock", "size": "Medium", "weight": "Bolder", "text": title},
					{"type": "TextBlock", "text": text, "wrap": true},
					{"type": "FactSet", "facts": fi},
				},
			},
		}},
	}
	return s.post(ctx, payload)
}

func (s *Sender) post(ctx context.Context, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("teams webhook HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
