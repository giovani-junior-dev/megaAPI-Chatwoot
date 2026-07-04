package bridge

import "context"

// megaProvider adapts the existing megaAPI code to the Provider seam. It only
// wraps functions that already existed — behaviour is unchanged.
type megaProvider struct{ s *Server }

func (m megaProvider) SendText(ctx context.Context, t Tenant, to, text string) error {
	return m.s.sendMegaAPIText(ctx, t, to, text)
}

func (m megaProvider) SendMedia(ctx context.Context, t Tenant, to string, a Attachment) error {
	prepared, err := m.s.prepareMedia(ctx, a)
	if err != nil {
		return err
	}
	return m.s.sendMegaAPIMedia(ctx, t, to, prepared)
}

func (m megaProvider) ParseInbound(body []byte) (InboundMessage, error) {
	p, err := parseWA(body)
	if err != nil {
		return InboundMessage{}, err
	}
	var att *Attachment
	if a, ok := waAttachment(p); ok {
		att = &a
	}
	return InboundMessage{
		ContactID:  waContactJID(p),
		Name:       p.PushName,
		Text:       waText(p),
		Attachment: att,
		ExternalID: p.Key.ID,
	}, nil
}

func (m megaProvider) DownloadMedia(ctx context.Context, t Tenant, a Attachment) ([]byte, string, error) {
	if a.MediaKey != "" {
		return m.s.downloadMegaAPIMedia(ctx, t, a)
	}
	return downloadPublicURL(ctx, a)
}
