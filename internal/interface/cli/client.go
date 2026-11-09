package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// requestTimeout bounds one request: a terminal waiting on an installation that
// has gone away should say so rather than hang until somebody presses Ctrl-C.
const requestTimeout = 30 * time.Second

// maxAnswer bounds how much of an answer is read. The largest this asks for is a
// page of entries, which is some tens of kilobytes; anything near this is not
// one of this application's answers.
const maxAnswer = 8 << 20

// maxFile bounds a file the installation answers with, which is an export of
// every entry a filter finds: larger than any answer, and still bounded.
const maxFile = 64 << 20

// client asks one installation's API as the owner of one personal token, and
// says what went wrong in the language the client speaks.
type client struct {
	base  string
	token string
	http  *http.Client
	lang  language
}

// newClient is a client for the installation at base, which may end in a slash.
func newClient(base, token string, lang language) *client {
	return &client{
		base:  strings.TrimRight(base, "/"),
		token: token,
		http:  &http.Client{Timeout: requestTimeout},
		lang:  lang,
	}
}

// refusal is what the installation answered instead of doing something, in its
// own words - which are what the person at the terminal needs to read.
type refusal string

func (r refusal) Error() string { return string(r) }

// call sends a request and reads the data of the answer into into, nil for an
// answer nobody needs.
func (c *client) call(ctx context.Context, method, path string, body, into any) error {
	var (
		payload io.Reader
		kind    string
	)

	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}

		payload, kind = bytes.NewReader(encoded), "application/json"
	}

	raw, err := c.send(ctx, method, path, kind, payload, maxAnswer)
	if err != nil || into == nil {
		return err
	}

	return c.data(raw, into)
}

// fetch reads a file the installation answers with, such as an export, as it
// comes: a file is not wrapped in the envelope data travels in.
func (c *client) fetch(ctx context.Context, path string) ([]byte, error) {
	return c.send(ctx, http.MethodGet, path, "", nil, maxFile)
}

// render sends a document to be laid out, and reads the file it comes back as.
func (c *client) render(ctx context.Context, path string, document any) ([]byte, error) {
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}

	return c.send(ctx, http.MethodPost, path, "application/json", bytes.NewReader(encoded), maxFile)
}

// upload sends a file the way a browser's form sends one, named name, beside the
// fields given, and reads the data of the answer into into.
func (c *client) upload(ctx context.Context, path, name string, content []byte, fields map[string]string, into any) error {
	var form bytes.Buffer

	writer := multipart.NewWriter(&form)

	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return err
		}
	}

	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		return err
	}

	if _, err := part.Write(content); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	raw, err := c.send(ctx, http.MethodPost, path, writer.FormDataContentType(), &form, maxAnswer)
	if err != nil {
		return err
	}

	return c.data(raw, into)
}

// send sends a request with a payload of the kind given, and answers with what
// came back - no more than most of it - or with why it was refused.
func (c *client) send(ctx context.Context, method, path, kind string, payload io.Reader, most int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+"/api/v1"+path, payload)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.lang.text("track.badAddress", "\"{0}\" is not an address this can ask", c.base), err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	if kind != "" {
		req.Header.Set("Content-Type", kind)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.lang.text("track.unreachable", "could not reach {0}", c.base), err)
	}

	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, most))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.lang.text("track.unreadable", "reading the answer from {0}", c.base), err)
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, c.lang.refusal(resp.StatusCode, raw)
	}

	return raw, nil
}

// data reads the data of an answer, which travels in GoFr's envelope, into into.
func (c *client) data(raw []byte, into any) error {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data == nil {
		return errors.New(c.lang.text("track.notThisApplication", "{0} answered with something that is not this application's", c.base))
	}

	return json.Unmarshal(envelope.Data, into)
}
