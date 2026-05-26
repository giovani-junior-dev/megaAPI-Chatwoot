package bridge

import "testing"

func TestExtractWAConnectionUpdate_PhoneConnected(t *testing.T) {
	body := []byte(`{"messageType":"connection_update","message":"phone_connected","jid":"5511999@s.whatsapp.net","pushName":"Bob"}`)
	jid, ok := extractWAConnectionUpdate(body)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if jid != "5511999@s.whatsapp.net" {
		t.Fatalf("jid=%q", jid)
	}
}

func TestExtractWAConnectionUpdate_OtherEvent(t *testing.T) {
	body := []byte(`{"messageType":"chat","key":{"id":"ABC"}}`)
	if _, ok := extractWAConnectionUpdate(body); ok {
		t.Fatal("expected ok=false")
	}
}

func TestExtractWAConnectionUpdate_DifferentMessage(t *testing.T) {
	body := []byte(`{"messageType":"connection_update","message":"phone_disconnected","jid":"5511@s"}`)
	if _, ok := extractWAConnectionUpdate(body); ok {
		t.Fatal("expected ok=false for non-phone_connected")
	}
}
