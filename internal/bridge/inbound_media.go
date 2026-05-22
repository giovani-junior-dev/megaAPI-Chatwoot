package bridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path"
	"strings"
)

const cwUploadLimit = 40 * 1024 * 1024

func (s *Server) cwPostMultipart(ctx context.Context, t Tenant, endpoint, content, externalID string, atts []Attachment) error {
	tok, err := Decrypt(t.ChatwootTokenEnc, s.Key)
	if err != nil {
		return notRetriable(err)
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("content", content)
	_ = mw.WriteField("message_type", "incoming")
	_ = mw.WriteField("content_attributes[external_id]", externalID)
	for _, a := range atts {
		data, mime, err := s.fetchInboundMedia(ctx, t, a)
		if err != nil {
			return err
		}
		if int64(len(data)) > cwUploadLimit {
			return notRetriable(fmt.Errorf("attachment %d bytes exceeds %d", len(data), cwUploadLimit))
		}
		part, err := newFilePart(mw, "attachments[]", chooseFileName(a, mime), mime)
		if err != nil {
			return retriable(err)
		}
		if _, err := part.Write(data); err != nil {
			return retriable(err)
		}
	}
	if err := mw.Close(); err != nil {
		return retriable(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return notRetriable(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("api_access_token", string(tok))
	resp, err := httpClient.Do(req)
	if err != nil {
		return retriable(err)
	}
	defer resp.Body.Close()
	return classifyHTTP(resp, "chatwoot "+endpoint)
}

func (s *Server) fetchInboundMedia(ctx context.Context, t Tenant, a Attachment) ([]byte, string, error) {
	if a.MediaKey != "" {
		return s.downloadMegaAPIMedia(ctx, t, a)
	}
	return downloadPublicURL(ctx, a)
}

func (s *Server) downloadMegaAPIMedia(ctx context.Context, t Tenant, a Attachment) ([]byte, string, error) {
	tok, err := Decrypt(t.MegaAPITokenEnc, s.Key)
	if err != nil {
		return nil, "", notRetriable(err)
	}
	body := map[string]any{
		"messageKeys": map[string]any{
			"mediaKey":    a.MediaKey,
			"directPath":  a.DirectPath,
			"url":         a.URL,
			"mimetype":    a.MimeType,
			"messageType": a.Kind,
		},
	}
	endpoint := fmt.Sprintf("%s/rest/instance/downloadMediaMessage/%s",
		strings.TrimRight(t.MegaAPIHost, "/"), t.MegaAPIInstance)
	resp, err := bearerPost(ctx, endpoint, string(tok), body)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if err := classifyHTTP(resp, "megaapi downloadMediaMessage"); err != nil {
		return nil, "", err
	}
	var dl struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
		Data    string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dl); err != nil {
		return nil, "", retriable(fmt.Errorf("decode downloadMediaMessage: %w", err))
	}
	if dl.Error || dl.Data == "" {
		return nil, "", notRetriable(fmt.Errorf("megaapi downloadMediaMessage: %s", dl.Message))
	}
	payload, mime := splitDataURL(dl.Data)
	bin, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", notRetriable(fmt.Errorf("decode base64: %w", err))
	}
	if mime == "" {
		mime = a.MimeType
	}
	if mime == "" {
		mime = defaultMime(a.Kind)
	}
	return bin, mime, nil
}

func splitDataURL(s string) (payload, mime string) {
	if !strings.HasPrefix(s, "data:") {
		return s, ""
	}
	rest := strings.TrimPrefix(s, "data:")
	semi := strings.Index(rest, ";")
	comma := strings.Index(rest, ",")
	if comma < 0 {
		return s, ""
	}
	if semi >= 0 && semi < comma {
		mime = rest[:semi]
	} else {
		mime = rest[:comma]
	}
	return rest[comma+1:], mime
}

func downloadPublicURL(ctx context.Context, a Attachment) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return nil, "", notRetriable(err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", retriable(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return nil, "", retriable(fmt.Errorf("download %s %d", a.URL, resp.StatusCode))
	}
	if resp.StatusCode >= 400 {
		return nil, "", notRetriable(fmt.Errorf("download %s %d", a.URL, resp.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, cwUploadLimit+1))
	if err != nil {
		return nil, "", retriable(err)
	}
	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = a.MimeType
	}
	if mime == "" {
		mime = defaultMime(a.Kind)
	}
	return data, mime, nil
}

func chooseFileName(a Attachment, mime string) string {
	if a.FileName != "" {
		return a.FileName
	}
	if base := path.Base(a.URL); base != "" && base != "/" && base != "." && !strings.Contains(base, "?") {
		if ext := path.Ext(base); ext != "" {
			return base
		}
	}
	return a.Kind + extFromMime(mime, a.Kind)
}

func extFromMime(mime, kind string) string {
	if e := mimeExt(mime); e != "" {
		return e
	}
	switch kind {
	case "image":
		return ".jpg"
	case "audio":
		return ".ogg"
	case "video":
		return ".mp4"
	}
	return ".bin"
}

func defaultMime(kind string) string {
	switch kind {
	case "image":
		return "image/jpeg"
	case "audio":
		return "audio/ogg"
	case "video":
		return "video/mp4"
	}
	return "application/octet-stream"
}

func newFilePart(mw *multipart.Writer, field, filename, contentType string) (io.Writer, error) {
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename))
	h.Set("Content-Type", contentType)
	return mw.CreatePart(h)
}

