package orden

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

type gmailHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type gmailPart struct {
	Mime    string        `json:"mimeType"`
	Headers []gmailHeader `json:"headers"`
	Body    struct {
		Data string `json:"data"`
	} `json:"body"`
	Parts []gmailPart `json:"parts"`
}
type gmailMessage struct {
	ID           string    `json:"id"`
	InternalDate string    `json:"internalDate"`
	Payload      gmailPart `json:"payload"`
}
type Purchase struct{ ID, Amount, Date, LastFour string }

func header(p gmailPart, name string) string {
	for _, h := range p.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}
func plainHTML(input string) string {
	t := html.NewTokenizer(strings.NewReader(input))
	var b strings.Builder
	skip := 0
	for {
		kind := t.Next()
		if kind == html.ErrorToken {
			break
		}
		switch kind {
		case html.StartTagToken:
			tag, _ := t.TagName()
			name := string(tag)
			if name == "script" || name == "style" {
				skip++
			}
			if skip == 0 {
				b.WriteByte(' ')
			}
		case html.EndTagToken:
			tag, _ := t.TagName()
			name := string(tag)
			if (name == "script" || name == "style") && skip > 0 {
				skip--
			}
			b.WriteByte(' ')
		case html.TextToken:
			if skip == 0 {
				b.Write(t.Text())
				b.WriteByte(' ')
			}
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
func mailBody(p gmailPart, depth int) string {
	if depth > 8 {
		return ""
	}
	if p.Mime == "text/html" || p.Mime == "text/plain" {
		if len(p.Body.Data) > 1400000 {
			return ""
		}
		data, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(p.Body.Data, "="))
		if err != nil {
			return ""
		}
		// Gmail removes transfer encoding, but MIME text may still use a legacy
		// charset. Decode it before matching accented purchase field names.
		if contentType := header(p, "Content-Type"); contentType != "" {
			_, params, err := mime.ParseMediaType(contentType)
			if err != nil {
				return ""
			}
			if label := params["charset"]; label != "" {
				reader, err := charset.NewReaderLabel(label, bytes.NewReader(data))
				if err != nil {
					return ""
				}
				data, err = io.ReadAll(io.LimitReader(reader, (4<<20)+1))
				if err != nil || len(data) > 4<<20 {
					return ""
				}
			}
		}
		if p.Mime == "text/html" {
			return plainHTML(string(data))
		}
		return strings.Join(strings.Fields(string(data)), " ")
	} // Prefer HTML; don't concatenate alternative bodies and duplicate fields.
	for _, part := range p.Parts {
		if part.Mime == "text/html" {
			if body := mailBody(part, depth+1); body != "" {
				return body
			}
		}
	}
	for _, part := range p.Parts {
		if body := mailBody(part, depth+1); body != "" {
			return body
		}
	}
	return ""
}

var amountRE = regexp.MustCompile(`(?i)Monto pagado\s*\$\s*([0-9]{1,3}(?:\.[0-9]{3})*|[0-9]{1,12})(?:\s|$)`)
var cardRE = regexp.MustCompile(`(?i)Últimos 4 dígitos de la tarjeta\s*([0-9]{4})(?:\s|$)`)
var dateRE = regexp.MustCompile(`(?i)Fecha y hora\s*([0-9]{2}/[0-9]{2}/[0-9]{4})\s*-\s*[0-9]{2}:[0-9]{2}`)
var purchaseRE = regexp.MustCompile(`(?i)Nr\.? identificador de la compra\s*([a-z0-9_-]{4,80})(?:\s|$)`)

// Gmail can report the aligned organizational domain rather than the exact
// sender subdomain. Keep this an explicit allowlist, not a suffix match.
var dmarcRE = regexp.MustCompile(`(?i)(?:^|;)\s*dmarc=pass\b[^;]*\bheader\.from=(?:mail\.)?machbank\.cl(?:\s|;|$)`)

func parseMACH(m gmailMessage) (Purchase, bool) {
	p, reason := parseMACHReason(m)
	return p, reason == ""
}

// Reasons are fixed, non-sensitive codes. Never return header/body fragments.
func parseMACHReason(m gmailMessage) (Purchase, string) {
	var p Purchase
	from, err := mail.ParseAddress(header(m.Payload, "From"))
	if err != nil || !strings.EqualFold(from.Address, "contacto@mail.machbank.cl") {
		return p, "sender"
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(header(m.Payload, "Subject"))
	if err != nil {
		return p, "subject"
	}
	if subject != "Has hecho una compra con tu Tarjeta de Crédito MACHBANK" {
		return p, "subject"
	}
	// Only the receiving Gmail server's authentication result is accepted.
	auth := header(m.Payload, "Authentication-Results")
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(auth)), "mx.google.com;") || !dmarcRE.MatchString(auth) {
		return p, "authentication"
	}
	body := mailBody(m.Payload, 0)
	if body == "" {
		return p, "body"
	}
	if strings.Contains(strings.ToUpper(body), "USD") || strings.Contains(strings.ToUpper(body), "US$") {
		return p, "currency"
	}
	amount, card, date, id := amountRE.FindAllStringSubmatch(body, -1), cardRE.FindAllStringSubmatch(body, -1), dateRE.FindAllStringSubmatch(body, -1), purchaseRE.FindAllStringSubmatch(body, -1)
	if len(amount) != 1 {
		return p, "amount"
	}
	if len(card) != 1 {
		return p, "card"
	}
	if len(date) != 1 {
		return p, "date"
	}
	if len(id) != 1 {
		return p, "purchase_id"
	}
	d, err := time.Parse("02/01/2006", date[0][1])
	if err != nil || d.After(time.Now().Add(48*time.Hour)) || d.Year() < 2020 {
		return p, "date"
	}
	p = Purchase{ID: id[0][1], Amount: strings.ReplaceAll(amount[0][1], ".", ""), Date: d.Format("2006-01-02"), LastFour: card[0][1]}
	if !validMoney(p.Amount, "CLP", true) {
		return Purchase{}, "amount"
	}
	return p, ""
}
