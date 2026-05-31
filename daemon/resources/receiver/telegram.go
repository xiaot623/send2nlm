// Telegram Receiver — delivers pipeline artifacts to a Telegram chat.
// Placed in ~/.send2nlm/receiver/ and loaded by the yaegi script engine.
//
// Configuration (in ~/.send2nlm/config.json):
//   {
//     "receivers": {
//       "telegram": {
//         "enabled": true,
//         "bot_token": "123456:ABC-DEF...",
//         "chat_id": "-1001234567890"
//       }
//     }
//   }
//
// Prerequisites: create a bot via @BotFather and get your chat ID via @userinfobot.

package receiver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	"send2nlm/sdk"
)

// TelegramReceiver delivers artifacts via Telegram Bot API.
type TelegramReceiver struct{}

func (r *TelegramReceiver) Name() string { return "telegram" }

func (r *TelegramReceiver) Receive(ctx context.Context, resources []sdk.Resource) error {
	cfg := sdk.LoadConfig()
	rc, ok := cfg.Receivers["telegram"]
	if !ok {
		return nil // not configured, skip silently
	}

	enabled, _ := rc["enabled"].(bool)
	if !enabled {
		return nil
	}

	botToken, _ := rc["bot_token"].(string)
	chatID, _ := rc["chat_id"].(string)
	if botToken == "" || chatID == "" {
		return fmt.Errorf("telegram: bot_token or chat_id not configured")
	}

	// Send a summary message
	sourceURL := ""
	if len(resources) > 0 {
		sourceURL = resources[0].SourceURL
	}
	notebookTitle := ""
	if len(resources) > 0 {
		notebookTitle = resources[0].NotebookTitle
	}

	msg := fmt.Sprintf("📬 *Send2NLM Job Complete*\nNotebook: %s\nSource: %s",
		notebookTitle, sourceURL)
	if err := telegramSendMessage(ctx, botToken, chatID, msg); err != nil {
		return fmt.Errorf("telegram message: %w", err)
	}

	// Send each file
	for _, res := range resources {
		if res.AssetPath == "" {
			continue
		}
		if err := telegramSendDocument(ctx, botToken, chatID, res.AssetPath, res.DeliveryName); err != nil {
			return fmt.Errorf("telegram document %s: %w", res.TaskType, err)
		}
	}

	return nil
}

func telegramSendMessage(ctx context.Context, token, chatID, text string) error {
	body, _ := json.Marshal(map[string]string{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "MarkdownV2",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.telegram.org/bot"+token+"/sendMessage",
		bytes.NewReader(body))
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
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram sendMessage HTTP %d: %s", resp.StatusCode, string(data))
	}
	return nil
}

func telegramSendDocument(ctx context.Context, token, chatID, path, filename string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("chat_id", chatID)
	part, err := writer.CreateFormFile("document", filename)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.telegram.org/bot"+token+"/sendDocument",
		&body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram sendDocument HTTP %d: %s", resp.StatusCode, string(data))
	}
	return nil
}

// Receiver is the exported variable required by the script engine.
var Receiver sdk.Receiver = &TelegramReceiver{}
