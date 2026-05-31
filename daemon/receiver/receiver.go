package receiver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"send2nlm/core"
)

func Deliver(ctx context.Context, cfg core.RuntimeConfig, job *core.Job, results map[string]core.TaskResult) error {
	appCfg, err := core.LoadAppConfig(cfg)
	if err != nil {
		return err
	}
	files, err := copyDownloads(results)
	if err != nil {
		return err
	}
	log.Printf("[receiver] copied %d files to Downloads", len(files))

	if rcfg, ok := appCfg.Receivers["telegram"]; ok && rcfg.Enabled && rcfg.BotToken != "" && rcfg.ChatID != "" {
		log.Printf("[receiver] sending via Telegram to chat %s...", rcfg.ChatID)
		if err := sendTelegram(ctx, rcfg, job, files); err != nil {
			log.Printf("[receiver] Telegram send failed: %v", err)
			return err
		}
	}
	return nil
}

func copyDownloads(results map[string]core.TaskResult) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	outDir := filepath.Join(home, "Downloads", "send2nlm")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	var copied []string
	for taskType, result := range results {
		if result.AssetPath == "" {
			continue
		}
		src, err := os.ReadFile(result.AssetPath)
		if err != nil {
			return nil, err
		}
		dst := filepath.Join(outDir, fmt.Sprintf("%s_%s%s", time.Now().UTC().Format("20060102_150405"), taskType, filepath.Ext(result.AssetPath)))
		if err := os.WriteFile(dst, src, 0o644); err != nil {
			return nil, err
		}
		copied = append(copied, dst)
	}
	return copied, nil
}

func sendTelegram(ctx context.Context, rcfg core.ReceiverConfig, job *core.Job, files []string) error {
	if err := telegramMessage(ctx, rcfg.BotToken, rcfg.ChatID, fmt.Sprintf("Send2NLM job done\nNotebook: %s\nURL: %s", job.NotebookTitle, job.URL)); err != nil {
		return err
	}
	for _, file := range files {
		if err := telegramDocument(ctx, rcfg.BotToken, rcfg.ChatID, file); err != nil {
			return err
		}
	}
	return nil
}

func telegramMessage(ctx context.Context, token, chatID, text string) error {
	body, _ := json.Marshal(map[string]string{
		"chat_id": chatID,
		"text":    text,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+token+"/sendMessage", bytes.NewReader(body))
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
		return fmt.Errorf("telegram sendMessage failed: %s", string(data))
	}
	return nil
}

func telegramDocument(ctx context.Context, token, chatID, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("chat_id", chatID)
	part, err := writer.CreateFormFile("document", filepath.Base(path))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+token+"/sendDocument", &body)
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
		return fmt.Errorf("telegram sendDocument failed: %s", string(data))
	}
	return nil
}
