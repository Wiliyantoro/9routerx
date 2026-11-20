// 9RouterX — this file is part of a port maintained by https://github.com/yanstd888/ — https://t.me/aiclub_id
package backup

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"9routerx/proxy/internal/db"
	"9routerx/proxy/internal/log"
)

// Remote push (N8): after a local backup succeeds, the JSON export is pushed
// to the configured off-host destination — a Telegram chat's documents, or a
// GitHub repository's contents API. The push is best-effort by contract: a
// failed push records the error on the status row and never fails the local
// backup it rode on, because the local file is already durable and a dead
// remote must not stop the schedule from rotating.
//
// The API bases are vars so tests run against local servers; ROUTERX_BLOCK_NET
// stays effective for anything that does not override them.

var (
	tgAPIBase = "https://api.telegram.org"
	ghAPIBase = "https://api.github.com"
)

// remoteHTTPTimeout bounds one push. A hung remote must not hold the backup
// schedule's slot for the tick's whole two-minute budget.
const remoteHTTPTimeout = 45 * time.Second

// PushRemote uploads the newest backup to cfg's destination and records the
// outcome on the status row. Returns the destination kind, "" when nothing
// was configured.
func (m *Manager) PushRemote(ctx context.Context, cfg db.AutoBackupConfig, result *BackupResult) (kind, status, errMsg string) {
	if !cfg.RemoteArmed() || result == nil {
		return "", "", ""
	}
	// The JSON export is the pushed artifact: it is the readable, restorable
	// one (the .sqlite copy needs a matching engine), and it is far smaller.
	file := result.JSONPath
	if file == "" {
		file = result.SQLitePath
	}
	payload, err := os.ReadFile(file)
	if err != nil {
		m.recordRemote(ctx, cfg, "error", "read backup: "+err.Error())
		return cfg.Remote.Kind, "error", err.Error()
	}

	var pushErr error
	switch cfg.Remote.Kind {
	case db.RemoteKindTelegram:
		pushErr = pushTelegram(ctx, cfg.Remote, filepath.Base(file), payload)
	case db.RemoteKindGithub:
		pushErr = pushGithub(ctx, cfg.Remote, payload)
	default:
		return "", "", ""
	}
	if pushErr != nil {
		log.Warn("backup", "remote push failed", "kind", cfg.Remote.Kind, "error", pushErr)
		m.recordRemote(ctx, cfg, "error", pushErr.Error())
		return cfg.Remote.Kind, "error", pushErr.Error()
	}
	m.recordRemote(ctx, cfg, "success", "")
	return cfg.Remote.Kind, "success", ""
}

// recordRemote stamps the push outcome on the status row, preserving the
// local backup fields the same row already carries.
func (m *Manager) recordRemote(ctx context.Context, cfg db.AutoBackupConfig, status, errMsg string) {
	st, err := m.repo.GetAutoBackupStatus(ctx)
	if err != nil {
		st = db.AutoBackupStatus{}
	}
	st.LastRemoteKind = cfg.Remote.Kind
	st.LastRemoteAt = m.now().UTC().Format(time.RFC3339)
	st.LastRemoteStatus = status
	st.LastRemoteError = errMsg
	if err := m.repo.SetAutoBackupStatus(ctx, st); err != nil {
		log.Warn("backup", "write auto-backup remote status failed", "error", err)
	}
}

// pushTelegram uploads the backup as a document to the configured chat.
func pushTelegram(ctx context.Context, r db.AutoBackupRemote, name string, payload []byte) error {
	client := &http.Client{Timeout: remoteHTTPTimeout}
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	if err := form.WriteField("chat_id", r.TelegramChat); err != nil {
		return err
	}
	part, err := form.CreateFormFile("document", name)
	if err != nil {
		return err
	}
	if _, err := part.Write(payload); err != nil {
		return err
	}
	if err := form.Close(); err != nil {
		return err
	}
	url := tgAPIBase + "/bot" + r.TelegramToken + "/sendDocument"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("telegram answered %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

// pushGithub writes the backup to the repository's contents API, creating or
// updating the file in one call. The existing file's sha is fetched first:
// the API refuses an update that does not name it.
func pushGithub(ctx context.Context, r db.AutoBackupRemote, payload []byte) error {
	client := &http.Client{Timeout: remoteHTTPTimeout}
	branch := r.GithubBranch
	if branch == "" {
		branch = "main"
	}
	target := r.GithubPath
	if target == "" {
		target = "backups/9routerx-backup.json"
	}
	ownerRepo := r.GithubRepo

	// Existing file sha, when there is an existing file at all.
	var existingSHA string
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		ghAPIBase+"/repos/"+ownerRepo+"/contents/"+path.Clean(target)+"?ref="+branch, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.GithubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusOK {
		var existing struct {
			SHA string `json:"sha"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&existing); err == nil {
			existingSHA = existing.SHA
		}
		resp.Body.Close()
	} else {
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("github contents probe answered %d", resp.StatusCode)
		}
	}

	payloadBody := map[string]any{
		"message": "9routerx auto-backup " + time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"content": base64.StdEncoding.EncodeToString(payload),
		"branch":  branch,
	}
	if existingSHA != "" {
		payloadBody["sha"] = existingSHA
	}
	encoded, err := json.Marshal(payloadBody)
	if err != nil {
		return err
	}
	put, err := http.NewRequestWithContext(ctx, http.MethodPut,
		ghAPIBase+"/repos/"+ownerRepo+"/contents/"+path.Clean(target), bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	put.Header.Set("Authorization", "Bearer "+r.GithubToken)
	put.Header.Set("Accept", "application/vnd.github+json")
	put.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(put)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
		return nil
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp2.Body, 300))
	return fmt.Errorf("github answered %d: %s", resp2.StatusCode, strings.TrimSpace(string(snippet)))
}
