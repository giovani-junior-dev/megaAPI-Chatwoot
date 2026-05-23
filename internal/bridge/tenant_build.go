package bridge

import "encoding/base64"

type TenantSpec struct {
	Slug              string
	MegaAPIHost       string
	MegaAPIInstance   string
	MegaAPIToken      string
	ChatwootURL       string
	ChatwootToken     string
	ChatwootAccountID int64
	ChatwootInboxID   int64
}

func BuildTenantInsert(key []byte, s TenantSpec) (string, string, TenantInsert, error) {
	bearer := base64.RawURLEncoding.EncodeToString(RandomBytes(32))
	hmacSecret := base64.RawURLEncoding.EncodeToString(RandomBytes(32))
	encMega, err := Encrypt([]byte(s.MegaAPIToken), key)
	if err != nil {
		return "", "", TenantInsert{}, err
	}
	encCW, err := Encrypt([]byte(s.ChatwootToken), key)
	if err != nil {
		return "", "", TenantInsert{}, err
	}
	encBearer, err := Encrypt([]byte(bearer), key)
	if err != nil {
		return "", "", TenantInsert{}, err
	}
	encHMAC, err := Encrypt([]byte(hmacSecret), key)
	if err != nil {
		return "", "", TenantInsert{}, err
	}
	return bearer, hmacSecret, TenantInsert{
		Slug:              s.Slug,
		MegaAPIHost:       s.MegaAPIHost,
		MegaAPIInstance:   s.MegaAPIInstance,
		MegaAPITokenEnc:   encMega,
		ChatwootURL:       s.ChatwootURL,
		ChatwootTokenEnc:  encCW,
		ChatwootAccountID: s.ChatwootAccountID,
		ChatwootInboxID:   s.ChatwootInboxID,
		HMACSecretEnc:     encHMAC,
		WebhookBearerEnc:  encBearer,
	}, nil
}
