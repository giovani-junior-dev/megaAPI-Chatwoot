# Media Bidirectional Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add image/audio/video/document/sticker support both directions (WhatsApp → Chatwoot via webhook, Chatwoot → WhatsApp via megaAPI `sendMessage/{instance}/mediaUrl`), pass-through URL only.

**Architecture:** Extend existing `waPayload`/`cwPayload` structs in `internal/bridge/bridge.go` to carry media fields. Add `waMedia()` to extract attachment from WA message. Add `sendMegaAPIMedia()` mirror to `sendMegaAPIText()`. Modify `processInbound`/`processOutbound` to handle attachments. No new files. No new packages.

**Tech Stack:** Go 1.25, chi v5, pgx v5, zerolog, testify. Real megaAPI host pattern `POST /rest/sendMessage/{instance}/mediaUrl`. Real Chatwoot pattern `attachments[].file_url` in incoming messages, `attachments[].data_url` in outgoing webhook.

---

## Constraints

- **No new files.** All changes in `internal/bridge/bridge.go` and tests in `internal/bridge/bridge_test.go` + `internal/bridge/handlers_integration_test.go`.
- **No new dependencies.** Use existing stdlib + chi + pgx + zerolog + testify.
- **Pass-through URL only.** Bridge does NOT download/upload bytes. Sends URL string to Chatwoot and megaAPI.
- **No refactor of existing code.** Edit minimal, surgical.
- **TDD strict.** RED test before each production change.
- **Production LOC ≤200.** Test LOC ≤200.

---

## File Structure

| File | Change |
|------|--------|
| `internal/bridge/bridge.go` | Extend `waPayload` (image/audio/video/document/sticker fields); add `waAttachment()` returning `Attachment`; add `cwAttachment` struct on `cwPayload`; add `sendMegaAPIMedia()`; modify `processInbound` to call `postChatwootMessage` with attachments param; modify `postChatwootMessage` signature to accept `[]Attachment`; modify `processOutbound` to loop attachments |
| `internal/bridge/bridge_test.go` | Add 8 unit tests (parser per type + dispatcher) |
| `internal/bridge/handlers_integration_test.go` | Add 2 integration tests (image roundtrip, document roundtrip) |

---

## Task 1: Add `Attachment` type + `waAttachment()` parser (image)

**Files:**
- Modify: `internal/bridge/bridge.go` (add type + fn near `waContactJID`)
- Test: `internal/bridge/bridge_test.go`

- [ ] **Step 1: Write failing test for image extraction**

Add to `internal/bridge/bridge_test.go`:

```go
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
```

- [ ] **Step 2: Run test — must fail**

Run: `go test ./internal/bridge -run TestWaAttachment_ImageMessage -v`
Expected: FAIL — `undefined: waAttachment` and `Attachment` type.

- [ ] **Step 3: Implement `Attachment` type and image case**

Add to `internal/bridge/bridge.go` (place near `waContactJID`):

```go
type Attachment struct {
	URL      string
	MimeType string
	Caption  string
	FileName string
	Kind     string // "image" | "audio" | "video" | "document" | "sticker"
}
```

Extend `waPayload.Message` struct to include image fields. Replace the existing `Message` struct in `waPayload` with:

```go
Message struct {
	Conversation string `json:"conversation"`
	Extended     struct {
		Text string `json:"text"`
	} `json:"extendedTextMessage"`
	Image struct {
		URL      string `json:"url"`
		MimeType string `json:"mimetype"`
		Caption  string `json:"caption"`
	} `json:"imageMessage"`
} `json:"message"`
```

Add function:

```go
func waAttachment(p waPayload) (Attachment, bool) {
	if p.Message.Image.URL != "" {
		return Attachment{
			URL: p.Message.Image.URL, MimeType: p.Message.Image.MimeType,
			Caption: p.Message.Image.Caption, Kind: "image",
		}, true
	}
	return Attachment{}, false
}
```

- [ ] **Step 4: Run test — must pass**

Run: `go test ./internal/bridge -run TestWaAttachment_ImageMessage -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bridge/bridge.go internal/bridge/bridge_test.go
git commit -m "feat(media): add Attachment type + waAttachment image case"
```

---

## Task 2: Add audio + sticker cases to `waAttachment()`

**Files:**
- Modify: `internal/bridge/bridge.go` (extend `waPayload.Message`; extend `waAttachment`)
- Test: `internal/bridge/bridge_test.go`

- [ ] **Step 1: Write failing test for audio (PTT) and sticker**

Add to `internal/bridge/bridge_test.go`:

```go
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
	if !ok || att.URL != "https://media.example/sticker.webp" || att.Kind != "image" {
		t.Fatalf("sticker: %+v ok=%v", att, ok)
	}
}
```

- [ ] **Step 2: Run tests — must fail**

Run: `go test ./internal/bridge -run "TestWaAttachment_AudioMessage|TestWaAttachment_StickerMessage" -v`
Expected: FAIL.

- [ ] **Step 3: Add audio + sticker fields and cases**

Extend `waPayload.Message` adding fields after `Image`:

```go
Audio struct {
	URL      string `json:"url"`
	MimeType string `json:"mimetype"`
	PTT      bool   `json:"ptt"`
} `json:"audioMessage"`
Sticker struct {
	URL      string `json:"url"`
	MimeType string `json:"mimetype"`
} `json:"stickerMessage"`
```

Extend `waAttachment` with:

```go
if p.Message.Audio.URL != "" {
	return Attachment{
		URL: p.Message.Audio.URL, MimeType: p.Message.Audio.MimeType, Kind: "audio",
	}, true
}
if p.Message.Sticker.URL != "" {
	return Attachment{
		URL: p.Message.Sticker.URL, MimeType: p.Message.Sticker.MimeType, Kind: "image",
	}, true
}
```

(Sticker maps to `image` — Chatwoot has no sticker UI; megaAPI sendMessage/mediaUrl has no sticker mode either.)

- [ ] **Step 4: Run tests — must pass**

Run: `go test ./internal/bridge -run TestWaAttachment_ -v`
Expected: 3 PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bridge/bridge.go internal/bridge/bridge_test.go
git commit -m "feat(media): add audio + sticker waAttachment cases"
```

---

## Task 3: Add video + document cases to `waAttachment()`

**Files:**
- Modify: `internal/bridge/bridge.go`
- Test: `internal/bridge/bridge_test.go`

- [ ] **Step 1: Write failing tests for video and document**

Add to `internal/bridge/bridge_test.go`:

```go
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
```

- [ ] **Step 2: Run — must fail**

Run: `go test ./internal/bridge -run TestWaAttachment_ -v`
Expected: 2 new FAIL, others PASS.

- [ ] **Step 3: Add video + document fields and cases**

Extend `waPayload.Message` adding:

```go
Video struct {
	URL      string `json:"url"`
	MimeType string `json:"mimetype"`
	Caption  string `json:"caption"`
} `json:"videoMessage"`
Document struct {
	URL      string `json:"url"`
	MimeType string `json:"mimetype"`
	FileName string `json:"fileName"`
	Caption  string `json:"caption"`
} `json:"documentMessage"`
```

Extend `waAttachment` (place before the final `return` line):

```go
if p.Message.Video.URL != "" {
	return Attachment{
		URL: p.Message.Video.URL, MimeType: p.Message.Video.MimeType,
		Caption: p.Message.Video.Caption, Kind: "video",
	}, true
}
if p.Message.Document.URL != "" {
	return Attachment{
		URL: p.Message.Document.URL, MimeType: p.Message.Document.MimeType,
		Caption: p.Message.Document.Caption, FileName: p.Message.Document.FileName, Kind: "document",
	}, true
}
```

- [ ] **Step 4: Run — must pass**

Run: `go test ./internal/bridge -run TestWaAttachment_ -v`
Expected: 5 PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bridge/bridge.go internal/bridge/bridge_test.go
git commit -m "feat(media): add video + document waAttachment cases"
```

---

## Task 4: Modify `postChatwootMessage` to accept attachments

**Files:**
- Modify: `internal/bridge/bridge.go` (`postChatwootMessage` signature + body)
- Test: `internal/bridge/bridge_test.go`

- [ ] **Step 1: Write failing test for attachment in CW request body**

Add to `internal/bridge/bridge_test.go`:

```go
func TestPostChatwootMessage_WithAttachment_IncludesFileURL(t *testing.T) {
	var captured map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer mock.Close()
	key := bytes.Repeat([]byte{1}, 32)
	tokEnc, _ := Encrypt([]byte("tok"), key)
	t1 := Tenant{
		ID: uuid.New(), ChatwootURL: mock.URL,
		ChatwootTokenEnc: tokEnc, ChatwootAccountID: 1, ChatwootInboxID: 5,
	}
	s := &Server{Key: key}
	att := []Attachment{{URL: "https://media.example/img.jpg", Kind: "image", Caption: "hi"}}
	if err := s.postChatwootMessage(context.Background(), t1, 42, "hi", "WAID-1", att); err != nil {
		t.Fatalf("postChatwootMessage: %v", err)
	}
	atts, _ := captured["attachments"].([]any)
	if len(atts) != 1 {
		t.Fatalf("expected 1 attachment in body, got %d", len(atts))
	}
	first, _ := atts[0].(map[string]any)
	if first["file_url"] != "https://media.example/img.jpg" {
		t.Errorf("file_url: got %v", first["file_url"])
	}
	if first["file_type"] != "image" {
		t.Errorf("file_type: got %v", first["file_type"])
	}
}
```

(Add imports `bytes`, `context`, `encoding/json`, `net/http`, `net/http/httptest`, `github.com/google/uuid` if missing.)

- [ ] **Step 2: Run — must fail**

Run: `go test ./internal/bridge -run TestPostChatwootMessage_WithAttachment -v`
Expected: FAIL — signature mismatch (`postChatwootMessage` takes 5 args, test passes 6).

- [ ] **Step 3: Modify `postChatwootMessage` signature**

Replace existing function with:

```go
func (s *Server) postChatwootMessage(ctx context.Context, t Tenant, convID int64, content, externalID string, attachments []Attachment) error {
	body := map[string]any{
		"content":            content,
		"message_type":       "incoming",
		"content_attributes": map[string]any{"external_id": externalID},
	}
	if len(attachments) > 0 {
		out := make([]map[string]any, 0, len(attachments))
		for _, a := range attachments {
			out = append(out, map[string]any{"file_url": a.URL, "file_type": a.Kind})
		}
		body["attachments"] = out
	}
	url := fmt.Sprintf("%s/api/v1/accounts/%d/conversations/%d/messages",
		strings.TrimRight(t.ChatwootURL, "/"), t.ChatwootAccountID, convID)
	return s.cwDo(ctx, t, http.MethodPost, url, body, nil)
}
```

- [ ] **Step 4: Update existing caller**

In `processInbound`, change last line:

From:
```go
return s.postChatwootMessage(ctx, tenant, convID, waText(p), p.Key.ID)
```

To:
```go
var atts []Attachment
if a, ok := waAttachment(p); ok {
	atts = []Attachment{a}
}
content := waText(p)
if content == "" && len(atts) > 0 {
	content = atts[0].Caption
}
return s.postChatwootMessage(ctx, tenant, convID, content, p.Key.ID, atts)
```

- [ ] **Step 5: Run all package tests — must pass (no regression)**

Run: `go test ./internal/bridge -v`
Expected: all PASS (existing tests still call the new signature via `processInbound`; new test passes).

- [ ] **Step 6: Commit**

```bash
git add internal/bridge/bridge.go internal/bridge/bridge_test.go
git commit -m "feat(media): postChatwootMessage accepts attachments"
```

---

## Task 5: Add `cwPayload.Attachments` + `cwAttachments()` extractor

**Files:**
- Modify: `internal/bridge/bridge.go` (extend `cwPayload`; add helper)
- Test: `internal/bridge/bridge_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/bridge/bridge_test.go`:

```go
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
```

- [ ] **Step 2: Run — must fail**

Run: `go test ./internal/bridge -run TestCwAttachments_Extracts -v`
Expected: FAIL.

- [ ] **Step 3: Extend `cwPayload` with attachments**

Modify `cwPayload` struct adding field after `Sender`:

```go
Attachments []struct {
	FileType string `json:"file_type"`
	DataURL  string `json:"data_url"`
} `json:"attachments"`
```

Add helper after `parseCW`:

```go
func cwAttachments(p cwPayload) []Attachment {
	if len(p.Attachments) == 0 {
		return nil
	}
	out := make([]Attachment, 0, len(p.Attachments))
	for _, a := range p.Attachments {
		if a.DataURL == "" {
			continue
		}
		out = append(out, Attachment{URL: a.DataURL, Kind: cwTypeToMega(a.FileType)})
	}
	return out
}

func cwTypeToMega(ft string) string {
	switch ft {
	case "image":
		return "image"
	case "audio":
		return "audio"
	case "video":
		return "video"
	default:
		return "document"
	}
}
```

- [ ] **Step 4: Run — must pass**

Run: `go test ./internal/bridge -run TestCwAttachments_Extracts -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bridge/bridge.go internal/bridge/bridge_test.go
git commit -m "feat(media): cwAttachments extractor + file_type → mega type mapping"
```

---

## Task 6: Add `sendMegaAPIMedia()`

**Files:**
- Modify: `internal/bridge/bridge.go`
- Test: `internal/bridge/bridge_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/bridge/bridge_test.go`:

```go
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
	if md["mediaUrl"] != "https://m.example/x.jpg" {
		t.Errorf("mediaUrl: got %v", md["mediaUrl"])
	}
	if md["type"] != "image" {
		t.Errorf("type: got %v", md["type"])
	}
	if md["caption"] != "hi" {
		t.Errorf("caption: got %v", md["caption"])
	}
}
```

- [ ] **Step 2: Run — must fail**

Run: `go test ./internal/bridge -run TestSendMegaAPIMedia -v`
Expected: FAIL — `undefined: sendMegaAPIMedia`.

- [ ] **Step 3: Implement `sendMegaAPIMedia`**

Add after `sendMegaAPIText`:

```go
func (s *Server) sendMegaAPIMedia(ctx context.Context, t Tenant, to string, att Attachment) error {
	tok, err := Decrypt(t.MegaAPITokenEnc, s.Key)
	if err != nil {
		return notRetriable(err)
	}
	md := map[string]any{
		"to":       to,
		"mediaUrl": att.URL,
		"type":     att.Kind,
		"caption":  att.Caption,
	}
	if att.FileName != "" {
		md["fileName"] = att.FileName
	}
	if att.MimeType != "" {
		md["mimetype"] = att.MimeType
	}
	body := map[string]any{"messageData": md}
	url := fmt.Sprintf("%s/rest/sendMessage/%s/mediaUrl",
		strings.TrimRight(t.MegaAPIHost, "/"), t.MegaAPIInstance)
	resp, err := bearerPost(ctx, url, string(tok), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return classifyHTTP(resp, "megaapi")
}
```

- [ ] **Step 4: Run — must pass**

Run: `go test ./internal/bridge -run TestSendMegaAPIMedia -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bridge/bridge.go internal/bridge/bridge_test.go
git commit -m "feat(media): sendMegaAPIMedia posts to /mediaUrl endpoint"
```

---

## Task 7: Wire outbound dispatcher (`processOutbound` loops attachments)

**Files:**
- Modify: `internal/bridge/bridge.go` (`processOutbound`)
- Test: `internal/bridge/handlers_integration_test.go` (uses real testcontainer PG via `setupDB(t)` from `storage_test.go` since `processOutbound` calls `s.tenantByID` which queries the pool directly)

- [ ] **Step 1: Write failing integration test (3 attachments + text → caption on first only)**

Add to `internal/bridge/handlers_integration_test.go`:

```go
func TestProcessOutbound_MultipleAttachments_CaptionOnlyOnFirst(t *testing.T) {
	db := setupDB(t)
	captions := []string{}
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		md, _ := body["messageData"].(map[string]any)
		captions = append(captions, fmt.Sprint(md["caption"]))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer mock.Close()
	key := bytes.Repeat([]byte{1}, 32)
	tokEnc, _ := Encrypt([]byte("tok"), key)
	tID, err := db.InsertTenant(context.Background(), TenantInsert{
		Slug: "demo-out", MegaAPIHost: mock.URL, MegaAPIInstance: "abc",
		MegaAPITokenEnc: tokEnc, ChatwootURL: "http://x", ChatwootTokenEnc: tokEnc,
		ChatwootAccountID: 1, ChatwootInboxID: 5,
		HMACSecretEnc: tokEnc, WebhookBearerEnc: tokEnc,
	})
	if err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	s := &Server{Key: key, DB: db}
	body := []byte(`{
		"event":"message_created","message_type":"outgoing","private":false,"id":1,
		"content":"hello",
		"conversation":{"id":1,"contact_inbox":{"source_id":"5511999999999"}},
		"attachments":[
			{"file_type":"image","data_url":"https://m/1.jpg"},
			{"file_type":"image","data_url":"https://m/2.jpg"},
			{"file_type":"image","data_url":"https://m/3.jpg"}
		]
	}`)
	if err := s.processOutbound(context.Background(), Job{TenantID: tID, Payload: body}); err != nil {
		t.Fatalf("processOutbound: %v", err)
	}
	if len(captions) != 3 || captions[0] != "hello" || captions[1] != "" || captions[2] != "" {
		t.Fatalf("captions: %#v", captions)
	}
}
```

- [ ] **Step 2: Run — must fail (compile error or wrong path)**

Run (Docker required): `go test -tags=integration ./internal/bridge -run TestProcessOutbound_MultipleAttachments -v`
Expected: FAIL — current `processOutbound` calls only `sendMegaAPIText`, never `sendMegaAPIMedia`, so no calls to `/mediaUrl`.

- [ ] **Step 3: Modify `processOutbound`**

Replace existing `processOutbound` with:

```go
func (s *Server) processOutbound(ctx context.Context, job Job) error {
	tenant, err := s.tenantByID(ctx, job.TenantID)
	if err != nil {
		return err
	}
	p, err := parseCW(job.Payload)
	if err != nil {
		return notRetriable(err)
	}
	jid := p.Conversation.ContactInbox.SourceID
	if jid == "" {
		jid = p.Sender.PhoneNumber
	}
	atts := cwAttachments(p)
	if jid == "" || (p.Content == "" && len(atts) == 0) {
		return notRetriable(errors.New("missing recipient or content"))
	}
	if len(atts) == 0 {
		return s.sendMegaAPIText(ctx, tenant, jid, p.Content)
	}
	for i, a := range atts {
		caption := ""
		if i == 0 {
			caption = p.Content
		}
		a.Caption = caption
		if err := s.sendMegaAPIMedia(ctx, tenant, jid, a); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run — must pass**

Run: `go test -tags=integration ./internal/bridge -run TestProcessOutbound_MultipleAttachments -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bridge/bridge.go internal/bridge/handlers_integration_test.go
git commit -m "feat(media): processOutbound loops attachments, caption on first only"
```

---

## Task 8: Inbound integration test (image roundtrip)

**Files:**
- Test only: `internal/bridge/handlers_integration_test.go`

- [ ] **Step 1: Write failing test**

Add:

```go
func TestProcessInbound_ImageMessage_PostsAttachmentToCW(t *testing.T) {
	db := setupDB(t)
	var capturedBody map[string]any
	cwMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Three CW endpoints get hit in sequence: contacts, conversations, messages.
		// We only capture the messages body.
		if strings.Contains(r.URL.Path, "/messages") {
			_ = json.NewDecoder(r.Body).Decode(&capturedBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		if strings.Contains(r.URL.Path, "/contacts") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"payload":{"contact":{"id":11}}}`))
			return
		}
		if strings.Contains(r.URL.Path, "/conversations") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":99}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer cwMock.Close()
	key := bytes.Repeat([]byte{1}, 32)
	tokEnc, _ := Encrypt([]byte("tok"), key)
	tID, err := db.InsertTenant(context.Background(), TenantInsert{
		Slug: "demo-in", MegaAPIHost: "http://x", MegaAPIInstance: "abc",
		MegaAPITokenEnc: tokEnc, ChatwootURL: cwMock.URL, ChatwootTokenEnc: tokEnc,
		ChatwootAccountID: 1, ChatwootInboxID: 5,
		HMACSecretEnc: tokEnc, WebhookBearerEnc: tokEnc,
	})
	if err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	s := &Server{Key: key, DB: db}
	body := []byte(`{
		"key":{"id":"WAID-IMG","remoteJid":"5511999999999@s.whatsapp.net","fromMe":false},
		"pushName":"Alice",
		"message":{"imageMessage":{"url":"https://media.example/img.jpg","mimetype":"image/jpeg","caption":"hello"}}
	}`)
	if err := s.processInbound(context.Background(), Job{TenantID: tID, Payload: body}); err != nil {
		t.Fatalf("processInbound: %v", err)
	}
	atts, _ := capturedBody["attachments"].([]any)
	if len(atts) != 1 {
		t.Fatalf("expected 1 attachment; capturedBody=%v", capturedBody)
	}
	first := atts[0].(map[string]any)
	if first["file_url"] != "https://media.example/img.jpg" {
		t.Errorf("file_url: %v", first["file_url"])
	}
	if first["file_type"] != "image" {
		t.Errorf("file_type: %v", first["file_type"])
	}
	if capturedBody["content"] != "hello" {
		t.Errorf("content: %v", capturedBody["content"])
	}
}
```

- [ ] **Step 2: Run — must pass (Tasks 1-4 already covered the impl)**

Run (Docker required): `go test -tags=integration ./internal/bridge -run TestProcessInbound_ImageMessage -v`
Expected: PASS.

If FAIL: fix `processInbound` content-fallback logic — when text empty and attachment has caption, content must equal caption. The Task 4 patch already handles this; verify.

- [ ] **Step 3: Commit**

```bash
git add internal/bridge/handlers_integration_test.go
git commit -m "test(media): inbound image roundtrip integration test"
```

---

## Task 9: Document inbound integration test (PDF roundtrip)

**Files:**
- Test only: `internal/bridge/handlers_integration_test.go`

- [ ] **Step 1: Write failing test (asserts fileName + content=caption)**

Add:

```go
func TestProcessInbound_DocumentMessage_PostsFileNameAndCaption(t *testing.T) {
	db := setupDB(t)
	var capturedBody map[string]any
	cwMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/messages"):
			_ = json.NewDecoder(r.Body).Decode(&capturedBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case strings.Contains(r.URL.Path, "/contacts"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"payload":{"contact":{"id":12}}}`))
		case strings.Contains(r.URL.Path, "/conversations"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":100}`))
		}
	}))
	defer cwMock.Close()
	key := bytes.Repeat([]byte{1}, 32)
	tokEnc, _ := Encrypt([]byte("tok"), key)
	tID, _ := db.InsertTenant(context.Background(), TenantInsert{
		Slug: "demo-doc", MegaAPIHost: "http://x", MegaAPIInstance: "abc",
		MegaAPITokenEnc: tokEnc, ChatwootURL: cwMock.URL, ChatwootTokenEnc: tokEnc,
		ChatwootAccountID: 1, ChatwootInboxID: 5,
		HMACSecretEnc: tokEnc, WebhookBearerEnc: tokEnc,
	})
	s := &Server{Key: key, DB: db}
	body := []byte(`{
		"key":{"id":"WAID-DOC","remoteJid":"5511999999999@s.whatsapp.net","fromMe":false},
		"pushName":"Alice",
		"message":{"documentMessage":{"url":"https://media.example/c.pdf","mimetype":"application/pdf","fileName":"contract.pdf","caption":"sign please"}}
	}`)
	if err := s.processInbound(context.Background(), Job{TenantID: tID, Payload: body}); err != nil {
		t.Fatalf("processInbound: %v", err)
	}
	if capturedBody["content"] != "sign please" {
		t.Errorf("content: %v", capturedBody["content"])
	}
	atts, _ := capturedBody["attachments"].([]any)
	first := atts[0].(map[string]any)
	if first["file_type"] != "document" {
		t.Errorf("file_type: %v", first["file_type"])
	}
}
```

- [ ] **Step 2: Run — must pass**

Run: `go test -tags=integration ./internal/bridge -run TestProcessInbound_DocumentMessage -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/bridge/handlers_integration_test.go
git commit -m "test(media): inbound document roundtrip integration test"
```

---

## Task 10: Final regression check + push

- [ ] **Step 1: Run full suite**

Run: `go test ./...`
Expected: all PASS.

Run: `go test -tags=integration ./...` (Docker required)
Expected: all PASS.

Run: `go vet ./...`
Expected: clean.

Run: `go build ./...`
Expected: clean.

- [ ] **Step 2: LOC budget verification**

Run:
```bash
git diff --stat archon/task-feat-reset-mvp -- internal/bridge/bridge.go internal/bridge/bridge_test.go internal/bridge/handlers_integration_test.go
```
Expected: production diff ≤200 lines added in `bridge.go`; tests ≤300 lines combined.

- [ ] **Step 3: Push branch + open PR**

Run:
```bash
git push -u origin feat/media
gh pr create --base master --head feat/media \
  --title "feat: media bidirectional (image/audio/video/document/sticker)" \
  --body "Implements .agents/plans not used; see docs/plans/2026-05-06-media-bidirectional.md. 5 inbound types, outbound loop with caption-on-first. Pass-through URL only. ~12 new tests."
```
Expected: PR URL printed.

---

## Out of Scope (do NOT touch in this plan)

| Item | Reason |
|------|--------|
| Quick wins QW1-6 from PR #2 review | Ritual; address in separate cleanup PR |
| Worth-doing WD1-5 from PR #2 review | Separate concern (graceful shutdown, drain backlog, etc.) |
| Follow-ups FU1-FU10 (CI workflow, mutation testing, refactors, ADRs) | Backlog |
| Stream proxy / download-then-upload media bytes | YAGNI — pass-through URL works |
| Re-host expired megaAPI URLs | YAGNI — accept signed URL TTL |
| Reactions, location, contact (vCard) | Out of media scope; megaAPI/CW support inconsistent |
| Prometheus metrics, observability extras | Out of media scope |
| Admin UI, DLQ endpoint | Out of MVP |
| Cache, rate limit, backpressure tuning | Defer until profiled |
| Refactor `*repo.Queries` interface | Already filed as F2 issues in `bd` |
| Comments, magic-number polishing, RFC purism | Cosmetic |

If a step in this plan would touch any of the above → STOP and ask user.

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| megaAPI URL expires before CW agent opens it | Accept; pass-through URL design. Document caveat in commit message. |
| CW returns new `file_type` not in mapping | Default to `document`. `cwTypeToMega` handles via `default:` branch. |
| Mimetype missing in inbound payload | Pass empty string to CW. CW uses URL extension. Acceptable. |
| Multiple attachments + retry → resend duplicates | Caller already idempotent via `messages.UNIQUE(tenant_id, direction, external_id)`. Job retry replays whole job; megaAPI receives N posts; on success all OK. On partial failure, retry sends N more — accept; very rare with pass-through. |
| Sticker WEBP not displayable in some CW versions | CW handles WEBP since v3.x. Accept. |
| Test 7-9 require Docker (testcontainers) | Document in plan header; CI must have Docker. Manual run requires Docker Desktop running. |

---

## Acceptance Criteria

- [ ] All 53 existing tests still pass (no regression).
- [ ] 6 new unit tests pass (Tasks 1-6).
- [ ] 3 new integration tests pass (Tasks 7-9).
- [ ] `go vet` clean.
- [ ] `go build` clean.
- [ ] Production LOC delta ≤200 in `bridge.go`.
- [ ] Test LOC delta ≤300 across `bridge_test.go` + `handlers_integration_test.go`.
- [ ] No new Go file created.
- [ ] No new dependency in `go.mod` (`go mod tidy` produces no diff).
- [ ] Live smoke (manual, post-merge): WhatsApp image/audio/video/document → CW shows attachment; CW image/file → WhatsApp delivers.

---

## Branch Strategy

Branch from `archon/task-feat-reset-mvp` (current PR #2 branch) so this stacks on the not-yet-merged base, OR from `master` after PR #2 merges.

**Recommended:** Wait for PR #2 merge, then branch `feat/media` from `master`. Avoids stacking PRs.
