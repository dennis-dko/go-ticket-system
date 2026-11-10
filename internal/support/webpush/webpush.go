package webpush

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Keys is an installation's VAPID key pair. The private half signs every message
// the installation sends; browsers are given the public half when they subscribe,
// and their push service then accepts messages for that subscription only from
// whoever holds the private one.
type Keys struct {
	private *ecdsa.PrivateKey
}

// GenerateKeys makes a new pair.
func GenerateKeys() (Keys, error) {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Keys{}, fmt.Errorf("generating the push keys: %w", err)
	}

	return Keys{private: private}, nil
}

// ParseKeys reads a pair back from its private half, as Private writes it.
func ParseKeys(private string) (Keys, error) {
	raw, err := base64.RawURLEncoding.DecodeString(private)
	if err != nil {
		return Keys{}, fmt.Errorf("reading the push key: %w", err)
	}

	key, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), raw)
	if err != nil {
		return Keys{}, fmt.Errorf("reading the push key: %w", err)
	}

	return Keys{private: key}, nil
}

// Private is the private half, base64url, for keeping sealed.
func (k Keys) Private() (string, error) {
	raw, err := k.private.Bytes()
	if err != nil {
		return "", fmt.Errorf("writing the push key: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Public is the public half as a browser takes it to subscribe with: the
// uncompressed point, base64url.
func (k Keys) Public() (string, error) {
	raw, err := k.private.PublicKey.Bytes()
	if err != nil {
		return "", fmt.Errorf("writing the push key: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Subscription is one browser's push channel as the browser hands it over: the
// push service's address to post to, and the two keys a message is encrypted
// for - P256dh, the browser's public key, and Auth, a secret it shares with the
// installation alone. Origin is the address the browser reached the
// installation at, which signs each message as whom the push service may contact.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
	Origin   string
}

// Message is what one push says: a notification's title and words, and the tag
// that lets a newer notification take an older one's place.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Tag   string `json:"tag"`
}

// ErrGone is a push service saying a subscription no longer exists - the browser
// unsubscribed, or was reset - so it is to be forgotten rather than tried again.
var ErrGone = errors.New("the push service no longer knows this subscription")

// recordSize is the size of the one record every message is: RFC 8188's
// default, and far more than a reminder needs.
const recordSize = 4096

// Send posts message to the push service of sub, encrypted for sub and signed
// with k. ttl is how long the push service may keep it for a browser that is not
// online; a reminder kept longer than its moment is no use to anybody.
func (k Keys) Send(ctx context.Context, client *http.Client, sub Subscription, message Message, ttl time.Duration) error {
	plain, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("writing the push message: %w", err)
	}

	body, err := Encrypt(plain, sub)
	if err != nil {
		return err
	}

	signed, err := k.authorization(sub.Endpoint, sub.Origin, time.Now())
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("addressing the push service: %w", err)
	}

	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", strconv.Itoa(int(ttl.Seconds())))
	req.Header.Set("Urgency", "normal")
	req.Header.Set("Authorization", signed)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("reaching the push service: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	// Read so the connection can be used again; what it says is not needed.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrGone
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return fmt.Errorf("the push service answered %d", resp.StatusCode)
	}

	return nil
}

// Encrypt is the body of a message saying plain to sub, encrypted as RFC 8291
// says with a key pair and a salt made for this message alone.
func Encrypt(plain []byte, sub Subscription) ([]byte, error) {
	sender, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("making the message's key: %w", err)
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("making the message's salt: %w", err)
	}

	return encrypt(plain, sub, sender, salt)
}

// encrypt is Encrypt with the sender's key pair and the salt given, which is what
// lets it be held to the RFC's example.
func encrypt(plain []byte, sub Subscription, sender *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	receiverKey, err := base64.RawURLEncoding.DecodeString(sub.P256dh)
	if err != nil {
		return nil, fmt.Errorf("reading the browser's key: %w", err)
	}

	receiver, err := ecdh.P256().NewPublicKey(receiverKey)
	if err != nil {
		return nil, fmt.Errorf("reading the browser's key: %w", err)
	}

	secret, err := base64.RawURLEncoding.DecodeString(sub.Auth)
	if err != nil {
		return nil, fmt.Errorf("reading the browser's secret: %w", err)
	}

	shared, err := sender.ECDH(receiver)
	if err != nil {
		return nil, fmt.Errorf("agreeing a key with the browser: %w", err)
	}

	senderKey := sender.PublicKey().Bytes()

	// RFC 8291, section 3.4: the shared secret and the browser's own secret make
	// the keying material, which with the salt makes the key and the nonce.
	combining, err := hkdf.Extract(sha256.New, shared, secret)
	if err != nil {
		return nil, fmt.Errorf("deriving the message's key: %w", err)
	}

	material, err := hkdf.Expand(sha256.New, combining, "WebPush: info\x00"+string(receiverKey)+string(senderKey), 32)
	if err != nil {
		return nil, fmt.Errorf("deriving the message's key: %w", err)
	}

	content, err := hkdf.Extract(sha256.New, material, salt)
	if err != nil {
		return nil, fmt.Errorf("deriving the message's key: %w", err)
	}

	key, err := hkdf.Expand(sha256.New, content, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, fmt.Errorf("deriving the message's key: %w", err)
	}

	nonce, err := hkdf.Expand(sha256.New, content, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, fmt.Errorf("deriving the message's nonce: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("encrypting the message: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encrypting the message: %w", err)
	}

	// One record, ended by the delimiter that says it is the last.
	record := append(append(make([]byte, 0, len(plain)+1), plain...), 0x02)
	if len(record)+gcm.Overhead() > recordSize {
		return nil, fmt.Errorf("a push message of %d bytes is more than one record holds", len(plain))
	}

	header := make([]byte, 0, len(salt)+5+len(senderKey))
	header = append(header, salt...)
	header = binary.BigEndian.AppendUint32(header, recordSize)
	header = append(header, byte(len(senderKey)))
	header = append(header, senderKey...)

	return gcm.Seal(header, nonce, record, nil), nil
}

// authorization is the Authorization header that signs a message to endpoint,
// as RFC 8292 says: a JWT, ES256, whose audience is the push service, valid for
// twelve hours, naming subject as whom the push service may contact - and the
// public key beside it, for the push service to check it against the key the
// browser subscribed with.
func (k Keys) authorization(endpoint, subject string, now time.Time) (string, error) {
	service, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("reading the push service's address: %w", err)
	}

	claims, err := json.Marshal(map[string]any{
		"aud": service.Scheme + "://" + service.Host,
		"exp": now.Add(12 * time.Hour).Unix(),
		"sub": subject,
	})
	if err != nil {
		return "", fmt.Errorf("writing the push signature: %w", err)
	}

	signed := encoded([]byte(`{"typ":"JWT","alg":"ES256"}`)) + "." + encoded(claims)
	digest := sha256.Sum256([]byte(signed))

	r, s, err := ecdsa.Sign(rand.Reader, k.private, digest[:])
	if err != nil {
		return "", fmt.Errorf("signing the push message: %w", err)
	}

	// JWS writes an ES256 signature as the two numbers side by side, 32 bytes each.
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])

	public, err := k.Public()
	if err != nil {
		return "", err
	}

	return "vapid t=" + signed + "." + encoded(signature) + ", k=" + public, nil
}

func encoded(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// KnownService says whether endpoint is the address of a push service a browser
// subscribes through: Google's for Chrome, Edge's own, Mozilla's for Firefox and
// Apple's for Safari, over https.
//
// Anything else is refused, because the installation posts to whatever address
// a subscription names, and an address anybody signed in could choose would let
// them make the installation ask its own network for whatever they liked.
func KnownService(endpoint string) bool {
	address, err := url.Parse(endpoint)
	if err != nil || address.Scheme != "https" || address.User != nil {
		return false
	}

	host := strings.ToLower(address.Hostname())
	if address.Port() != "" && address.Port() != "443" {
		return false
	}

	switch {
	case host == "fcm.googleapis.com":
		return true
	case host == "push.services.mozilla.com" || strings.HasSuffix(host, ".push.services.mozilla.com"):
		return true
	case strings.HasSuffix(host, ".push.apple.com"):
		return true
	case strings.HasSuffix(host, ".notify.windows.com"):
		return true
	}

	return false
}
