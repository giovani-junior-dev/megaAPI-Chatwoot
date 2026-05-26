package bridge

import "encoding/json"

type connectionUpdate struct {
	MessageType string `json:"messageType"`
	Message     string `json:"message"`
	JID         string `json:"jid"`
	PushName    string `json:"pushName"`
}

func extractWAConnectionUpdate(body []byte) (string, bool) {
	var p connectionUpdate
	if err := json.Unmarshal(body, &p); err != nil {
		return "", false
	}
	if p.MessageType != "connection_update" || p.Message != "phone_connected" {
		return "", false
	}
	return p.JID, true
}
