package alipay

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"html"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	MethodPagePay       = "alipay.trade.page.pay"
	ProductCodePagePay  = "FAST_INSTANT_TRADE_PAY"
	TradeStatusSuccess  = "TRADE_SUCCESS"
	TradeStatusFinished = "TRADE_FINISHED"
)

type Config struct {
	AppID      string
	GatewayURL string
	PrivateKey string
	PublicKey  string
	NotifyURL  string
	ReturnURL  string
}

type Client struct {
	cfg        Config
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	now        func() time.Time
}

type PagePayRequest struct {
	OutTradeNo  string
	Subject     string
	TotalAmount string
}

type PagePayResponse struct {
	GatewayURL string
	FormHTML   string
	Params     url.Values
}

type Notify struct {
	AppID       string
	OutTradeNo  string
	TradeNo     string
	TotalAmount string
	TradeStatus string
	Raw         url.Values
}

func NewClient(cfg Config) (*Client, error) {
	if cfg.AppID == "" {
		return nil, errors.New("alipay app id is required")
	}
	if cfg.GatewayURL == "" {
		return nil, errors.New("alipay gateway URL is required")
	}

	privateKey, err := parsePrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, err
	}
	publicKey, err := parsePublicKey(cfg.PublicKey)
	if err != nil {
		return nil, err
	}

	return &Client{
		cfg:        cfg,
		privateKey: privateKey,
		publicKey:  publicKey,
		now:        time.Now,
	}, nil
}

func (c *Client) PagePay(req PagePayRequest) (PagePayResponse, error) {
	bizContent, err := json.Marshal(map[string]string{
		"out_trade_no": req.OutTradeNo,
		"product_code": ProductCodePagePay,
		"total_amount": req.TotalAmount,
		"subject":      req.Subject,
	})
	if err != nil {
		return PagePayResponse{}, err
	}

	params := url.Values{}
	params.Set("app_id", c.cfg.AppID)
	params.Set("method", MethodPagePay)
	params.Set("charset", "utf-8")
	params.Set("sign_type", "RSA2")
	params.Set("timestamp", c.now().Format("2006-01-02 15:04:05"))
	params.Set("version", "1.0")
	params.Set("notify_url", c.cfg.NotifyURL)
	params.Set("return_url", c.cfg.ReturnURL)
	params.Set("biz_content", string(bizContent))

	sign, err := c.sign(params)
	if err != nil {
		return PagePayResponse{}, err
	}
	params.Set("sign", sign)

	return PagePayResponse{
		GatewayURL: c.cfg.GatewayURL,
		FormHTML:   buildForm(c.cfg.GatewayURL, params),
		Params:     params,
	}, nil
}

func (c *Client) ParseNotify(values url.Values) (Notify, error) {
	if !c.Verify(values) {
		return Notify{}, errors.New("invalid alipay signature")
	}

	return Notify{
		AppID:       values.Get("app_id"),
		OutTradeNo:  values.Get("out_trade_no"),
		TradeNo:     values.Get("trade_no"),
		TotalAmount: values.Get("total_amount"),
		TradeStatus: values.Get("trade_status"),
		Raw:         values,
	}, nil
}

func (c *Client) Verify(values url.Values) bool {
	signature := values.Get("sign")
	if signature == "" {
		return false
	}
	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}

	hash := sha256.Sum256([]byte(canonicalString(values)))
	return rsa.VerifyPKCS1v15(c.publicKey, crypto.SHA256, hash[:], signatureBytes) == nil
}

func (c *Client) sign(values url.Values) (string, error) {
	hash := sha256.Sum256([]byte(canonicalString(values)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func canonicalString(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key == "sign" || key == "sign_type" || values.Get(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	return strings.Join(parts, "&")
}

func buildForm(action string, values url.Values) string {
	var b strings.Builder
	b.WriteString(`<form method="post" action="`)
	b.WriteString(html.EscapeString(action))
	b.WriteString(`">`)
	for key, values := range values {
		if len(values) == 0 {
			continue
		}
		b.WriteString(`<input type="hidden" name="`)
		b.WriteString(html.EscapeString(key))
		b.WriteString(`" value="`)
		b.WriteString(html.EscapeString(values[0]))
		b.WriteString(`">`)
	}
	b.WriteString(`<button type="submit">Pay with Alipay</button></form>`)
	return b.String()
}

func parsePrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(normalizePEM(raw, "PRIVATE KEY")))
	if block == nil {
		return nil, errors.New("invalid alipay private key PEM")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("alipay private key is not RSA")
		}
		return rsaKey, nil
	}

	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return rsaKey, nil
}

func parsePublicKey(raw string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(normalizePEM(raw, "PUBLIC KEY")))
	if block == nil {
		return nil, errors.New("invalid alipay public key PEM")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("alipay public key is not RSA")
		}
		return rsaKey, nil
	}

	cert, certErr := x509.ParseCertificate(block.Bytes)
	if certErr == nil {
		rsaKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("alipay certificate public key is not RSA")
		}
		return rsaKey, nil
	}
	return nil, err
}

func normalizePEM(raw string, typ string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, `\n`, "\n"))
	if strings.Contains(raw, "BEGIN ") {
		return raw
	}
	return fmt.Sprintf("-----BEGIN %s-----\n%s\n-----END %s-----", typ, raw, typ)
}

func AmountFromCents(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func CentsFromAmount(amount string) (int64, error) {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return 0, errors.New("amount is required")
	}

	whole, fraction, ok := strings.Cut(amount, ".")
	if !ok {
		fraction = "00"
	}
	if len(fraction) == 1 {
		fraction += "0"
	}
	if len(fraction) != 2 {
		return 0, errors.New("amount must have at most two decimal places")
	}

	wholeCents, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, err
	}
	fractionCents, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, err
	}
	return wholeCents*100 + fractionCents, nil
}
