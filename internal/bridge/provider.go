package bridge

import (
	"context"
	"errors"
)

// InboundMessage is the provider-neutral shape of one inbound WhatsApp message.
// Both megaAPI and WaBlast parse their gateway-specific webhook into this so the
// Chatwoot side (resolveContact/postChatwootMessage) stays provider-agnostic.
type InboundMessage struct {
	ContactID  string // bare digits, no '+' (matches contacts.wa_jid + Chatwoot identifier)
	Name       string
	Text       string
	Attachment *Attachment // nil when text-only
	ExternalID string      // messages.external_id (idempotency)
}

// Provider is the gateway seam. One concrete impl per WhatsApp gateway. A tenant
// binds to exactly one via tenants.provider.
type Provider interface {
	SendText(ctx context.Context, t Tenant, to, text string) error
	SendMedia(ctx context.Context, t Tenant, to string, a Attachment) error
	ParseInbound(body []byte) (InboundMessage, error)
	DownloadMedia(ctx context.Context, t Tenant, a Attachment) ([]byte, string, error)
}

// errWindowClosed signals the WhatsApp 24h free-form window is shut; the caller
// posts a private note to Chatwoot and stops retrying (a template is required).
var errWindowClosed = errors.New("wablast: 24h window closed")

func (s *Server) providerFor(t Tenant) Provider {
	if t.Provider == providerWablast {
		return wablastProvider{s: s}
	}
	return megaProvider{s: s}
}
