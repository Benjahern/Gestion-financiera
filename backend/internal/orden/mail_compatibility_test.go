package orden

import (
	"encoding/base64"
	"mime"
	"strings"
	"testing"
)

func TestMACHAuthenticationDomains(t *testing.T) {
	for _, tc := range []struct {
		name, auth string
		accepted   bool
	}{
		{"subdomain", "mx.google.com; dmarc=pass header.from=mail.machbank.cl", true},
		{"organizational domain", "mx.google.com; dkim=pass header.i=@mail.machbank.cl; dmarc=pass (p=NONE sp=NONE dis=NONE) header.from=machbank.cl", true},
		{"folded result", "mx.google.com;\r\n dmarc=pass (p=NONE)\r\n header.from=machbank.cl; spf=pass", true},
		{"failed DMARC", "mx.google.com; dmarc=fail header.from=machbank.cl", false},
		{"untrusted verifier", "other.example; dmarc=pass header.from=machbank.cl", false},
		{"lookalike suffix", "mx.google.com; dmarc=pass header.from=machbank.cl.example", false},
		{"lookalike prefix", "mx.google.com; dmarc=pass header.from=notmachbank.cl", false},
		{"unapproved subdomain", "mx.google.com; dmarc=pass header.from=other.machbank.cl", false},
		{"separate result", "mx.google.com; dmarc=pass header.from=other.example; spf=pass header.from=machbank.cl", false},
		{"not a DMARC result", "mx.google.com; notdmarc=pass header.from=machbank.cl", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testMail()
			for i := range m.Payload.Headers {
				if m.Payload.Headers[i].Name == "Authentication-Results" {
					m.Payload.Headers[i].Value = tc.auth
				}
			}
			if _, ok := parseMACH(m); ok != tc.accepted {
				t.Fatalf("accepted=%v, want %v", ok, tc.accepted)
			}
		})
	}
}

func TestMACHLegacyCharset(t *testing.T) {
	for _, mimeType := range []string{"text/html", "text/plain"} {
		t.Run(mimeType, func(t *testing.T) {
			m := testMail()
			body, _ := base64.RawURLEncoding.DecodeString(m.Payload.Body.Data)
			if mimeType == "text/plain" {
				body = []byte(plainHTML(string(body)))
			}
			latin := make([]byte, 0, len(body))
			for _, r := range string(body) {
				if r > 255 {
					t.Fatal("fixture is not Latin-1")
				}
				latin = append(latin, byte(r))
			}
			headers := m.Payload.Headers
			for i := range headers {
				if headers[i].Name == "Subject" {
					headers[i].Value = mime.QEncoding.Encode("utf-8", headers[i].Value)
				}
			}
			part := gmailPart{Mime: mimeType, Headers: []gmailHeader{{"Content-Type", mimeType + "; charset=iso-8859-1"}}}
			part.Body.Data = base64.URLEncoding.EncodeToString(latin)
			m.Payload = gmailPart{Mime: "multipart/alternative", Headers: headers, Parts: []gmailPart{part}}
			if p, ok := parseMACH(m); !ok || p.Amount != "4000" || p.LastFour != "4100" {
				t.Fatal("legacy charset purchase was not decoded")
			}
			m.Payload.Parts[0].Headers[0].Value = mimeType + "; charset=unknown-example-charset"
			if _, ok := parseMACH(m); ok {
				t.Fatal("unknown charset accepted")
			}
		})
	}
}

func TestMACHStillRequiresExactSender(t *testing.T) {
	m := testMail()
	for i := range m.Payload.Headers {
		if m.Payload.Headers[i].Name == "From" {
			m.Payload.Headers[i].Value = "someone@example.com"
		}
		if m.Payload.Headers[i].Name == "Authentication-Results" {
			m.Payload.Headers[i].Value = "mx.google.com; dmarc=pass header.from=machbank.cl"
		}
	}
	if _, ok := parseMACH(m); ok {
		t.Fatal("unexpected sender accepted")
	}
	// ARC results are not a replacement for Gmail's receiving-server result.
	m = testMail()
	for i := range m.Payload.Headers {
		if strings.EqualFold(m.Payload.Headers[i].Name, "Authentication-Results") {
			m.Payload.Headers[i].Name = "ARC-Authentication-Results"
		}
	}
	if _, ok := parseMACH(m); ok {
		t.Fatal("ARC-only authentication accepted")
	}
}

func TestMACHDiagnosticReasons(t *testing.T) {
	for _, tc := range []struct{ field, replacement, reason string }{
		{"Monto pagado", "Unknown amount field", "amount"},
		{"Últimos 4 dígitos de la tarjeta", "Unknown card field", "card"},
		{"Fecha y hora", "Unknown date field", "date"},
		{"Nr. identificador de la compra", "Unknown identifier field", "purchase_id"},
		{"$4.000", "US$4.000 USD", "currency"},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			m := testMail()
			body, _ := base64.RawURLEncoding.DecodeString(m.Payload.Body.Data)
			m.Payload.Body.Data = base64.RawURLEncoding.EncodeToString([]byte(strings.ReplaceAll(string(body), tc.field, tc.replacement)))
			if _, reason := parseMACHReason(m); reason != tc.reason {
				t.Fatalf("got reason %q, want fixed code %q", reason, tc.reason)
			}
		})
	}
	m := testMail()
	m.Payload.Body.Data = ""
	if _, reason := parseMACHReason(m); reason != "body" {
		t.Fatalf("missing body reason: %q", reason)
	}
}
