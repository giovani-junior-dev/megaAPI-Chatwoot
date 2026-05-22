package bridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSplitDataURL(t *testing.T) {
	p, m := splitDataURL("data:image/jpeg;base64,QUJD")
	require.Equal(t, "QUJD", p)
	require.Equal(t, "image/jpeg", m)

	p, m = splitDataURL("data:application/pdf,xxxx")
	require.Equal(t, "xxxx", p)
	require.Equal(t, "application/pdf", m)

	p, m = splitDataURL("plain-base64-string")
	require.Equal(t, "plain-base64-string", p)
	require.Equal(t, "", m)
}

func TestDownloadMegaAPIMedia_DecodesDataURL(t *testing.T) {
	wantBytes := []byte("hello-jpeg-bytes")
	dataURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(wantBytes)
	var gotBody map[string]any
	mega := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		require.Contains(t, r.URL.Path, "/rest/instance/downloadMediaMessage/inst1")
		require.Equal(t, "Bearer megaTok", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": false, "message": "ok", "data": dataURL,
		})
	}))
	defer mega.Close()
	key := bytes.Repeat([]byte{1}, 32)
	tokEnc, _ := Encrypt([]byte("megaTok"), key)
	tenant := Tenant{
		ID: uuid.New(), MegaAPIHost: mega.URL, MegaAPIInstance: "inst1",
		MegaAPITokenEnc: tokEnc,
	}
	s := &Server{Key: key}
	a := Attachment{
		URL: "https://wa/x.enc", Kind: "image", MimeType: "image/jpeg",
		MediaKey: "MK==", DirectPath: "/v/path",
	}
	data, mime, err := s.downloadMegaAPIMedia(context.Background(), tenant, a)
	require.NoError(t, err)
	require.Equal(t, wantBytes, data)
	require.Equal(t, "image/jpeg", mime)

	mk := gotBody["messageKeys"].(map[string]any)
	require.Equal(t, "MK==", mk["mediaKey"])
	require.Equal(t, "/v/path", mk["directPath"])
	require.Equal(t, "https://wa/x.enc", mk["url"])
	require.Equal(t, "image/jpeg", mk["mimetype"])
	require.Equal(t, "image", mk["messageType"])
}

func TestDownloadMegaAPIMedia_ErrorResponse(t *testing.T) {
	mega := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": true, "message": "media not found", "data": "",
		})
	}))
	defer mega.Close()
	key := bytes.Repeat([]byte{1}, 32)
	tokEnc, _ := Encrypt([]byte("megaTok"), key)
	tenant := Tenant{MegaAPIHost: mega.URL, MegaAPIInstance: "i", MegaAPITokenEnc: tokEnc}
	s := &Server{Key: key}
	_, _, err := s.downloadMegaAPIMedia(context.Background(), tenant,
		Attachment{MediaKey: "X", Kind: "image"})
	require.Error(t, err)
	require.False(t, isRetriable(err))
	require.Contains(t, err.Error(), "media not found")
}

func TestFetchInboundMedia_FallsBackToPublicURL(t *testing.T) {
	want := []byte("png-bytes")
	pub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(want)
	}))
	defer pub.Close()
	s := &Server{}
	data, mime, err := s.fetchInboundMedia(context.Background(), Tenant{},
		Attachment{URL: pub.URL, Kind: "image"})
	require.NoError(t, err)
	require.Equal(t, want, data)
	require.Equal(t, "image/png", mime)
}

func TestWaAttachment_ExtractsMediaKeyDirectPath(t *testing.T) {
	body := []byte(`{"message":{"imageMessage":{"url":"https://wa/x.enc","mimetype":"image/jpeg","mediaKey":"MK==","directPath":"/v/path","caption":"cap"}}}`)
	p, err := parseWA(body)
	require.NoError(t, err)
	a, ok := waAttachment(p)
	require.True(t, ok)
	require.Equal(t, "MK==", a.MediaKey)
	require.Equal(t, "/v/path", a.DirectPath)
	require.Equal(t, "image", a.Kind)
	require.Equal(t, "cap", a.Caption)
}

func TestCwPostMultipart_UsesMegaAPIForEncryptedAttachment(t *testing.T) {
	wantBytes := []byte("decoded-jpeg-bytes")
	dataURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(wantBytes)
	mega := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": false, "data": dataURL,
		})
	}))
	defer mega.Close()
	var cwBody []byte
	var cwCT string
	cw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cwCT = r.Header.Get("Content-Type")
		cwBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer cw.Close()
	key := bytes.Repeat([]byte{1}, 32)
	cwTokEnc, _ := Encrypt([]byte("cwTok"), key)
	megaTokEnc, _ := Encrypt([]byte("megaTok"), key)
	tenant := Tenant{
		ID: uuid.New(), ChatwootURL: cw.URL, ChatwootTokenEnc: cwTokEnc,
		ChatwootAccountID: 1, ChatwootInboxID: 5,
		MegaAPIHost: mega.URL, MegaAPIInstance: "inst1", MegaAPITokenEnc: megaTokEnc,
	}
	s := &Server{Key: key}
	att := []Attachment{{
		URL: "https://wa/x.enc", Kind: "image", MimeType: "image/jpeg",
		MediaKey: "MK==", DirectPath: "/v/path",
	}}
	err := s.postChatwootMessage(context.Background(), tenant, 42, "cap", "WAID-1", att)
	require.NoError(t, err)
	require.Contains(t, cwCT, "multipart/form-data")
	body := string(cwBody)
	require.Contains(t, body, string(wantBytes), "decoded bytes must be in multipart payload")
	require.Contains(t, body, "Content-Type: image/jpeg")
	require.Contains(t, body, `name="attachments[]"`)
	require.Contains(t, body, `name="content_attributes[external_id]"`)
	require.Contains(t, body, "WAID-1")
}

func TestChooseFileName_KeepsExtensionFromURL(t *testing.T) {
	a := Attachment{URL: "https://x/foo.png", Kind: "image"}
	require.Equal(t, "foo.png", chooseFileName(a, "image/png"))
}

func TestChooseFileName_FallsBackToMime(t *testing.T) {
	a := Attachment{URL: "", Kind: "image"}
	name := chooseFileName(a, "image/jpeg")
	require.True(t, strings.HasSuffix(name, ".jpg"), "got %s", name)
}

func TestChooseFileName_PrefersExplicit(t *testing.T) {
	a := Attachment{FileName: "report.pdf", URL: "https://x/u", Kind: "document"}
	require.Equal(t, "report.pdf", chooseFileName(a, "application/pdf"))
}
