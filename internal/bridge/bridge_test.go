package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPostChatwootMessage_WithAttachment_UploadsMultipart(t *testing.T) {
	mediaPayload := []byte("FAKEJPEGBYTES")
	mediaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(mediaPayload)
	}))
	defer mediaSrv.Close()

	var gotContentType string
	var gotToken string
	var gotBody []byte
	cwSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotToken = r.Header.Get("api_access_token")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer cwSrv.Close()

	key := bytes.Repeat([]byte{1}, 32)
	tokEnc, _ := Encrypt([]byte("tok"), key)
	t1 := Tenant{
		ID: uuid.New(), ChatwootURL: cwSrv.URL,
		ChatwootTokenEnc: tokEnc, ChatwootAccountID: 1, ChatwootInboxID: 5,
	}
	s := &Server{Key: key}
	att := []Attachment{{URL: mediaSrv.URL + "/img.jpg", Kind: "image", MimeType: "image/jpeg"}}
	if err := s.postChatwootMessage(context.Background(), t1, 42, "hi", "WAID-1", att); err != nil {
		t.Fatalf("postChatwootMessage: %v", err)
	}
	require.Equal(t, "tok", gotToken)
	require.Contains(t, gotContentType, "multipart/form-data")
	body := string(gotBody)
	require.Contains(t, body, `name="content"`)
	require.Contains(t, body, "hi")
	require.Contains(t, body, `name="message_type"`)
	require.Contains(t, body, "incoming")
	require.Contains(t, body, `name="content_attributes[external_id]"`)
	require.Contains(t, body, "WAID-1")
	require.Contains(t, body, `name="attachments[]"`)
	require.Contains(t, body, "Content-Type: image/jpeg")
	require.Contains(t, body, string(mediaPayload))
}

func TestCwAttachments_DocumentExtractsFileNameAndMime(t *testing.T) {
	body := []byte(`{
		"event":"message_created","id":1,
		"attachments":[
			{"file_type":"file","data_url":"http://cw.example/rails/active_storage/blobs/redirect/eyJ.../contract.pdf","extension":"pdf"}
		]
	}`)
	p, err := parseCW(body)
	require.NoError(t, err)
	atts := cwAttachments(p)
	require.Len(t, atts, 1)
	require.Equal(t, "document", atts[0].Kind)
	require.Equal(t, "contract.pdf", atts[0].FileName)
	require.Equal(t, "application/pdf", atts[0].MimeType)
}

func TestCwAttachments_DocxXlsxZip(t *testing.T) {
	body := []byte(`{
		"event":"message_created","id":1,
		"attachments":[
			{"file_type":"file","data_url":"http://cw/rails/.../report.docx","extension":"docx"},
			{"file_type":"file","data_url":"http://cw/rails/.../sheet.xlsx","extension":"xlsx"},
			{"file_type":"file","data_url":"http://cw/rails/.../bundle.zip","extension":"zip"}
		]
	}`)
	p, err := parseCW(body)
	require.NoError(t, err)
	atts := cwAttachments(p)
	require.Len(t, atts, 3)
	require.Equal(t, "report.docx", atts[0].FileName)
	require.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", atts[0].MimeType)
	require.Equal(t, "sheet.xlsx", atts[1].FileName)
	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", atts[1].MimeType)
	require.Equal(t, "bundle.zip", atts[2].FileName)
	require.Equal(t, "application/zip", atts[2].MimeType)
}

func TestFileNameFromURL_StripsQueryAndUsesBasename(t *testing.T) {
	require.Equal(t, "report.pdf",
		fileNameFromURL("http://x/path/report.pdf?token=abc", ""))
	require.Equal(t, "file.pdf",
		fileNameFromURL("http://x/path/", "pdf"))
	require.Equal(t, "file",
		fileNameFromURL("http://x/path/", ""))
}

func TestMimeFromExt_KnownExtensions(t *testing.T) {
	require.Equal(t, "application/pdf", mimeFromExt("pdf"))
	require.Equal(t, "application/zip", mimeFromExt("zip"))
	require.Equal(t, "", mimeFromExt("unknown"))
}

func TestWaIsFromMe_TrueWhenKeyFromMeTrue(t *testing.T) {
	require.True(t, waIsFromMe([]byte(`{"key":{"id":"X","fromMe":true}}`)))
	require.False(t, waIsFromMe([]byte(`{"key":{"id":"X","fromMe":false}}`)))
	require.False(t, waIsFromMe([]byte(`{"key":{"id":"X"}}`)))
	require.False(t, waIsFromMe([]byte(`not-json`)))
}

func TestRetriable_DefaultIsRetriable(t *testing.T) {
	require.True(t, isRetriable(errors.New("network")))
}

func TestRetriable_FatalIsNotRetried(t *testing.T) {
	require.False(t, isRetriable(notRetriable(errors.New("400"))))
}

func TestRetriable_RetriableExplicit(t *testing.T) {
	require.True(t, isRetriable(retriable(errors.New("500"))))
}

func TestDisplayName_FallsBackToJID(t *testing.T) {
	require.Equal(t, "5511999", displayName("", "5511999"))
	require.Equal(t, "Alice", displayName("Alice", "5511999"))
}

func TestSendMegaAPIText_4xxNotRetriable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"err":"bad"}`))
	}))
	defer srv.Close()
	s, t2 := newBridgeWithMega(t, srv.URL)
	err := s.sendMegaAPIText(context.Background(), t2, "5511999", "hi")
	require.Error(t, err)
	require.False(t, isRetriable(err))
}

func TestSendMegaAPIText_5xxRetriable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	s, t2 := newBridgeWithMega(t, srv.URL)
	err := s.sendMegaAPIText(context.Background(), t2, "5511999", "hi")
	require.Error(t, err)
	require.True(t, isRetriable(err))
}

func TestSendMegaAPIText_2xxOk(t *testing.T) {
	var got atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer plain-mega-token", r.Header.Get("Authorization"))
		require.True(t, strings.HasSuffix(r.URL.Path, "/rest/sendMessage/inst-1/text"))
		got.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s, t2 := newBridgeWithMega(t, srv.URL)
	require.NoError(t, s.sendMegaAPIText(context.Background(), t2, "5511999", "hi"))
	require.Equal(t, int32(1), got.Load())
}

func TestPostChatwootMessage_SendsExternalID(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "plain-cw-token", r.Header.Get("api_access_token"))
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	s, t2 := newBridgeWithCW(t, srv.URL)
	err := s.postChatwootMessage(context.Background(), t2, 99, "hello", "wa-1", nil)
	require.NoError(t, err)
	require.Equal(t, "hello", captured["content"])
	attrs := captured["content_attributes"].(map[string]any)
	require.Equal(t, "wa-1", attrs["external_id"])
}

func TestRunRetryLoop_SucceedsFirstAttempt(t *testing.T) {
	calls := atomic.Int32{}
	err := runRetryLoop(context.Background(), []time.Duration{0, 0, 0}, func() error {
		calls.Add(1)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), calls.Load())
}

func TestRunRetryLoop_RunsFourAttemptsWithThreeBackoffs(t *testing.T) {
	calls := atomic.Int32{}
	err := runRetryLoop(context.Background(), []time.Duration{0, 0, 0}, func() error {
		calls.Add(1)
		return retriable(errors.New("boom"))
	})
	require.Error(t, err)
	require.Equal(t, int32(4), calls.Load())
}

func TestRunRetryLoop_FatalShortCircuits(t *testing.T) {
	calls := atomic.Int32{}
	err := runRetryLoop(context.Background(), []time.Duration{0, 0, 0}, func() error {
		calls.Add(1)
		return notRetriable(errors.New("400"))
	})
	require.Error(t, err)
	require.Equal(t, int32(1), calls.Load())
}

func TestRunRetryLoop_RetriableThenSuccess(t *testing.T) {
	calls := atomic.Int32{}
	err := runRetryLoop(context.Background(), []time.Duration{0, 0, 0}, func() error {
		if calls.Add(1) < 3 {
			return retriable(errors.New("boom"))
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, int32(3), calls.Load())
}

func TestJitterBackoff_StaysWithin25Percent(t *testing.T) {
	const base = 100 * time.Millisecond
	min := time.Duration(float64(base) * 0.75)
	max := time.Duration(float64(base) * 1.25)
	var sum time.Duration
	const samples = 500
	distinct := map[time.Duration]struct{}{}
	for i := 0; i < samples; i++ {
		d := jitterBackoff(base)
		require.GreaterOrEqual(t, d, min, "jitter went below -25%%")
		require.LessOrEqual(t, d, max, "jitter went above +25%%")
		distinct[d] = struct{}{}
		sum += d
	}
	require.Greater(t, len(distinct), 50, "jitter must vary across calls")
	mean := sum / samples
	require.InDelta(t, float64(base), float64(mean), float64(base)*0.10,
		"mean should be near base value")
}

func TestJitterBackoff_ZeroReturnsZero(t *testing.T) {
	require.Equal(t, time.Duration(0), jitterBackoff(0))
}

func TestRunRetryLoop_ContextCancelStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := atomic.Int32{}
	err := runRetryLoop(ctx, []time.Duration{50 * time.Millisecond, 50 * time.Millisecond, 50 * time.Millisecond}, func() error {
		calls.Add(1)
		return retriable(errors.New("boom"))
	})
	require.Error(t, err)
	require.Equal(t, int32(1), calls.Load())
}

func TestCheckBearer_RejectsMismatchAndAcceptsMatch(t *testing.T) {
	key := RandomBytes(32)
	enc, err := Encrypt([]byte("right-token"), key)
	require.NoError(t, err)
	s := &Server{Key: key}
	tn := Tenant{WebhookBearerEnc: enc}

	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{"missing header", "", false},
		{"empty bearer", "Bearer ", false},
		{"wrong token", "Bearer wrong-token", false},
		{"right token", "Bearer right-token", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if c.header != "" {
				req.Header.Set("Authorization", c.header)
			}
			ok, err := s.checkBearer(req, tn)
			require.NoError(t, err)
			require.Equal(t, c.want, ok)
		})
	}
}

func TestCheckBearer_DecryptErrorSurfaces(t *testing.T) {
	s := &Server{Key: RandomBytes(32)}
	tn := Tenant{WebhookBearerEnc: []byte("not-a-valid-ciphertext")}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer anything")
	_, err := s.checkBearer(req, tn)
	require.Error(t, err)
}

func TestCheckHMAC_DecryptErrorSurfaces(t *testing.T) {
	s := &Server{Key: RandomBytes(32)}
	tn := Tenant{HMACSecretEnc: []byte("not-a-valid-ciphertext")}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	_, err := s.checkHMAC(req, tn, []byte(`{}`))
	require.Error(t, err)
}

func newBridgeWithMega(t *testing.T, host string) (*Server, Tenant) {
	t.Helper()
	key := RandomBytes(32)
	enc, err := Encrypt([]byte("plain-mega-token"), key)
	require.NoError(t, err)
	s := &Server{Key: key, Cfg: Config{BufferLimit: 4}}
	tn := Tenant{
		ID:              uuid.New(),
		MegaAPIHost:     host,
		MegaAPIInstance: "inst-1",
		MegaAPITokenEnc: enc,
	}
	return s, tn
}

func newBridgeWithCW(t *testing.T, host string) (*Server, Tenant) {
	t.Helper()
	key := RandomBytes(32)
	enc, err := Encrypt([]byte("plain-cw-token"), key)
	require.NoError(t, err)
	s := &Server{Key: key, Cfg: Config{BufferLimit: 4}}
	tn := Tenant{
		ID:                uuid.New(),
		ChatwootURL:       host,
		ChatwootTokenEnc:  enc,
		ChatwootAccountID: 1,
		ChatwootInboxID:   2,
	}
	return s, tn
}

func TestWaAttachment_ImageMessage(t *testing.T) {
	body := []byte(`{
		"key":{"id":"WAID-1","remoteJid":"5511999999999@s.whatsapp.net","fromMe":false},
		"message":{"imageMessage":{"url":"https://media.example/img.jpg","mimetype":"image/jpeg","caption":"hello"}}
	}`)
	p, err := parseWA(body)
	if err != nil {
		t.Fatalf("parseWA: %v", err)
	}
	att, ok := waAttachment(p)
	if !ok {
		t.Fatalf("expected attachment, got none")
	}
	if att.URL != "https://media.example/img.jpg" {
		t.Errorf("URL: got %q", att.URL)
	}
	if att.MimeType != "image/jpeg" {
		t.Errorf("MimeType: got %q", att.MimeType)
	}
	if att.Caption != "hello" {
		t.Errorf("Caption: got %q", att.Caption)
	}
	if att.Kind != "image" {
		t.Errorf("Kind: got %q", att.Kind)
	}
}

func TestWaAttachment_AudioMessage(t *testing.T) {
	body := []byte(`{
		"key":{"id":"WAID-2","remoteJid":"5511999999999@s.whatsapp.net","fromMe":false},
		"message":{"audioMessage":{"url":"https://media.example/audio.ogg","mimetype":"audio/ogg","ptt":true}}
	}`)
	p, _ := parseWA(body)
	att, ok := waAttachment(p)
	if !ok || att.URL != "https://media.example/audio.ogg" || att.Kind != "audio" {
		t.Fatalf("audio: %+v ok=%v", att, ok)
	}
}

func TestWaAttachment_StickerMessage(t *testing.T) {
	body := []byte(`{
		"key":{"id":"WAID-3","remoteJid":"5511999999999@s.whatsapp.net","fromMe":false},
		"message":{"stickerMessage":{"url":"https://media.example/sticker.webp","mimetype":"image/webp"}}
	}`)
	p, _ := parseWA(body)
	att, ok := waAttachment(p)
	if !ok || att.URL != "https://media.example/sticker.webp" || att.Kind != "sticker" {
		t.Fatalf("sticker: %+v ok=%v", att, ok)
	}
}

func TestWaAttachment_VideoMessage(t *testing.T) {
	body := []byte(`{
		"key":{"id":"WAID-4","remoteJid":"5511999999999@s.whatsapp.net","fromMe":false},
		"message":{"videoMessage":{"url":"https://media.example/v.mp4","mimetype":"video/mp4","caption":"watch"}}
	}`)
	p, _ := parseWA(body)
	att, ok := waAttachment(p)
	if !ok || att.URL != "https://media.example/v.mp4" || att.Kind != "video" || att.Caption != "watch" {
		t.Fatalf("video: %+v ok=%v", att, ok)
	}
}

func TestWaAttachment_DocumentMessage(t *testing.T) {
	body := []byte(`{
		"key":{"id":"WAID-5","remoteJid":"5511999999999@s.whatsapp.net","fromMe":false},
		"message":{"documentMessage":{"url":"https://media.example/doc.pdf","mimetype":"application/pdf","fileName":"contract.pdf","caption":"sign"}}
	}`)
	p, _ := parseWA(body)
	att, ok := waAttachment(p)
	if !ok || att.FileName != "contract.pdf" || att.Kind != "document" {
		t.Fatalf("doc: %+v ok=%v", att, ok)
	}
}

func TestSendMegaAPIMedia_PostsMediaUrlEndpoint(t *testing.T) {
	var path string
	var body map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer mock.Close()
	key := bytes.Repeat([]byte{1}, 32)
	tokEnc, _ := Encrypt([]byte("tok"), key)
	t1 := Tenant{
		MegaAPIHost: mock.URL, MegaAPIInstance: "abc", MegaAPITokenEnc: tokEnc,
	}
	s := &Server{Key: key}
	att := Attachment{URL: "https://m.example/x.jpg", Kind: "image", Caption: "hi"}
	if err := s.sendMegaAPIMedia(context.Background(), t1, "5511999999999", att); err != nil {
		t.Fatalf("sendMegaAPIMedia: %v", err)
	}
	if path != "/rest/sendMessage/abc/mediaUrl" {
		t.Errorf("path: got %q", path)
	}
	md, _ := body["messageData"].(map[string]any)
	if md["url"] != "https://m.example/x.jpg" {
		t.Errorf("url: got %v", md["url"])
	}
	if md["type"] != "image" {
		t.Errorf("type: got %v", md["type"])
	}
	if md["caption"] != "hi" {
		t.Errorf("caption: got %v", md["caption"])
	}
	if md["fileName"] == "" || md["fileName"] == nil {
		t.Errorf("fileName missing")
	}
	if _, ok := md["gifPlayback"]; !ok {
		t.Errorf("gifPlayback missing")
	}
	if _, ok := md["viewOnce"]; !ok {
		t.Errorf("viewOnce missing")
	}
}

func TestCwAttachments_Extracts(t *testing.T) {
	body := []byte(`{
		"event":"message_created","message_type":"outgoing","private":false,"id":42,
		"content":"hello","conversation":{"id":1,"contact_inbox":{"source_id":"5511999999999"}},
		"attachments":[
			{"file_type":"image","data_url":"https://cw.example/a.jpg"},
			{"file_type":"file","data_url":"https://cw.example/b.pdf"}
		]
	}`)
	p, err := parseCW(body)
	if err != nil {
		t.Fatalf("parseCW: %v", err)
	}
	atts := cwAttachments(p)
	if len(atts) != 2 {
		t.Fatalf("expected 2, got %d", len(atts))
	}
	if atts[0].URL != "https://cw.example/a.jpg" || atts[0].Kind != "image" {
		t.Errorf("att0: %+v", atts[0])
	}
	if atts[1].Kind != "document" {
		t.Errorf("att1.Kind: got %q want document", atts[1].Kind)
	}
}

func TestCwAttachments_EmptyDataURLSkipped(t *testing.T) {
	body := []byte(`{
		"event":"message_created","id":1,
		"attachments":[
			{"file_type":"image","data_url":""},
			{"file_type":"video","data_url":"https://cw.example/v.mp4"}
		]
	}`)
	p, err := parseCW(body)
	require.NoError(t, err)
	atts := cwAttachments(p)
	require.Len(t, atts, 1, "empty data_url entries must be dropped")
	require.Equal(t, "video", atts[0].Kind)
}

func TestCwTypeToMega_UnknownDefaultsToDocument(t *testing.T) {
	require.Equal(t, "document", cwTypeToMega("application/pdf"))
	require.Equal(t, "document", cwTypeToMega(""))
	require.Equal(t, "image", cwTypeToMega("image"))
	require.Equal(t, "audio", cwTypeToMega("audio"))
	require.Equal(t, "video", cwTypeToMega("video"))
}

func TestWaText_ConversationPreferredOverExtended(t *testing.T) {
	p, err := parseWA([]byte(`{"message":{"conversation":"primary","extendedTextMessage":{"text":"fallback"}}}`))
	require.NoError(t, err)
	require.Equal(t, "primary", waText(p))
}

func TestWaContactJID_NoServerSuffixReturnsAsIs(t *testing.T) {
	p, err := parseWA([]byte(`{"key":{"remoteJid":"5511999"}}`))
	require.NoError(t, err)
	require.Equal(t, "5511999", waContactJID(p))
}

func TestSendMegaAPIText_DecryptErrorIsFatal(t *testing.T) {
	s := &Server{Key: RandomBytes(32)}
	// Ciphertext encrypted with a different key surfaces a decrypt failure
	// that must NOT be retried (no point hammering megaAPI without a token).
	bogus := bytes.Repeat([]byte{0xAA}, 64)
	tn := Tenant{MegaAPIHost: "http://nowhere.invalid", MegaAPIInstance: "i", MegaAPITokenEnc: bogus}
	err := s.sendMegaAPIText(context.Background(), tn, "5511", "hi")
	require.Error(t, err)
	require.False(t, isRetriable(err), "decrypt failure must be fatal")
}

func TestSendMegaAPIMedia_DecryptErrorIsFatal(t *testing.T) {
	s := &Server{Key: RandomBytes(32)}
	bogus := bytes.Repeat([]byte{0xBB}, 64)
	tn := Tenant{MegaAPIHost: "http://nowhere.invalid", MegaAPIInstance: "i", MegaAPITokenEnc: bogus}
	err := s.sendMegaAPIMedia(context.Background(), tn, "5511",
		Attachment{URL: "https://m/x.jpg", Kind: "image"})
	require.Error(t, err)
	require.False(t, isRetriable(err))
}

// HTTP-layer transport failure on megaAPI must be classified retriable so the
// worker retries the job after backoff instead of marking the message failed.
func TestSendMegaAPIText_TransportErrorIsRetriable(t *testing.T) {
	key := RandomBytes(32)
	enc, err := Encrypt([]byte("tok"), key)
	require.NoError(t, err)
	s := &Server{Key: key}
	// Unreachable host — Dial fails, classifyHTTP never runs, retriable() wraps.
	tn := Tenant{
		MegaAPIHost:     "http://127.0.0.1:1", // port 1 reserved/unreachable
		MegaAPIInstance: "i",
		MegaAPITokenEnc: enc,
	}
	err = s.sendMegaAPIText(context.Background(), tn, "5511", "hi")
	require.Error(t, err)
	require.True(t, isRetriable(err), "transport failure must retry")
}

func TestWaAttachment_TextOnly_ReturnsFalse(t *testing.T) {
	body := []byte(`{
		"key":{"id":"WAID-6","remoteJid":"5511999999999@s.whatsapp.net","fromMe":false},
		"message":{"conversation":"hi"}
	}`)
	p, _ := parseWA(body)
	if _, ok := waAttachment(p); ok {
		t.Fatalf("expected no attachment for text-only")
	}
}
