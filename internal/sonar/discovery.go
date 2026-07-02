// Package sonar discovers and proxies SteelSeries Sonar's local HTTP API,
// whose port rotates on every restart.
package sonar

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Discovery chain:
//   1. coreProps.json -> ggEncryptedAddress
//   2. https://{ggEncryptedAddress}/subApps  (self-signed TLS)
//   3. subApps.sonar.metadata.webServerAddress -> the API base URL
type Client struct {
	mu    sync.RWMutex
	base  *url.URL
	disco *http.Client
}

func NewClient() *Client {
	return &Client{
		disco: &http.Client{
			Timeout:   4 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		},
	}
}

// Base returns the currently known Sonar API base URL (nil until discovered).
func (c *Client) Base() *url.URL {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.base
}

func corePropsPath() string {
	if p := os.Getenv("SONAR_COREPROPS"); p != "" {
		return p
	}
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pd, "SteelSeries", "GG", "coreProps.json")
}

// Discover runs the full chain and caches the resulting base URL.
func (c *Client) Discover() error {
	raw, err := os.ReadFile(corePropsPath())
	if err != nil {
		return fmt.Errorf("read coreProps: %w", err)
	}
	var core struct {
		GGEncryptedAddress string `json:"ggEncryptedAddress"`
	}
	if err := json.Unmarshal(raw, &core); err != nil {
		return fmt.Errorf("parse coreProps: %w", err)
	}
	if core.GGEncryptedAddress == "" {
		return fmt.Errorf("ggEncryptedAddress missing (is SteelSeries GG running?)")
	}

	resp, err := c.disco.Get("https://" + core.GGEncryptedAddress + "/subApps")
	if err != nil {
		return fmt.Errorf("query subApps: %w", err)
	}
	defer resp.Body.Close()

	var sub struct {
		SubApps struct {
			Sonar struct {
				Metadata struct {
					WebServerAddress string `json:"webServerAddress"`
				} `json:"metadata"`
			} `json:"sonar"`
		} `json:"subApps"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		return fmt.Errorf("parse subApps: %w", err)
	}

	addr := sub.SubApps.Sonar.Metadata.WebServerAddress
	if addr == "" {
		return fmt.Errorf("sonar webServerAddress empty (Sonar sub-app not running?)")
	}
	u, err := url.Parse(addr)
	if err != nil {
		return fmt.Errorf("parse sonar address %q: %w", addr, err)
	}

	c.mu.Lock()
	c.base = u
	c.mu.Unlock()
	return nil
}

// EnsureDiscovered performs an initial discovery, retrying quietly in the
// background if Sonar isn't up yet so the server can start immediately.
func (c *Client) EnsureDiscovered() {
	if err := c.Discover(); err == nil {
		return
	}
	go func() {
		for c.Base() == nil {
			time.Sleep(3 * time.Second)
			_ = c.Discover()
		}
	}()
}
