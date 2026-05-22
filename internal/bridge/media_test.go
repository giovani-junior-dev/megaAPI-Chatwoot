package bridge

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsHEIC_DetectsMimeNameURL(t *testing.T) {
	require.True(t, isHEIC("image/heic", "", ""))
	require.True(t, isHEIC("image/heif", "", ""))
	require.True(t, isHEIC("", "foo.HEIC", ""))
	require.True(t, isHEIC("", "", "https://x/y.heif"))
	require.False(t, isHEIC("image/jpeg", "foo.jpg", "https://x/y.jpg"))
}

func TestPrepareMedia_RejectsHEIC(t *testing.T) {
	s := &Server{}
	_, err := s.prepareMedia(context.Background(), Attachment{
		Kind: "image", MimeType: "image/heic", URL: "https://x/y.heic",
	})
	require.Error(t, err)
	var fe fatalError
	require.True(t, errors.As(err, &fe), "must be notRetriable")
	require.Contains(t, err.Error(), "HEIC")
}

func TestPrepareMedia_RejectsOversizeImage(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(waLimitImage+1))
		w.WriteHeader(http.StatusOK)
	}))
	defer mock.Close()
	s := &Server{}
	_, err := s.prepareMedia(context.Background(), Attachment{Kind: "image", URL: mock.URL})
	require.Error(t, err)
	var fe fatalError
	require.True(t, errors.As(err, &fe))
	require.Contains(t, err.Error(), "exceeds WhatsApp limit")
}

func TestPrepareMedia_AcceptsWithinLimit(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
	}))
	defer mock.Close()
	s := &Server{}
	att, err := s.prepareMedia(context.Background(), Attachment{Kind: "image", URL: mock.URL})
	require.NoError(t, err)
	require.Equal(t, mock.URL, att.URL)
}

func TestPrepareMedia_NoContentLengthPassthrough(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mock.Close()
	s := &Server{}
	_, err := s.prepareMedia(context.Background(), Attachment{Kind: "image", URL: mock.URL})
	require.NoError(t, err)
}

func TestPrepareMedia_AudioMP3MimeAcceptedKindBecomesPTT(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
	}))
	defer mock.Close()
	s := &Server{}
	att, err := s.prepareMedia(context.Background(), Attachment{
		Kind: "audio", MimeType: "audio/mpeg", URL: mock.URL,
	})
	require.NoError(t, err)
	require.Equal(t, "ptt", att.Kind)
}

func TestPrepareMedia_AudioOggOpusAcceptedKindBecomesPTT(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
	}))
	defer mock.Close()
	s := &Server{}
	for _, mime := range []string{"audio/ogg", "audio/opus"} {
		att, err := s.prepareMedia(context.Background(), Attachment{
			Kind: "audio", MimeType: mime, URL: mock.URL,
		})
		require.NoError(t, err, "mime=%s", mime)
		require.Equal(t, "ptt", att.Kind)
	}
}

func TestPrepareMedia_AudioWavRejected(t *testing.T) {
	s := &Server{}
	_, err := s.prepareMedia(context.Background(), Attachment{
		Kind: "audio", MimeType: "audio/wav", URL: "https://x/y.wav",
	})
	require.Error(t, err)
	var fe fatalError
	require.True(t, errors.As(err, &fe))
	require.Contains(t, err.Error(), "audio format")
}

func TestPrepareMedia_AudioByExtensionWhenMimeMissing(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
	}))
	defer mock.Close()
	s := &Server{}
	att, err := s.prepareMedia(context.Background(), Attachment{
		Kind: "audio", FileName: "voice.ogg", URL: mock.URL,
	})
	require.NoError(t, err)
	require.Equal(t, "ptt", att.Kind)
}

func TestPrepareMedia_AudioOversizeRejected(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(waLimitAudio+1))
		w.WriteHeader(http.StatusOK)
	}))
	defer mock.Close()
	s := &Server{}
	_, err := s.prepareMedia(context.Background(), Attachment{
		Kind: "audio", MimeType: "audio/mpeg", URL: mock.URL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds WhatsApp limit")
}

func TestIsAcceptedAudio(t *testing.T) {
	require.True(t, isAcceptedAudio("audio/mpeg", ""))
	require.True(t, isAcceptedAudio("audio/mp3", ""))
	require.True(t, isAcceptedAudio("audio/ogg", ""))
	require.True(t, isAcceptedAudio("audio/opus", ""))
	require.True(t, isAcceptedAudio("", "mp3"))
	require.True(t, isAcceptedAudio("", "ogg"))
	require.False(t, isAcceptedAudio("audio/wav", "wav"))
	require.False(t, isAcceptedAudio("audio/x-m4a", "m4a"))
}

func TestWaLimitFor(t *testing.T) {
	require.Equal(t, int64(waLimitImage), waLimitFor("image"))
	require.Equal(t, int64(waLimitVideo), waLimitFor("video"))
	require.Equal(t, int64(waLimitAudio), waLimitFor("audio"))
	require.Equal(t, int64(waLimitDoc), waLimitFor("document"))
	require.Equal(t, int64(waLimitDoc), waLimitFor("unknown"))
}
