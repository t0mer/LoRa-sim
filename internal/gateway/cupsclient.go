package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/t0mer/cylon/internal/creds"
)

// PostCUPS performs the CUPS update-info exchange: it POSTs the JSON request to
// <cupsURI>/update-info and decodes the binary response.
func PostCUPS(ctx context.Context, client *http.Client, cupsURI string, req CupsRequest) (*CupsResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	url := strings.TrimSuffix(cupsURI, "/") + "/update-info"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("CUPS request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("reading CUPS response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CUPS returned %d: %s", resp.StatusCode, raw)
	}
	return ParseCupsResponse(raw)
}

// applyTCUpdate writes back the tc.* credential material from a CUPS response
// into credsDir, splitting the credential bundle into trust/cert/key.
func applyTCUpdate(credsDir string, resp *CupsResponse) error {
	if resp.TcURI == "" && len(resp.TcCred) == 0 {
		return nil
	}
	var trust, cert, key []byte
	if len(resp.TcCred) > 0 {
		var err error
		trust, cert, key, err = SplitCredBundle(resp.TcCred)
		if err != nil {
			return fmt.Errorf("splitting tc credential bundle: %w", err)
		}
	}
	return creds.WriteTC(credsDir, resp.TcURI, trust, cert, key)
}

// BootstrapCUPS runs the full CUPS bootstrap: load credentials, exchange
// update-info over mutual TLS, and write any returned tc.* credentials back into
// credsDir so the caller can then connect to the LNS. It returns the response.
func BootstrapCUPS(ctx context.Context, credsDir string, req CupsRequest) (*CupsResponse, error) {
	c, err := creds.Load(credsDir)
	if err != nil {
		return nil, err
	}
	if !c.HasCUPS() {
		return nil, fmt.Errorf("incomplete CUPS credentials in %s", credsDir)
	}
	tlsCfg, err := c.CUPSTLSConfig()
	if err != nil {
		return nil, err
	}
	req.CupsURI = c.CupsURI
	req.CupsCredCrc = c.CupsCredCRC()
	req.TcCredCrc = c.TcCredCRC()

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}
	resp, err := PostCUPS(ctx, client, c.CupsURI, req)
	if err != nil {
		return nil, err
	}
	if err := applyTCUpdate(credsDir, resp); err != nil {
		return nil, err
	}
	return resp, nil
}
