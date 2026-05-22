package bridge

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	waLimitImage = 5 * 1024 * 1024
	waLimitVideo = 16 * 1024 * 1024
	waLimitAudio = 16 * 1024 * 1024
	waLimitDoc   = 100 * 1024 * 1024
)

func waLimitFor(kind string) int64 {
	switch kind {
	case "image", "sticker":
		return waLimitImage
	case "video":
		return waLimitVideo
	case "audio", "ptt":
		return waLimitAudio
	default:
		return waLimitDoc
	}
}

func (s *Server) prepareMedia(ctx context.Context, att Attachment) (Attachment, error) {
	if att.Kind == "image" && isHEIC(att.MimeType, att.FileName, att.URL) {
		return att, notRetriable(fmt.Errorf("HEIC images not supported by WhatsApp — please convert to JPEG/PNG before sending"))
	}
	mime, ext := probeMedia(ctx, att)
	if att.Kind == "audio" {
		if !isAcceptedAudio(mime, ext) {
			return att, notRetriable(fmt.Errorf("audio format %q not supported — only mp3 and ogg/opus accepted by WhatsApp", firstNonEmpty(mime, ext)))
		}
		att.Kind = "ptt"
	}
	if att.Kind == "video" && !isAcceptedVideo(mime, ext) {
		return att, notRetriable(fmt.Errorf("video format %q not supported — only mp4 accepted by WhatsApp", firstNonEmpty(mime, ext)))
	}
	size, err := headSize(ctx, att.URL)
	if err == nil && size > 0 {
		if limit := waLimitFor(att.Kind); size > limit {
			return att, notRetriable(fmt.Errorf("media %d bytes exceeds WhatsApp limit %d for %s", size, limit, att.Kind))
		}
	}
	return att, nil
}

func probeMedia(ctx context.Context, a Attachment) (mime, ext string) {
	mime = strings.ToLower(strings.TrimSpace(a.MimeType))
	ext = strings.ToLower(strings.TrimPrefix(extFromURLOrName(a.URL, a.FileName), "."))
	if mime != "" {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, a.URL, nil)
	if err != nil {
		return
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		if i := strings.Index(ct, ";"); i >= 0 {
			ct = ct[:i]
		}
		mime = strings.ToLower(strings.TrimSpace(ct))
	}
	return
}

func extFromURLOrName(rawURL, name string) string {
	if name != "" {
		if i := strings.LastIndex(name, "."); i >= 0 {
			return name[i:]
		}
	}
	u := rawURL
	if i := strings.Index(u, "?"); i >= 0 {
		u = u[:i]
	}
	if i := strings.LastIndex(u, "."); i >= 0 && i > strings.LastIndex(u, "/") {
		return u[i:]
	}
	return ""
}

func isAcceptedVideo(mime, ext string) bool {
	switch mime {
	case "video/mp4":
		return true
	}
	switch ext {
	case "mp4":
		return true
	}
	return false
}

func isAcceptedAudio(mime, ext string) bool {
	switch mime {
	case "audio/mpeg", "audio/mp3":
		return true
	case "audio/ogg", "audio/opus", "audio/ogg; codecs=opus":
		return true
	}
	switch ext {
	case "mp3", "ogg", "opus", "oga":
		return true
	}
	return false
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func isHEIC(mime, name, rawURL string) bool {
	m := strings.ToLower(mime)
	if strings.Contains(m, "heic") || strings.Contains(m, "heif") {
		return true
	}
	for _, s := range []string{strings.ToLower(name), strings.ToLower(rawURL)} {
		if strings.HasSuffix(s, ".heic") || strings.HasSuffix(s, ".heif") {
			return true
		}
	}
	return false
}

func headSize(ctx context.Context, rawURL string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	cl := resp.Header.Get("Content-Length")
	if cl == "" {
		return 0, nil
	}
	return strconv.ParseInt(cl, 10, 64)
}
