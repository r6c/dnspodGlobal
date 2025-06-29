package dnspod

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/libdns/libdns"
)

const (
	// DNSPod API base URL - must use HTTPS as per API requirements
	baseURL = "https://dnsapi.cn"

	// Common response codes
	successCode = "1"

	// UserAgent format as required by DNSPod API: Program Name/Version (Contact Email)
	// DNSPod requires this exact format, otherwise the account will be banned
	userAgent = "libdns-dnspod/1.0.0 (github.com/r6c/dnspodGlobal)"
)

// DNSPod API response structures
type apiResponse struct {
	Status struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		CreatedAt string `json:"created_at"`
	} `json:"status"`
}

type domainListResponse struct {
	apiResponse
	Info struct {
		DomainTotal int `json:"domain_total"`
	} `json:"info"`
	Domains []domain `json:"domains"`
}

type recordListResponse struct {
	apiResponse
	Info struct {
		SubDomains string `json:"sub_domains"`
	} `json:"info"`
	Records []record `json:"records"`
}

type recordResponse struct {
	apiResponse
	Record record `json:"record"`
}

type domain struct {
	ID     json.Number `json:"id"`
	Name   string      `json:"name"`
	Status string      `json:"status"`
}

type record struct {
	ID        string `json:"id"`
	TTL       string `json:"ttl"`
	Value     string `json:"value"`
	Enabled   string `json:"enabled"`
	Status    string `json:"status"`
	UpdatedOn string `json:"updated_on"`
	Name      string `json:"name"`
	Line      string `json:"line"`
	LineID    string `json:"line_id"`
	Type      string `json:"type"`
	Weight    string `json:"weight,omitempty"`
	MX        string `json:"mx,omitempty"`
	Remark    string `json:"remark,omitempty"`
}

// Client wraps HTTP client for DNSPod API
type Client struct {
	httpClient *http.Client
	loginToken string
	mutex      sync.RWMutex
	domainList []domain
}

// newClient creates a new DNSPod API client
func newClient(loginToken string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		loginToken: loginToken,
	}
}

// makeRequest makes an HTTP POST request to DNSPod API
func (c *Client) makeRequest(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	if params == nil {
		params = make(map[string]string)
	}

	// Add common parameters as required by DNSPod API
	// See: https://docs.dnspod.com/api/common-request-parameters/
	params["login_token"] = c.loginToken
	params["format"] = "json"       // Recommended format
	params["error_on_empty"] = "no" // Don't return error when no results
	params["lang"] = "cn"           // Use Chinese for better error messages (CN API specific)

	// Prepare form data
	data := url.Values{}
	for key, value := range params {
		data.Set(key, value)
	}

	// Create request
	reqURL := fmt.Sprintf("%s/%s", baseURL, endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers as per DNSPod API specification
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Parse basic response to check API status
	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Status.Code != successCode {
		return nil, fmt.Errorf("API error: %s - %s", apiResp.Status.Code, apiResp.Status.Message)
	}

	return body, nil
}

// getDomains fetches and caches domain list
func (c *Client) getDomains(ctx context.Context) ([]domain, error) {
	c.mutex.RLock()
	if len(c.domainList) > 0 {
		domains := make([]domain, len(c.domainList))
		copy(domains, c.domainList)
		c.mutex.RUnlock()
		return domains, nil
	}
	c.mutex.RUnlock()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Double-check after acquiring write lock
	if len(c.domainList) > 0 {
		domains := make([]domain, len(c.domainList))
		copy(domains, c.domainList)
		return domains, nil
	}

	body, err := c.makeRequest(ctx, "Domain.List", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %w", err)
	}

	var resp domainListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse domain list response: %w", err)
	}

	c.domainList = resp.Domains
	domains := make([]domain, len(c.domainList))
	copy(domains, c.domainList)
	return domains, nil
}

// getDomainID finds domain ID by domain name
func (c *Client) getDomainID(ctx context.Context, domainName string) (string, error) {
	domainName = strings.TrimSuffix(domainName, ".")

	domains, err := c.getDomains(ctx)
	if err != nil {
		return "", err
	}

	for _, domain := range domains {
		if domain.Name == domainName {
			return string(domain.ID), nil
		}
	}

	return "", fmt.Errorf("domain %s not found in DNSPod account", domainName)
}

// listRecords retrieves all DNS records for a domain
func (c *Client) listRecords(ctx context.Context, domainID string) ([]record, error) {
	params := map[string]string{
		"domain_id": domainID,
	}

	body, err := c.makeRequest(ctx, "Record.List", params)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}

	var resp recordListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse record list response: %w", err)
	}

	return resp.Records, nil
}

// createRecord creates a new DNS record
func (c *Client) createRecord(ctx context.Context, domainID string, rec record) (*record, error) {
	params := map[string]string{
		"domain_id":   domainID,
		"sub_domain":  rec.Name,
		"record_type": rec.Type,
		"record_line": "默认",
		"value":       rec.Value,
	}

	if rec.TTL != "" {
		params["ttl"] = rec.TTL
	} else {
		params["ttl"] = "600" // Default TTL
	}

	if rec.MX != "" {
		params["mx"] = rec.MX
	}

	body, err := c.makeRequest(ctx, "Record.Create", params)
	if err != nil {
		return nil, fmt.Errorf("failed to create record: %w", err)
	}

	var resp recordResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse create record response: %w", err)
	}

	return &resp.Record, nil
}

// updateRecord updates an existing DNS record
func (c *Client) updateRecord(ctx context.Context, domainID, recordID string, rec record) (*record, error) {
	params := map[string]string{
		"domain_id":   domainID,
		"record_id":   recordID,
		"sub_domain":  rec.Name,
		"record_type": rec.Type,
		"record_line": "默认",
		"value":       rec.Value,
	}

	if rec.TTL != "" {
		params["ttl"] = rec.TTL
	}

	if rec.MX != "" {
		params["mx"] = rec.MX
	}

	body, err := c.makeRequest(ctx, "Record.Modify", params)
	if err != nil {
		return nil, fmt.Errorf("failed to update record: %w", err)
	}

	var resp recordResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse update record response: %w", err)
	}

	return &resp.Record, nil
}

// deleteRecord deletes a DNS record
func (c *Client) deleteRecord(ctx context.Context, domainID, recordID string) error {
	params := map[string]string{
		"domain_id": domainID,
		"record_id": recordID,
	}

	_, err := c.makeRequest(ctx, "Record.Remove", params)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	return nil
}

// Helper functions for record name processing

// extractRecordName extracts the subdomain part from a full domain name
func extractRecordName(name, zone string) string {
	name = strings.TrimSuffix(name, ".")
	zone = strings.TrimSuffix(zone, ".")

	if name == zone {
		return "@"
	}

	if strings.HasSuffix(name, "."+zone) {
		return strings.TrimSuffix(name, "."+zone)
	}

	return name
}

// makeAbsoluteName creates an absolute domain name from a relative name and zone
func makeAbsoluteName(name, zone string) string {
	zone = strings.TrimSuffix(zone, ".")

	if name == "@" || name == "" {
		return zone + "."
	}

	if strings.HasSuffix(name, ".") {
		return name
	}

	return name + "." + zone + "."
}

// convertToLibDNSRecord converts a DNSPod record to libdns.Record format
func convertToLibDNSRecord(rec record, zone string) libdns.Record {
	ttl, _ := strconv.ParseInt(rec.TTL, 10, 64)
	ttlDuration := time.Duration(ttl) * time.Second

	absoluteName := makeAbsoluteName(rec.Name, zone)

	// Return specific libdns record types based on the DNS record type
	switch strings.ToUpper(rec.Type) {
	case "A", "AAAA":
		ip, err := netip.ParseAddr(rec.Value)
		if err != nil {
			// Fallback to RR if IP parsing fails
			return libdns.RR{
				Name: absoluteName,
				Type: rec.Type,
				TTL:  ttlDuration,
				Data: rec.Value,
			}
		}
		return libdns.Address{
			Name: absoluteName,
			IP:   ip,
			TTL:  ttlDuration,
		}
	case "TXT":
		return libdns.TXT{
			Name: absoluteName,
			Text: rec.Value,
			TTL:  ttlDuration,
		}
	case "CNAME":
		return libdns.CNAME{
			Name:   absoluteName,
			Target: rec.Value,
			TTL:    ttlDuration,
		}
	case "MX":
		preference := 0
		if rec.MX != "" {
			if p, err := strconv.Atoi(rec.MX); err == nil {
				preference = p
			}
		}
		return libdns.MX{
			Name:       absoluteName,
			Target:     rec.Value,
			Preference: uint16(preference),
			TTL:        ttlDuration,
		}
	default:
		// For all other record types (NS, SOA, SRV, etc.), use RR
		return libdns.RR{
			Name: absoluteName,
			Type: rec.Type,
			TTL:  ttlDuration,
			Data: rec.Value,
		}
	}
}

// convertFromLibDNSRecord converts a libdns.Record to DNSPod record format
func convertFromLibDNSRecord(libRec libdns.Record, zone string) record {
	// Handle different record types
	switch r := libRec.(type) {
	case libdns.Address:
		return record{
			Name:  extractRecordName(r.Name, zone),
			Type:  getRecordType(r.IP),
			Value: r.IP.String(),
			TTL:   strconv.Itoa(int(r.TTL.Seconds())),
		}
	case libdns.TXT:
		return record{
			Name:  extractRecordName(r.Name, zone),
			Type:  "TXT",
			Value: r.Text,
			TTL:   strconv.Itoa(int(r.TTL.Seconds())),
		}
	case libdns.CNAME:
		return record{
			Name:  extractRecordName(r.Name, zone),
			Type:  "CNAME",
			Value: r.Target,
			TTL:   strconv.Itoa(int(r.TTL.Seconds())),
		}
	case libdns.MX:
		return record{
			Name:  extractRecordName(r.Name, zone),
			Type:  "MX",
			Value: r.Target,
			MX:    strconv.Itoa(int(r.Preference)),
			TTL:   strconv.Itoa(int(r.TTL.Seconds())),
		}
	case libdns.RR:
		return record{
			Name:  extractRecordName(r.Name, zone),
			Type:  r.Type,
			Value: r.Data,
			TTL:   strconv.Itoa(int(r.TTL.Seconds())),
		}
	default:
		// Fallback to RR conversion
		rr := libRec.RR()
		return record{
			Name:  extractRecordName(rr.Name, zone),
			Type:  rr.Type,
			Value: rr.Data,
			TTL:   strconv.Itoa(int(rr.TTL.Seconds())),
		}
	}
}

// getRecordType determines A or AAAA based on IP address
func getRecordType(ip fmt.Stringer) string {
	ipStr := ip.String()
	if strings.Contains(ipStr, ":") {
		return "AAAA"
	}
	return "A"
}
