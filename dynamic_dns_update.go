package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	toolName             = "Dynamic DNS Update"
	whatsMyIPv4URL       = "http://ipv4.whatismyip.akamai.com"
	whatsMyIPv6URL       = "http://ipv6.whatismyip.akamai.com"
	gandiAPIBaseURL      = "https://api.gandi.net/v5/livedns"
	cloudflareAPIBaseURL = "https://api.cloudflare.com/client/v4"
	defaultTTL           = 300
	providerGandi        = "gandi"
	providerCloudflare   = "cloudflare"
)

type recordSet struct {
	Type   string
	TTL    int
	Values []string
}

type gandiRecordSet struct {
	Type   string   `json:"rrset_type"`
	TTL    int      `json:"rrset_ttl"`
	Values []string `json:"rrset_values"`
}

type optionalBoolFlag struct {
	value bool
	set   bool
}

func (f *optionalBoolFlag) Set(value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	f.value = parsed
	f.set = true
	return nil
}

func (f *optionalBoolFlag) String() string {
	return strconv.FormatBool(f.value)
}

func (f *optionalBoolFlag) IsBoolFlag() bool {
	return true
}

func (f *optionalBoolFlag) valuePtr() *bool {
	if !f.set {
		return nil
	}
	return &f.value
}

type cloudflareRecordPayload struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied *bool  `json:"proxied,omitempty"`
}

type cloudflareDNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

type cloudflareAPIMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareListRecordsResponse struct {
	Success  bool                   `json:"success"`
	Errors   []cloudflareAPIMessage `json:"errors"`
	Messages []cloudflareAPIMessage `json:"messages"`
	Result   []cloudflareDNSRecord  `json:"result"`
}

type cloudflareRecordResponse struct {
	Success  bool                   `json:"success"`
	Errors   []cloudflareAPIMessage `json:"errors"`
	Messages []cloudflareAPIMessage `json:"messages"`
	Result   cloudflareDNSRecord    `json:"result"`
}

func main() {
	var apiKey, myIPv4, myIPv6, domainName, recordName, provider, cloudflareZoneID string
	var cloudflareProxied optionalBoolFlag
	var ttl int

	flag.CommandLine.Init(toolName, flag.ExitOnError)
	flag.StringVar(&provider, "provider", providerGandi, "DNS provider: gandi or cloudflare")
	flag.StringVar(&apiKey, "api_key", "", "API key or token for the selected DNS provider")
	flag.StringVar(&myIPv4, "ip4", getMyIPv4(), "IPv4 address to set. Default is to get your public IP from Akamai")
	flag.StringVar(&myIPv6, "ip6", getMyIPv6(), "IPv6 address to set. Default is to get your public IP from Akamai")
	flag.StringVar(&domainName, "domain_name", "", "Domain name")
	flag.StringVar(&recordName, "record_name", "", "Record name")
	flag.IntVar(&ttl, "ttl", defaultTTL, "TTL for the DNS record in seconds")
	flag.StringVar(&cloudflareZoneID, "cloudflare_zone_id", "", "Cloudflare zone ID. Required when provider is cloudflare")
	flag.Var(&cloudflareProxied, "cloudflare_proxied", "Cloudflare proxy setting. Omit to preserve existing records; pass true or false to set it")
	flag.Parse()

	if apiKey == "" {
		log.Fatal("api_key can't be empty")
	}
	if domainName == "" {
		log.Fatal("domain_name can't be empty")
	}
	if recordName == "" {
		log.Fatal("record_name can't be empty")
	}
	if provider == providerCloudflare && cloudflareZoneID == "" {
		log.Fatal("cloudflare_zone_id can't be empty when provider is cloudflare")
	}

	if myIPv4 != "" {
		recordSet := recordSet{
			Type:   "A",
			TTL:    ttl,
			Values: []string{myIPv4},
		}
		updateRecord(provider, apiKey, domainName, recordName, cloudflareZoneID, cloudflareProxied.valuePtr(), recordSet)
	} else {
		log.Print("No IPv4 was found or set")
	}

	if myIPv6 != "" {
		recordSet := recordSet{
			Type:   "AAAA",
			TTL:    ttl,
			Values: []string{myIPv6},
		}
		updateRecord(provider, apiKey, domainName, recordName, cloudflareZoneID, cloudflareProxied.valuePtr(), recordSet)
	} else {
		log.Print("No IPv6 was found or set")
	}
}

func getMyIP(whatsMyIPURL string) string {
	resp, err := http.Get(whatsMyIPURL)
	if err != nil {
		log.Print(err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return ""
	}
	return string(body)
}

func getMyIPv4() string {
	return getMyIP(whatsMyIPv4URL)
}

func getMyIPv6() string {
	return getMyIP(whatsMyIPv6URL)
}

func updateRecord(provider string, apiKey string, domainName string, recordName string, cloudflareZoneID string, cloudflareProxied *bool, recordSet recordSet) {
	switch provider {
	case providerGandi:
		updateGandiRecord(apiKey, domainName, recordName, recordSet)
	case providerCloudflare:
		updateCloudflareRecord(apiKey, cloudflareZoneID, domainName, recordName, cloudflareProxied, recordSet)
	default:
		log.Fatalf("unsupported provider %q", provider)
	}
}

func updateGandiRecord(apiKey string, domainName string, recordName string, recordSet recordSet) {
	log.Printf("Updating Gandi %s.%s to %+v", recordName, domainName, recordSet)
	endpoint := fmt.Sprintf("%s/domains/%s/records/%s/%s", gandiAPIBaseURL, url.PathEscape(domainName), url.PathEscape(recordName), url.PathEscape(recordSet.Type))

	gandiRecordSet := gandiRecordSet{
		Type:   recordSet.Type,
		TTL:    recordSet.TTL,
		Values: recordSet.Values,
	}
	recordBytes, err := json.Marshal(gandiRecordSet)
	if err != nil {
		log.Fatal(err)
	}
	requestBody := bytes.NewReader(recordBytes)

	client := &http.Client{}
	req, err := http.NewRequest("PUT", endpoint, requestBody)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Apikey %s", apiKey))
	req.Header.Add("Content-Type", "application/json")

	responseBody := doRequest(client, req)
	log.Printf("Response: %s", string(responseBody))
}

func updateCloudflareRecord(apiToken string, zoneID string, domainName string, recordName string, proxied *bool, recordSet recordSet) {
	if len(recordSet.Values) != 1 {
		log.Fatalf("Cloudflare update expected exactly one value for %s records, got %d", recordSet.Type, len(recordSet.Values))
	}

	client := &http.Client{}
	dnsName := cloudflareRecordName(domainName, recordName)
	payload := cloudflareRecordPayload{
		Type:    recordSet.Type,
		Name:    dnsName,
		Content: recordSet.Values[0],
		TTL:     recordSet.TTL,
		Proxied: proxied,
	}

	records := findCloudflareRecords(client, apiToken, zoneID, recordSet.Type, dnsName)
	switch len(records) {
	case 0:
		log.Printf("Creating Cloudflare %s record %s to %s", recordSet.Type, dnsName, recordSet.Values[0])
		createCloudflareRecord(client, apiToken, zoneID, payload)
	case 1:
		log.Printf("Updating Cloudflare %s record %s to %s", recordSet.Type, dnsName, recordSet.Values[0])
		updateCloudflareDNSRecord(client, apiToken, zoneID, records[0].ID, payload)
	default:
		log.Fatalf("found %d Cloudflare %s records named %s; refusing to choose one", len(records), recordSet.Type, dnsName)
	}
}

func findCloudflareRecords(client *http.Client, apiToken string, zoneID string, recordType string, recordName string) []cloudflareDNSRecord {
	endpoint, err := url.Parse(fmt.Sprintf("%s/zones/%s/dns_records", cloudflareAPIBaseURL, url.PathEscape(zoneID)))
	if err != nil {
		log.Fatal(err)
	}
	query := endpoint.Query()
	query.Set("type", recordType)
	query.Set("name", recordName)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		log.Fatal(err)
	}
	addCloudflareHeaders(req, apiToken)

	responseBody := doRequest(client, req)
	var response cloudflareListRecordsResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		log.Fatal(err)
	}
	if !response.Success {
		log.Fatalf("Cloudflare failed to list records: %s", cloudflareMessages(response.Errors))
	}

	return response.Result
}

func createCloudflareRecord(client *http.Client, apiToken string, zoneID string, payload cloudflareRecordPayload) {
	endpoint := fmt.Sprintf("%s/zones/%s/dns_records", cloudflareAPIBaseURL, url.PathEscape(zoneID))
	requestBody := cloudflareRequestBody(payload)

	req, err := http.NewRequest("POST", endpoint, requestBody)
	if err != nil {
		log.Fatal(err)
	}
	addCloudflareHeaders(req, apiToken)

	responseBody := doRequest(client, req)
	var response cloudflareRecordResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		log.Fatal(err)
	}
	if !response.Success {
		log.Fatalf("Cloudflare failed to create record: %s", cloudflareMessages(response.Errors))
	}
	log.Printf("Response: %s", string(responseBody))
}

func updateCloudflareDNSRecord(client *http.Client, apiToken string, zoneID string, recordID string, payload cloudflareRecordPayload) {
	endpoint := fmt.Sprintf("%s/zones/%s/dns_records/%s", cloudflareAPIBaseURL, url.PathEscape(zoneID), url.PathEscape(recordID))
	requestBody := cloudflareRequestBody(payload)

	req, err := http.NewRequest("PATCH", endpoint, requestBody)
	if err != nil {
		log.Fatal(err)
	}
	addCloudflareHeaders(req, apiToken)

	responseBody := doRequest(client, req)
	var response cloudflareRecordResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		log.Fatal(err)
	}
	if !response.Success {
		log.Fatalf("Cloudflare failed to update record: %s", cloudflareMessages(response.Errors))
	}
	log.Printf("Response: %s", string(responseBody))
}

func cloudflareRequestBody(payload cloudflareRecordPayload) *bytes.Reader {
	recordBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatal(err)
	}
	return bytes.NewReader(recordBytes)
}

func addCloudflareHeaders(req *http.Request, apiToken string) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiToken))
	req.Header.Add("Content-Type", "application/json")
}

func cloudflareRecordName(domainName string, recordName string) string {
	domainName = strings.TrimSuffix(domainName, ".")
	recordName = strings.TrimSuffix(recordName, ".")
	if recordName == "@" || strings.EqualFold(recordName, domainName) {
		return domainName
	}
	if strings.HasSuffix(strings.ToLower(recordName), "."+strings.ToLower(domainName)) {
		return recordName
	}
	return fmt.Sprintf("%s.%s", recordName, domainName)
}

func cloudflareMessages(messages []cloudflareAPIMessage) string {
	if len(messages) == 0 {
		return "no error details returned"
	}

	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		if message.Code == 0 {
			parts = append(parts, message.Message)
		} else {
			parts = append(parts, fmt.Sprintf("%d: %s", message.Code, message.Message))
		}
	}
	return strings.Join(parts, "; ")
}

func doRequest(client *http.Client, req *http.Request) []byte {
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Fatalf("%s %s returned %s: %s", req.Method, req.URL.String(), resp.Status, string(responseBody))
	}
	return responseBody
}
