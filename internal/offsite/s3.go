// Package offsite uploads backup artifacts to S3-compatible storage (AWS S3,
// Cloudflare R2, MinIO, Backblaze B2) using SigV4 and only the standard library.
package offsite

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

// Config describes an S3-compatible destination. Credentials are never logged.
type Config struct {
	Endpoint  string // e.g. https://<account>.r2.cloudflarestorage.com
	Region    string // "auto" for R2; defaults to us-east-1
	Bucket    string
	Prefix    string // optional key prefix, e.g. cuan/backups
	AccessKey string
	SecretKey string
}

// Validate reports a plain-language problem with the configuration.
func (c Config) Validate() error {
	switch {
	case c.Endpoint == "":
		return errors.New("endpoint is required")
	case c.Bucket == "":
		return errors.New("bucket is required")
	case c.AccessKey == "" || c.SecretKey == "":
		return errors.New("access key and secret key are required")
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("endpoint must be an http(s) URL")
	}
	return nil
}

// Client uploads objects.
type Client struct {
	cfg  Config
	http *http.Client
	now  func() time.Time
}

func New(cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 30 * time.Minute}, now: time.Now}, nil
}

// Key joins the configured prefix and a file name.
func (c *Client) Key(name string) string {
	return strings.TrimPrefix(path.Join(c.cfg.Prefix, name), "/")
}

// Put uploads a file with a single signed PUT (path-style addressing), sending
// the SHA-256 of the body so the provider rejects a corrupted upload.
func (c *Client) Put(ctx context.Context, name, file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	payload := hex.EncodeToString(h.Sum(nil))
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	key := c.Key(name)
	base, _ := url.Parse(strings.TrimRight(c.cfg.Endpoint, "/"))
	canonURI := "/" + c.cfg.Bucket + "/" + escapePath(key)
	target := base.Scheme + "://" + base.Host + canonURI

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target, f)
	if err != nil {
		return "", err
	}
	req.ContentLength = st.Size()
	c.sign(req, canonURI, payload)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("upload rejected: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return key, nil
}

func (c *Client) sign(req *http.Request, canonURI, payload string) {
	t := c.now().UTC()
	amz := t.Format("20060102T150405Z")
	day := t.Format("20060102")
	req.Header.Set("x-amz-date", amz)
	req.Header.Set("x-amz-content-sha256", payload)

	signed := "host;x-amz-content-sha256;x-amz-date"
	canon := strings.Join([]string{
		req.Method, canonURI, "",
		"host:" + req.URL.Host,
		"x-amz-content-sha256:" + payload,
		"x-amz-date:" + amz,
		"", signed, payload,
	}, "\n")
	scope := day + "/" + c.cfg.Region + "/s3/aws4_request"
	sts := "AWS4-HMAC-SHA256\n" + amz + "\n" + scope + "\n" + hexSum(canon)
	k := mac([]byte("AWS4"+c.cfg.SecretKey), day)
	k = mac(k, c.cfg.Region)
	k = mac(k, "s3")
	k = mac(k, "aws4_request")
	sig := hex.EncodeToString(mac(k, sts))
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.cfg.AccessKey, scope, signed, sig))
}

func mac(key []byte, s string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(s))
	return m.Sum(nil)
}

func hexSum(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// escapePath percent-encodes each segment per SigV4 rules, keeping "/".
func escapePath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = strings.ReplaceAll(url.PathEscape(s), "+", "%2B")
	}
	return strings.Join(segs, "/")
}
