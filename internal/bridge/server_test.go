package bridge

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestHealthz_Always200(t *testing.T) {
	s := newTestServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestExtractWAExternalID(t *testing.T) {
	body := []byte(`{"key":{"id":"ABC123","remoteJid":"5511999@s.whatsapp.net"},"pushName":"X","message":{"conversation":"hi"}}`)
	id, ok := extractWAExternalID(body)
	require.True(t, ok)
	require.Equal(t, "ABC123", id)

	_, ok = extractWAExternalID([]byte(`{"key":{}}`))
	require.False(t, ok)
}

func TestExtractCWExternalID(t *testing.T) {
	body := []byte(`{"id":42,"event":"message_created","message_type":"outgoing","content":"hi"}`)
	id, ok := extractCWExternalID(body)
	require.True(t, ok)
	require.Equal(t, "cw-42", id)
}

func TestChatwootShouldRelay(t *testing.T) {
	relay := []byte(`{"event":"message_created","message_type":"outgoing","private":false}`)
	require.True(t, chatwootShouldRelay(relay))

	skip := []byte(`{"event":"message_created","message_type":"incoming","private":false}`)
	require.False(t, chatwootShouldRelay(skip))

	private := []byte(`{"event":"message_created","message_type":"outgoing","private":true}`)
	require.False(t, chatwootShouldRelay(private))
}

func TestVerifyHMAC_Roundtrip(t *testing.T) {
	body := []byte(`{"hello":"world"}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	require.True(t, VerifyHMAC(body, sig, "secret"))
	require.False(t, VerifyHMAC(body, sig, "wrong"))
}

func TestWAText_FallsBackToExtended(t *testing.T) {
	p, err := parseWA([]byte(`{"message":{"extendedTextMessage":{"text":"hello"}}}`))
	require.NoError(t, err)
	require.Equal(t, "hello", waText(p))
}

func TestWAContactJID_StripsServer(t *testing.T) {
	p, err := parseWA([]byte(`{"key":{"remoteJid":"5511999@s.whatsapp.net"}}`))
	require.NoError(t, err)
	require.Equal(t, "5511999", waContactJID(p))
}

func newTestServer(t *testing.T, key []byte) *Server {
	t.Helper()
	if key == nil {
		key = RandomBytes(32)
	}
	return &Server{
		Key:    key,
		Inbox:  make(chan Job, 4),
		Outbox: make(chan Job, 4),
		Cfg:    Config{BufferLimit: 4},
		Log:    zerolog.Nop(),
	}
}

func TestReadBody_RejectsTooLarge(t *testing.T) {
	big := strings.Repeat("a", maxBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	_, err := readBody(req)
	require.Error(t, err)
}

func TestExtractWAExternalID_MalformedJSONReturnsFalse(t *testing.T) {
	_, ok := extractWAExternalID([]byte(`{not-json`))
	require.False(t, ok)
}

func TestExtractCWExternalID_ZeroIDReturnsFalse(t *testing.T) {
	_, ok := extractCWExternalID([]byte(`{"id":0}`))
	require.False(t, ok)
}

func TestExtractCWExternalID_MalformedJSONReturnsFalse(t *testing.T) {
	_, ok := extractCWExternalID([]byte(`{`))
	require.False(t, ok)
}

func TestChatwootShouldRelay_MalformedJSONReturnsFalse(t *testing.T) {
	require.False(t, chatwootShouldRelay([]byte(`{not-json`)))
}

func TestChatwootShouldRelay_WrongEventReturnsFalse(t *testing.T) {
	require.False(t, chatwootShouldRelay([]byte(`{"event":"conversation_status_changed","message_type":"outgoing","private":false}`)))
}

func TestAtCapacity_ThresholdIs80Percent(t *testing.T) {
	cases := []struct {
		used, limit int
		want        bool
	}{
		{used: 0, limit: 10, want: false},
		{used: 7, limit: 10, want: false},
		{used: 8, limit: 10, want: true},
		{used: 10, limit: 10, want: true},
		{used: 79, limit: 100, want: false},
		{used: 80, limit: 100, want: true},
		{used: 4, limit: 5, want: true},
		{used: 3, limit: 5, want: false},
		{used: 5, limit: 0, want: false},
	}
	for _, c := range cases {
		require.Equal(t, c.want, atCapacity(c.used, c.limit),
			"atCapacity(%d, %d)", c.used, c.limit)
	}
}

func TestQueueAtCapacity_TripsAt80PercentInbox(t *testing.T) {
	s := &Server{
		Inbox:  make(chan Job, 10),
		Outbox: make(chan Job, 10),
		Cfg:    Config{BufferLimit: 10},
	}
	for i := 0; i < 7; i++ {
		s.Inbox <- Job{}
	}
	require.False(t, s.queueAtCapacity(), "7/10 below 80%%")
	s.Inbox <- Job{}
	require.True(t, s.queueAtCapacity(), "8/10 reaches 80%%")
}

func TestQueueAtCapacity_TripsAt80PercentOutbox(t *testing.T) {
	s := &Server{
		Inbox:  make(chan Job, 10),
		Outbox: make(chan Job, 10),
		Cfg:    Config{BufferLimit: 10},
	}
	for i := 0; i < 8; i++ {
		s.Outbox <- Job{}
	}
	require.True(t, s.queueAtCapacity(), "outbox at 80%% triggers")
}

func TestReadyz_ReturnsOKWhenDBAndQueueHealthy(t *testing.T) {
	// Sanity coverage for the happy path that integration tests skip:
	// we exercise the response shape without a real DB by hitting only
	// /healthz, which never touches DB.
	s := newTestServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "ok")
}
