// Telegram Receiver — delivers pipeline artifacts to a Telegram chat.
// Placed in ~/.send2nlm/receiver/ and run as a compiled external plugin.
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
	notebookTitle := ""
	notebookURL := ""
	if len(resources) > 0 {
		sourceURL = resources[0].SourceURL
		notebookTitle = resources[0].NotebookTitle
		notebookURL = resources[0].NotebookURL
	}

	msg := telegramSummaryMessage(notebookTitle, notebookURL, sourceURL)
	if err := telegramSendMessage(ctx, botToken, chatID, msg); err != nil {
		return fmt.Errorf("telegram message: %w", err)
	}

	// Send each file
	for _, res := range resources {
		if res.AssetPath == "" {
			continue
		}
		if err := telegramSendResource(ctx, botToken, chatID, res); err != nil {
			return fmt.Errorf("telegram resource %s: %w", res.TaskType, err)
		}
	}

	return nil
}

func telegramSummaryMessage(notebookTitle, notebookURL, sourceURL string) string {
	msg := fmt.Sprintf("📬 *Send2NLM Job Complete*\nNotebook: %s", notebookTitle)
	if notebookURL != "" {
		msg += fmt.Sprintf("\nNotebookLM: %s", notebookURL)
	}
	msg += fmt.Sprintf("\nSource: %s", sourceURL)
	return msg
}

func telegramSendResource(ctx context.Context, token, chatID string, res sdk.Resource) error {
	method, fileField, fields := telegramResourceUploadSpec(res)
	return telegramSendMultipartFile(ctx, token, method, chatID, fileField, res.AssetPath, res.DeliveryName, fields)
}

func telegramResourceUploadSpec(res sdk.Resource) (string, string, map[string]string) {
	switch res.TaskType {
	case "audio_overview":
		return "sendAudio", "audio", nil
	case "video_overview":
		return "sendVideo", "video", map[string]string{"supports_streaming": "true"}
	default:
		return "sendDocument", "document", nil
	}
}

func telegramSendMessage(ctx context.Context, token, chatID, text string) error {
	body, _ := json.Marshal(map[string]string{
		"chat_id": chatID,
		"text":    text,
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
	return telegramSendMultipartFile(ctx, token, "sendDocument", chatID, "document", path, filename, nil)
}

func telegramSendAudio(ctx context.Context, token, chatID, path, filename string) error {
	_, fileField, fields := telegramResourceUploadSpec(sdk.Resource{TaskType: "audio_overview"})
	return telegramSendMultipartFile(ctx, token, "sendAudio", chatID, fileField, path, filename, fields)
}

func telegramSendVideo(ctx context.Context, token, chatID, path, filename string) error {
	_, fileField, fields := telegramResourceUploadSpec(sdk.Resource{TaskType: "video_overview"})
	return telegramSendMultipartFile(ctx, token, "sendVideo", chatID, fileField, path, filename, fields)
}

func telegramSendMultipartFile(ctx context.Context, token, method, chatID, fileField, path, filename string, fields map[string]string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("chat_id", chatID)
	for key, value := range fields {
		_ = writer.WriteField(key, value)
	}
	part, err := writer.CreateFormFile(fileField, filename)
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
		"https://api.telegram.org/bot"+token+"/"+method,
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
		return fmt.Errorf("telegram %s HTTP %d: %s", method, resp.StatusCode, string(data))
	}
	return nil
}

// Receiver is the exported variable required by the external plugin wrapper.
var Receiver sdk.Receiver = &TelegramReceiver{}
