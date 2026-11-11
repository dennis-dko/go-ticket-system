package webpush

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The receiver of RFC 8291's example in section 5, which Appendix A works through.
const (
	receiverPrivate = "q1dXpw3UpT5VOmu_cf_v6ih07Aems3njxI-JWgLcM94"
	receiverPublic  = "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4"
	receiverSecret  = "BTBZMqHH6r4Tts7J_aSIgg"
)

func decoded(t *testing.T, s string) []byte {
	t.Helper()

	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("decoding %q: %v", s, err)
	}

	return b
}

// The RFC's own example comes out byte for byte: its sender's key pair, its salt,
// its receiver and its plaintext give exactly the body section 5 prints.
func TestTheRFCsExampleIsEncryptedByteForByte(t *testing.T) {
	sender, err := ecdh.P256().NewPrivateKey(decoded(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	if err != nil {
		t.Fatal(err)
	}

	if got := encoded(sender.PublicKey().Bytes()); got != "BP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A8" {
		t.Fatalf("the sender's public key is %s, which is not the RFC's", got)
	}

	body, err := encrypt([]byte("When I grow up, I want to be a watermelon"),
		Subscription{P256dh: receiverPublic, Auth: receiverSecret}, sender, decoded(t, "DGv6ra1nlYgDCS1FRnbzlw"))
	if err != nil {
		t.Fatal(err)
	}

	want := "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27ml" +
		"mlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPT" +
		"pK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"

	if got := encoded(body); got != want {
		t.Errorf("the RFC's example encrypts to\n%s\nwhere the RFC says\n%s", got, want)
	}
}

// opened is what the receiver of the RFC's example reads in body, worked the way
// a browser works it: from the header's salt and key, with its own private key.
func opened(t *testing.T, body []byte) []byte {
	t.Helper()

	receiver, err := ecdh.P256().NewPrivateKey(decoded(t, receiverPrivate))
	if err != nil {
		t.Fatal(err)
	}

	salt, size := body[:16], int(body[20])
	senderKey := body[21 : 21+size]

	sender, err := ecdh.P256().NewPublicKey(senderKey)
	if err != nil {
		t.Fatal(err)
	}

	shared, err := receiver.ECDH(sender)
	if err != nil {
		t.Fatal(err)
	}

	combining, _ := hkdf.Extract(sha256.New, shared, decoded(t, receiverSecret))
	material, _ := hkdf.Expand(sha256.New, combining, "WebPush: info\x00"+string(decoded(t, receiverPublic))+string(senderKey), 32)
	content, _ := hkdf.Extract(sha256.New, material, salt)
	key, _ := hkdf.Expand(sha256.New, content, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Expand(sha256.New, content, "Content-Encoding: nonce\x00", 12)

	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)

	record, err := gcm.Open(nil, nonce, body[21+size:], nil)
	if err != nil {
		t.Fatalf("the receiver cannot open the message: %v", err)
	}

	if len(record) == 0 || record[len(record)-1] != 0x02 {
		t.Fatalf("the record does not end with the delimiter that says it is the last: %x", record)
	}

	return record[:len(record)-1]
}

// What Encrypt writes with a key and a salt of its own, the receiver reads.
func TestAMessageIsReadByTheBrowserItIsFor(t *testing.T) {
	plain := []byte(`{"title":"The stopwatch is still running"}`)

	body, err := Encrypt(plain, Subscription{P256dh: receiverPublic, Auth: receiverSecret})
	if err != nil {
		t.Fatal(err)
	}

	if got := string(opened(t, body)); got != string(plain) {
		t.Errorf("the browser reads %q", got)
	}
}

// A message is one record: the longest that fits is encrypted, and one byte more
// is refused rather than sent as something no browser could read.
func TestAMessageLongerThanOneRecordIsRefused(t *testing.T) {
	sub := Subscription{P256dh: receiverPublic, Auth: receiverSecret}
	longest := recordSize - 1 - 16

	if _, err := Encrypt(make([]byte, longest), sub); err != nil {
		t.Errorf("a message of %d bytes, which one record holds, is refused: %v", longest, err)
	}

	if _, err := Encrypt(make([]byte, longest+1), sub); err == nil {
		t.Errorf("a message of %d bytes, more than one record holds, is encrypted", longest+1)
	}
}

// The signature names the push service as its audience, says whom to contact,
// lasts twelve hours, and is the installation's: the public key beside it checks it.
func TestAMessageIsSignedForItsPushService(t *testing.T) {
	keys, err := GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}

	now := time.Unix(1_800_000_000, 0)

	signed, err := keys.authorization("https://fcm.googleapis.com/fcm/send/abc", "https://time.example.com", now)
	if err != nil {
		t.Fatal(err)
	}

	token, public, found := strings.Cut(strings.TrimPrefix(signed, "vapid t="), ", k=")
	if !found || !strings.HasPrefix(signed, "vapid t=") {
		t.Fatalf("the header reads %q", signed)
	}

	if want, _ := keys.Public(); public != want {
		t.Errorf("the header names the key %s, and the installation's is %s", public, want)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("the token has %d parts", len(parts))
	}

	var claims struct {
		Audience string `json:"aud"`
		Expires  int64  `json:"exp"`
		Subject  string `json:"sub"`
	}

	if err := json.Unmarshal(decoded(t, parts[1]), &claims); err != nil {
		t.Fatal(err)
	}

	if claims.Audience != "https://fcm.googleapis.com" || claims.Subject != "https://time.example.com" ||
		claims.Expires != now.Add(12*time.Hour).Unix() {
		t.Errorf("the claims are %+v", claims)
	}

	key, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), decoded(t, public))
	if err != nil {
		t.Fatal(err)
	}

	signature := decoded(t, parts[2])
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))

	if len(signature) != 64 || !ecdsa.Verify(key, digest[:],
		new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:])) {
		t.Error("the signature does not check out against the installation's public key")
	}
}

// A pair written out and read back is the same pair, which is what lets the
// installation keep it.
func TestTheKeysSurviveBeingKept(t *testing.T) {
	keys, err := GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}

	private, err := keys.Private()
	if err != nil {
		t.Fatal(err)
	}

	again, err := ParseKeys(private)
	if err != nil {
		t.Fatal(err)
	}

	first, _ := keys.Public()
	second, _ := again.Public()

	if first != second {
		t.Errorf("read back, the public key is %s rather than %s", second, first)
	}
}

// Send posts the encrypted message with what a push service asks for, and says
// when the push service no longer knows the subscription.
func TestSendPostsWhatAPushServiceAsksFor(t *testing.T) {
	keys, err := GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}

	answer := http.StatusCreated

	var got *http.Request

	var body []byte

	service := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(answer)
	}))
	defer service.Close()

	sub := Subscription{Endpoint: service.URL + "/push/abc", P256dh: receiverPublic, Auth: receiverSecret, Origin: "https://time.example.com"}
	message := Message{Title: "The stopwatch is still running", Body: "Since 08:15", Tag: "gtr-stopwatch",
		At: "2026-11-10T09:15:00Z", About: "2026-11-10T07:15:00Z"}

	if err := keys.Send(t.Context(), service.Client(), sub, message, time.Minute); err != nil {
		t.Fatal(err)
	}

	// A reminder is about a moment: high, which a push service hands to a dozing
	// phone at once rather than when it next wakes.
	for header, want := range map[string]string{"Content-Encoding": "aes128gcm", "TTL": "60", "Urgency": "high"} {
		if got.Header.Get(header) != want {
			t.Errorf("%s is %q, want %q", header, got.Header.Get(header), want)
		}
	}

	if !strings.HasPrefix(got.Header.Get("Authorization"), "vapid t=") {
		t.Errorf("the message is not signed: %q", got.Header.Get("Authorization"))
	}

	var read Message
	if err := json.Unmarshal(opened(t, body), &read); err != nil || read != message {
		t.Errorf("the browser reads %+v (%v)", read, err)
	}

	answer = http.StatusGone
	if err := keys.Send(t.Context(), service.Client(), sub, message, time.Minute); !errors.Is(err, ErrGone) {
		t.Errorf("a subscription the push service no longer knows is answered %v", err)
	}

	answer = http.StatusInternalServerError
	if err := keys.Send(t.Context(), service.Client(), sub, message, time.Minute); err == nil || errors.Is(err, ErrGone) {
		t.Errorf("a push service that failed is answered %v", err)
	}
}

// Only the push services browsers subscribe through are posted to.
func TestOnlyTheKnownPushServicesArePostedTo(t *testing.T) {
	for endpoint, known := range map[string]bool{
		"https://fcm.googleapis.com/fcm/send/abc":                true,
		"https://updates.push.services.mozilla.com/wpush/v2/abc": true,
		"https://web.push.apple.com/abc":                         true,
		"https://wns2-par02p.notify.windows.com/w/?token=abc":    true,
		"http://fcm.googleapis.com/fcm/send/abc":                 false,
		"https://fcm.googleapis.com:8443/fcm/send/abc":           false,
		"https://user@fcm.googleapis.com/fcm/send/abc":           false,
		"https://fcm.googleapis.com.example.net/abc":             false,
		"https://push.apple.com.example.net/abc":                 false,
		"https://notify.windows.com.example.net/abc":             false,
		"https://127.0.0.1/abc":                                  false,
		"https://localhost/abc":                                  false,
		"https://intranet.example.com/push":                      false,
		"not an address":                                         false,
	} {
		if KnownService(endpoint) != known {
			t.Errorf("KnownService(%q) is %v", endpoint, !known)
		}
	}
}
