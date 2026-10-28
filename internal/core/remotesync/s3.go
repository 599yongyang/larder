package remotesync

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type S3 struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	Prefix    string
	Filename  string
	Client    *http.Client
}

func NewS3(endpoint, region, bucket, accessKey, secretKey, prefix, filename string) (*S3, error) {
	endpoint = strings.TrimSpace(endpoint)
	bucket = strings.TrimSpace(bucket)
	region = strings.TrimSpace(region)
	if endpoint == "" {
		return nil, errors.New("S3 endpoint is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname()))) {
		return nil, errors.New("S3 endpoint must use HTTPS (HTTP is only allowed for localhost)")
	}
	if bucket == "" {
		return nil, errors.New("S3 bucket is required")
	}
	if accessKey == "" || secretKey == "" {
		return nil, errors.New("S3 access key and secret key are required")
	}
	if region == "" {
		region = "us-east-1"
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = "larder-data.zip"
	}
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	return &S3{Endpoint: strings.TrimRight(endpoint, "/"), Region: region, Bucket: bucket, AccessKey: accessKey, SecretKey: secretKey, Prefix: prefix, Filename: filename, Client: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (s *S3) key() string {
	parts := []string{}
	if s.Prefix != "" {
		parts = append(parts, s.Prefix)
	}
	parts = append(parts, s.Filename)
	return strings.Join(parts, "/")
}

func encodePath(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

func (s *S3) request(ctx context.Context, method, objectKey string, body []byte, contentType string) (*http.Response, error) {
	path := "/" + encodePath(s.Bucket)
	if objectKey != "" {
		path += "/" + encodePath(objectKey)
	}
	u, err := url.Parse(s.Endpoint + path)
	if err != nil {
		return nil, err
	}
	payloadHash := sha256Hex(body)
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")
	canonicalHeaders := "host:" + u.Host + "\n" + "x-amz-content-sha256:" + payloadHash + "\n" + "x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := strings.Join([]string{method, u.EscapedPath(), "", canonicalHeaders, signedHeaders, payloadHash}, "\n")
	scope := shortDate + "/" + s.Region + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex([]byte(canonicalRequest))
	signingKey := hmacSHA256([]byte("AWS4"+s.SecretKey), shortDate)
	signingKey = hmacSHA256(signingKey, s.Region)
	signingKey = hmacSHA256(signingKey, "s3")
	signingKey = hmacSHA256(signingKey, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	authorization := "AWS4-HMAC-SHA256 Credential=" + s.AccessKey + "/" + scope + ", SignedHeaders=" + signedHeaders + ", Signature=" + signature
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("Authorization", authorization)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return nil, fmt.Errorf("S3 %s: HTTP %d %s", method, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return resp, nil
}

func (s *S3) Test(ctx context.Context) error {
	resp, err := s.request(ctx, "HEAD", "", nil, "")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (s *S3) Upload(ctx context.Context, data []byte) error {
	if len(data) > maxSyncPackageBytes {
		return errors.New("sync package is too large")
	}
	resp, err := s.request(ctx, "PUT", s.key(), data, "application/zip")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (s *S3) Download(ctx context.Context) ([]byte, error) {
	resp, err := s.request(ctx, "GET", s.key(), nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxSyncPackageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxSyncPackageBytes {
		return nil, errors.New("sync package is too large")
	}
	return raw, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(data))
	return mac.Sum(nil)
}
