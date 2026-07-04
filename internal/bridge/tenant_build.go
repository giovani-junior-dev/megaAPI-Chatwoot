package bridge

import "encoding/base64"

type TenantSpec struct {
	Slug              string
	Provider          string
	MegaAPIHost       string
	MegaAPIInstance   string
	MegaAPIToken      string
	WablastAPIKey     string
	WablastAccountID  string
	ChatwootURL       string
	ChatwootToken     string
	ChatwootAccountID int64
	ChatwootInboxID   int64
}

func BuildTenantInsert(key []byte, s TenantSpec) (string, string, TenantInsert, error) {
	bearer := base64.RawURLEncoding.EncodeToString(RandomBytes(32))
	hmacSecret := base64.RawURLEncoding.EncodeToString(RandomBytes(32))
	enc := func(v string) ([]byte, error) { return Encrypt([]byte(v), key) }
	encCW, err := enc(s.ChatwootToken)
	if err != nil {
		return "", "", TenantInsert{}, err
	}
	encBearer, err := enc(bearer)
	if err != nil {
		return "", "", TenantInsert{}, err
	}
	encHMAC, err := enc(hmacSecret)
	if err != nil {
		return "", "", TenantInsert{}, err
	}
	provider := s.Provider
	if provider == "" {
		provider = providerMega
	}
	ti := TenantInsert{
		Slug:              s.Slug,
		Provider:          provider,
		ChatwootURL:       s.ChatwootURL,
		ChatwootTokenEnc:  encCW,
		ChatwootAccountID: s.ChatwootAccountID,
		ChatwootInboxID:   s.ChatwootInboxID,
		HMACSecretEnc:     encHMAC,
		WebhookBearerEnc:  encBearer,
	}
	if provider == providerWablast {
		encKey, kerr := enc(s.WablastAPIKey)
		if kerr != nil {
			return "", "", TenantInsert{}, kerr
		}
		ti.WablastAPIKeyEnc = encKey
		ti.WablastAccountID = s.WablastAccountID
		return bearer, hmacSecret, ti, nil
	}
	encMega, merr := enc(s.MegaAPIToken)
	if merr != nil {
		return "", "", TenantInsert{}, merr
	}
	ti.MegaAPIHost = s.MegaAPIHost
	ti.MegaAPIInstance = s.MegaAPIInstance
	ti.MegaAPITokenEnc = encMega
	return bearer, hmacSecret, ti, nil
}
