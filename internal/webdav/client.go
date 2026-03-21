package webdav

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// FileEntry represents a file found via WebDAV PROPFIND.
type FileEntry struct {
	Path         string    // absolute path on server (e.g. /journal/Journal/standing/doc.md)
	Name         string    // filename only (e.g. doc.md)
	LastModified time.Time
}

// Client is a minimal WebDAV HTTP client for listing and fetching markdown files.
type Client struct {
	baseURL  string // full base URL e.g. http://host:port/journal
	origin   string // scheme+host+port only e.g. http://host:port
	username string
	password string
	client   *http.Client
	logger   *logrus.Logger
}

// NewClient creates a new WebDAV client with a 30-second timeout.
func NewClient(baseURL, username, password string) *Client {
	trimmed := strings.TrimRight(baseURL, "/")
	origin := trimmed
	if u, err := url.Parse(trimmed); err == nil {
		origin = u.Scheme + "://" + u.Host
	}
	return &Client{
		baseURL:  trimmed,
		origin:   origin,
		username: username,
		password: password,
		client:   &http.Client{Timeout: 30 * time.Second},
		logger:   logrus.New(),
	}
}

// SetLogger replaces the default logger.
func (c *Client) SetLogger(logger *logrus.Logger) {
	c.logger = logger
}

// List performs a PROPFIND Depth:1 on the given path and returns .md files found.
// The collection entry itself is filtered out.
func (c *Client) List(dirPath string) ([]FileEntry, error) {
	reqURL := c.baseURL + dirPath

	req, err := http.NewRequest("PROPFIND", reqURL, strings.NewReader(`<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:getlastmodified/>
    <D:getcontenttype/>
  </D:prop>
</D:propfind>`))
	if err != nil {
		return nil, fmt.Errorf("failed to create PROPFIND request: %w", err)
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Depth", "1")
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PROPFIND request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PROPFIND returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read PROPFIND response: %w", err)
	}

	entries, err := parsePropfind(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PROPFIND response: %w", err)
	}

	// Filter: only .md files, skip the collection itself
	var results []FileEntry
	normalizedDir := strings.TrimRight(dirPath, "/") + "/"
	for _, e := range entries {
		if e.Path == dirPath || e.Path == normalizedDir {
			continue // skip the collection entry
		}
		if strings.HasSuffix(strings.ToLower(e.Name), ".md") {
			results = append(results, e)
		}
	}

	c.logger.WithFields(logrus.Fields{
		"path":  dirPath,
		"found": len(results),
	}).Debug("WebDAV list complete")

	return results, nil
}

// Get fetches the content of a file at the given path.
// filePath may be an absolute server path as returned by PROPFIND href (e.g. /journal/Journal/standing/doc.md).
// It is joined to the server origin, not the baseURL, to avoid doubling the base path.
func (c *Client) Get(filePath string) ([]byte, error) {
	reqURL := c.origin + filePath

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET returned status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read GET response: %w", err)
	}

	return data, nil
}

// --- XML parsing ---

type propfindResponse struct {
	XMLName   xml.Name   `xml:"multistatus"`
	Responses []davResponse `xml:"response"`
}

type davResponse struct {
	Href     string    `xml:"href"`
	Propstat []propstat `xml:"propstat"`
}

type propstat struct {
	Status string `xml:"status"`
	Prop   davProp `xml:"prop"`
}

type davProp struct {
	LastModified string `xml:"getlastmodified"`
	ContentType  string `xml:"getcontenttype"`
}

func parsePropfind(data []byte) ([]FileEntry, error) {
	var ms propfindResponse
	if err := xml.Unmarshal(data, &ms); err != nil {
		return nil, fmt.Errorf("xml unmarshal failed: %w", err)
	}

	var entries []FileEntry
	for _, r := range ms.Responses {
		href, err := url.PathUnescape(r.Href)
		if err != nil {
			href = r.Href
		}

		entry := FileEntry{
			Path: href,
			Name: path.Base(href),
		}

		for _, ps := range r.Propstat {
			if ps.Prop.LastModified != "" {
				t, err := time.Parse(time.RFC1123, ps.Prop.LastModified)
				if err != nil {
					// try RFC1123Z variant
					t, err = time.Parse(time.RFC1123Z, ps.Prop.LastModified)
				}
				if err == nil {
					entry.LastModified = t
				}
			}
		}

		entries = append(entries, entry)
	}

	return entries, nil
}
