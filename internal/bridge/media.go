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
	case "image":
		return waLimitImage
	case "video":
		return waLimitVideo
	case "audio":
		return waLimitAudio
	default:
		return waLimitDoc
	}
}

func (s *Server) prepareMedia(ctx context.Context, att Attachment) (Attachment, error) {
	if att.Kind == "image" && isHEIC(att.MimeType, att.FileName, att.URL) {
		return att, notRetriable(fmt.Errorf("HEIC images not supported by WhatsApp — please convert to JPEG/PNG before sending"))
	}
	size, err := headSize(ctx, att.URL)
	if err == nil && size > 0 {
		if limit := waLimitFor(att.Kind); size > limit {
			return att, notRetriable(fmt.Errorf("media %d bytes exceeds WhatsApp limit %d for %s", size, limit, att.Kind))
		}
	}
	return att, nil
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
