# Dynamic DNS Update

Updates A and AAAA records via a DNS provider API, by default using your public IP on the Internet.
This is an easy alternative to dynamic DNS services.

Gandi and Cloudflare are the two supported providers:

- [Gandi LiveDNS](https://doc.livedns.gandi.net/)
- [Cloudflare DNS](https://developers.cloudflare.com/api/resources/dns/subresources/records/)

## Install

```
go install
```

## Gandi Usage

```
$ dynamic-dns-update -help
```

```
Usage of Dynamic DNS Update:
  -api_key string
    	API key or token for the selected DNS provider
  -cloudflare_proxied
    	Cloudflare proxy setting. Omit to preserve existing records; pass true or false to set it
  -domain_name string
    	Domain name
  -ip4 string
      IPv4 address to set. Default is to get your public IP from Akamai (default "YOUR_IP")
  -ip6 string
      IPv6 address to set. Default is to get your public IP from Akamai (default "YOUR_IP")
  -provider string
    	DNS provider: gandi or cloudflare (default "gandi")
  -record_name string
    	Record name
  -ttl int
    	TTL for the DNS record in seconds (default 300)
```

Example:

```
dynamic-dns-update \
  -provider gandi \
  -api_key "$GANDI_API_KEY" \
  -domain_name example.com \
  -record_name home
```

## Cloudflare Usage

Cloudflare requires an API token with zone read access and DNS write access. The provider looks up the zone ID from `-domain_name`.

```
dynamic-dns-update \
  -provider cloudflare \
  -api_key "$CLOUDFLARE_API_TOKEN" \
  -domain_name example.com \
  -record_name home
```

The Cloudflare provider looks up an exact matching record by type and full name, updates it when one exists, and creates it when none exists. For `-record_name home` and `-domain_name example.com`, the Cloudflare record name is `home.example.com`. Use `-record_name @` for the zone apex.

By default, Cloudflare updates preserve the existing proxy setting when a record already exists. Pass `-cloudflare_proxied=true` or `-cloudflare_proxied=false` to explicitly set the Cloudflare proxy status.
