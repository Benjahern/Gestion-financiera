package orden

import (
	"encoding/hex"
	"errors"
	"net/mail"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	OwnerEmail         string
	ProxySecret        string
	DatabaseURL        string
	Origin             string
	Address            string
	SecureCookies      bool
	GoogleClientID     string
	GoogleClientSecret string
	TokenEncryptionKey string
}

func LoadConfig() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), Origin: strings.TrimRight(os.Getenv("APP_ORIGIN"), "/"), Address: os.Getenv("HTTP_ADDR"), SecureCookies: true}
	c.OwnerEmail = strings.ToLower(strings.TrimSpace(os.Getenv("OWNER_EMAIL")))
	address, emailErr := mail.ParseAddress(c.OwnerEmail)
	if emailErr != nil || address.Address != c.OwnerEmail || len(c.OwnerEmail) > 254 {
		return c, errors.New("OWNER_EMAIL debe contener únicamente el correo del propietario")
	}
	c.ProxySecret = os.Getenv("INTERNAL_PROXY_SECRET")
	c.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	c.GoogleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	c.TokenEncryptionKey = os.Getenv("TOKEN_ENCRYPTION_KEY")
	if c.GoogleClientID != "" || c.GoogleClientSecret != "" {
		if !c.gmailEnabled() {
			return c, errors.New("Gmail requiere GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET y TOKEN_ENCRYPTION_KEY de 32 bytes en base64")
		}
	}
	if c.Address == "" {
		c.Address = ":8080"
	}
	if c.DatabaseURL == "" {
		if os.Getenv("PGHOST") == "" || os.Getenv("PGPASSWORD") == "" {
			return c, errors.New("DATABASE_URL o configuración PGHOST/PGPASSWORD es obligatoria")
		}
		u := url.URL{Scheme: "postgres", Host: os.Getenv("PGHOST") + ":5432", Path: "/" + os.Getenv("PGDATABASE"), User: url.UserPassword(os.Getenv("PGUSER"), os.Getenv("PGPASSWORD"))}
		q := u.Query()
		q.Set("sslmode", "disable")
		u.RawQuery = q.Encode()
		c.DatabaseURL = u.String()
	}
	u, err := url.Parse(c.Origin)
	if err != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return c, errors.New("APP_ORIGIN debe ser el origen público, sin ruta")
	}
	if os.Getenv("APP_ENV") == "development" {
		if u.Scheme != "http" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1") {
			return c, errors.New("desarrollo solo permite un origen HTTP local")
		}
		c.SecureCookies = false
	} else if u.Scheme != "https" {
		return c, errors.New("producción requiere APP_ORIGIN con HTTPS")
	}
	proxyKey, proxyKeyErr := hex.DecodeString(c.ProxySecret)
	if c.SecureCookies && (len(proxyKey) != 32 || proxyKeyErr != nil || c.ProxySecret != strings.ToLower(c.ProxySecret)) {
		return c, errors.New("producción requiere INTERNAL_PROXY_SECRET hexadecimal de 64 caracteres")
	}
	return c, nil
}
