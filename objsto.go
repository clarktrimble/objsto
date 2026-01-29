package objsto

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

// Config is Client configurables tagged for use with envconfig.
type Config struct {
	Region    string `json:"region" desc:"provider region" required:"true"`
	Scheme    string `json:"scheme" desc:"http or https" default:"https"`
	Host      string `json:"host" desc:"endpoint hostname" required:"true"`
	Bucket    string `json:"bucket" desc:"bucket name" required:"true"`
	AccessKey string `json:"access_key" desc:"credential identifier" required:"true"`
	SecretKey Redact `json:"secret_key" desc:"credential secret or path to file" required:"true"`
}

// HttpDoer performs HTTP requests. *http.Client satisfies this interface.
type HttpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client is an S3 client.
type Client struct {
	region    string
	scheme    string
	host      string
	bucket    string
	accessKey string
	secretKey string
	client    HttpDoer
	logger    Logger
}

// New creates Client from Config.
func (cfg *Config) New(lgr Logger, client HttpDoer) *Client {

	return &Client{
		region:    cfg.Region,
		scheme:    cfg.Scheme,
		host:      cfg.Host,
		bucket:    cfg.Bucket,
		accessKey: cfg.AccessKey,
		secretKey: string(cfg.SecretKey),
		client:    client,
		logger:    lgr,
	}
}

// Get gets an object.
func (c *Client) Get(ctx context.Context, object string) (reader io.ReadCloser, err error) {

	req, err := c.buildRequest(ctx, "GET", object, nil)
	if err != nil {
		return
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return
	}

	reader = resp.Body
	return
}

// Put puts an object.
func (c *Client) Put(ctx context.Context, object string, reader io.ReadSeeker) (err error) {

	req, err := c.buildRequest(ctx, "PUT", object, reader)
	if err != nil {
		return
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return
	}
	resp.Body.Close()

	return
}

// unexported

func (c *Client) buildRequest(ctx context.Context, method, object string, pyld io.ReadSeeker) (req *http.Request, err error) {

	if object == "" {
		err = errors.Errorf("object cannot be blank")
		return
	}

	// create request

	path := fmt.Sprintf("/%s/%s", c.bucket, object)
	uri := fmt.Sprintf("%s://%s%s", c.scheme, c.host, path)
	now := time.Now().UTC()

	req, err = http.NewRequestWithContext(ctx, method, uri, pyld)
	if err != nil {
		err = errors.Wrapf(err, "failed to create request to %q", uri)
		return
	}

	c.logger.Debug(ctx, "signing request",
		"region", c.region,
		"host", c.host,
		"path", path,
		"access_key", c.accessKey,
		"now", now,
	)

	// add signature headers

	hash, size, err := hashPayload(pyld)
	if err != nil {
		return
	}

	headers := signRequest(method, c.region, c.host, path, c.accessKey, c.secretKey, hash, now)

	req.ContentLength = size
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	c.logger.Debug(ctx, "signed request",
		"url", req.URL.String(),
		"host", req.Host,
		"headers", req.Header,
	)

	return
}

func (c *Client) sendRequest(ctx context.Context, req *http.Request) (resp *http.Response, err error) {

	start := time.Now()
	resp, err = c.client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		err = errors.Wrapf(err, "failed request to %q", req.URL)
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		err = parseS3Error(resp)
		return
	}

	c.logger.Info(ctx, "received response", "status", resp.StatusCode, "elapsed", elapsed)

	return
}

func hashPayload(body io.ReadSeeker) (hash string, size int64, err error) {

	h := sha256.New()
	if body != nil {
		size, err = io.Copy(h, body)
		if err != nil {
			err = errors.Wrap(err, "failed to hash body")
			return
		}
		_, err = body.Seek(0, io.SeekStart)
		if err != nil {
			err = errors.Wrap(err, "failed to seek body")
			return
		}
	}
	hash = hex.EncodeToString(h.Sum(nil))

	return
}

type s3Error struct {
	Code      string `xml:"Code"`
	Message   string `xml:"Message"`
	RequestID string `xml:"RequestId"`
}

func parseS3Error(resp *http.Response) error {

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*4))

	var s3Err s3Error
	err := xml.Unmarshal(bodyBytes, &s3Err)
	if err != nil {
		return errors.Errorf("http error, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return errors.Errorf("s3 error, code: %s, request_id: %s, message: %s, headers: %s",
		s3Err.Code, s3Err.RequestID, s3Err.Message, resp.Header)
}

// vibe coded goodness

const (
	service = "s3"
)

func signRequest(method, region, host, path, accessKey, secretKey, payloadHash string, t time.Time) map[string]string {

	amzDate := t.Format("20060102T150405Z")
	dateStamp := t.Format("20060102")

	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", host, payloadHash, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := fmt.Sprintf("%s\n%s\n\n%s\n%s\n%s", method, path, canonicalHeaders, signedHeaders, payloadHash)

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s", amzDate, credentialScope, sha256Hash(canonicalRequest))

	signingKey := getSignatureKey(secretKey, dateStamp, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credentialScope, signedHeaders, signature)

	return map[string]string{
		"Authorization":        authHeader,
		"x-amz-date":           amzDate,
		"x-amz-content-sha256": payloadHash,
	}
}

func sha256Hash(data string) string {
	h := sha256.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func getSignatureKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	return kSigning
}
