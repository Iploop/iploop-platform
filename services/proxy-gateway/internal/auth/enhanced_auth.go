package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// EnhancedProxyAuth supports multiple authentication methods and parameters
type EnhancedProxyAuth struct {
	Customer     *Customer
	Method       string // "basic", "token", "ip", "signature"
	
	// Geographic targeting
	Country      string
	City         string
	Region       string
	ASN          int
	
	// Session management  
	SessionID    string
	SessionType  string // "sticky", "rotating", "per-request"
	Lifetime     time.Duration
	
	// Rotation control
	RotateMode   string // "request", "time", "manual", "ip-change"
	RotateInterval time.Duration
	
	// Protocol preferences
	Profile      string // "chrome-win", "firefox-mac", "mobile-ios", "custom"
	UserAgent    string
	
	// Quality requirements
	MinSpeed     int // Mbps
	MaxLatency   int // milliseconds
	Protocol     string // "http", "https", "socks5"
	
	// Advanced features
	Headers      map[string]string
	WhitelistIPs []string
	Debug        bool
	
	OriginalAuth string
}

// Enhanced parameter parsing supporting multiple formats:
// Basic: user:pass-country-US-city-Miami-session-sticky30m
// Advanced: user:pass-geo-US-NY-ASN12345-profile-chrome-rotate-5m-speed-50
// Token: token:abc123xyz-country-US-session-sticky-lifetime-60m
func (a *Authenticator) ParseEnhancedAuth(authHeader string, clientIP string) (*EnhancedProxyAuth, error) {
	auth := &EnhancedProxyAuth{
		Method:       "basic",
		SessionType:  "sticky",
		Lifetime:     30 * time.Minute,
		RotateMode:   "manual",
		Profile:      "chrome-win",
		MinSpeed:     10,
		MaxLatency:   1000,
		Protocol:     "http",
		Headers:      make(map[string]string),
		OriginalAuth: authHeader,
	}
	
	// Decode Base64 if needed
	if strings.HasPrefix(authHeader, "Basic ") {
		authHeader = authHeader[6:]
		decoded, err := base64.StdEncoding.DecodeString(authHeader)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 encoding")
		}
		authHeader = string(decoded)
	}
	
	// Check for IP whitelist authentication
	if strings.HasPrefix(authHeader, "ip:") {
		return a.parseIPAuth(authHeader[3:], clientIP, auth)
	}
	
	// Check for token authentication  
	if strings.HasPrefix(authHeader, "token:") {
		return a.parseTokenAuth(authHeader[6:], auth)
	}
	
	// Check for signature authentication
	if strings.HasPrefix(authHeader, "sig:") {
		return a.parseSignatureAuth(authHeader[4:], auth)
	}
	
	// Default: Basic authentication
	return a.parseBasicAuth(authHeader, auth)
}

func (a *Authenticator) parseBasicAuth(authStr string, auth *EnhancedProxyAuth) (*EnhancedProxyAuth, error) {
	parts := strings.SplitN(authStr, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid basic auth format")
	}
	
	customerID := parts[0]
	keyAndParams := parts[1]
	
	// Split parameters by dash
	paramParts := strings.Split(keyAndParams, "-")
	apiKey := paramParts[0]
	
	// Authenticate customer
	customer, err := a.authenticateCustomer(customerID, apiKey)
	if err != nil {
		return nil, err
	}
	auth.Customer = customer
	
	// Parse parameters
	return a.parseParameters(paramParts[1:], auth)
}

func (a *Authenticator) parseTokenAuth(tokenStr string, auth *EnhancedProxyAuth) (*EnhancedProxyAuth, error) {
	auth.Method = "token"
	
	parts := strings.Split(tokenStr, "-")
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid token format")
	}
	
	token := parts[0]
	
	// Validate token against database
	customer, err := a.validateToken(token)
	if err != nil {
		return nil, err
	}
	auth.Customer = customer
	
	// Parse additional parameters
	return a.parseParameters(parts[1:], auth)
}

func (a *Authenticator) parseIPAuth(userID string, clientIP string, auth *EnhancedProxyAuth) (*EnhancedProxyAuth, error) {
	auth.Method = "ip"
	
	// Check if IP is whitelisted for this user
	customer, err := a.validateIPWhitelist(userID, clientIP)
	if err != nil {
		return nil, err
	}
	auth.Customer = customer
	
	return auth, nil
}

func (a *Authenticator) parseSignatureAuth(sigStr string, auth *EnhancedProxyAuth) (*EnhancedProxyAuth, error) {
	auth.Method = "signature"
	
	// Parse: user_timestamp_signature-params
	parts := strings.Split(sigStr, "_")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid signature format")
	}
	
	userID := parts[0]
	timestampStr := parts[1]
	signature := parts[2]
	
	// Validate timestamp (max 5 minutes old)
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp")
	}
	
	if time.Unix(timestamp, 0).Add(5*time.Minute).Before(time.Now()) {
		return nil, fmt.Errorf("signature expired")
	}
	
	// Validate signature
	customer, err := a.validateSignature(userID, timestampStr, signature)
	if err != nil {
		return nil, err
	}
	auth.Customer = customer
	
	// Parse remaining parameters
	if len(parts) > 3 {
		paramStr := strings.Join(parts[3:], "_")
		paramParts := strings.Split(paramStr, "-")
		return a.parseParameters(paramParts, auth)
	}
	
	return auth, nil
}

func (a *Authenticator) parseParameters(params []string, auth *EnhancedProxyAuth) (*EnhancedProxyAuth, error) {
	for i := 0; i < len(params)-1; i += 2 {
		if i+1 >= len(params) {
			break
		}
		
		key := params[i]
		value := params[i+1]
		
		switch key {
		case "country", "geo":
			auth.Country = strings.ToUpper(value)
		case "city":
			auth.City = strings.ToLower(value)
		case "region", "state":
			auth.Region = strings.ToUpper(value)
		case "asn":
			if asn, err := strconv.Atoi(value); err == nil {
				auth.ASN = asn
			}
		case "session", "sess":
			auth.SessionID = value
		case "sesstype", "stype":
			auth.SessionType = value // sticky, rotating, per-request
		case "lifetime", "life", "ttl":
			auth.Lifetime = a.parseDuration(value)
		case "rotate", "rot":
			auth.RotateMode = value
		case "rotateint", "rint":
			auth.RotateInterval = a.parseDuration(value)
		case "profile", "prof":
			auth.Profile = value
		case "ua", "useragent":
			auth.UserAgent = value
		case "speed", "minspeed":
			if speed, err := strconv.Atoi(value); err == nil {
				auth.MinSpeed = speed
			}
		case "latency", "maxlat":
			if lat, err := strconv.Atoi(value); err == nil {
				auth.MaxLatency = lat
			}
		case "proto", "protocol":
			auth.Protocol = value
		case "debug":
			auth.Debug = value == "1" || value == "true"
		case "header":
			// Format: header-Name:Value
			if strings.Contains(value, ":") {
				parts := strings.SplitN(value, ":", 2)
				auth.Headers[parts[0]] = parts[1]
			}
		}
	}
	
	// Generate session ID if not provided
	if auth.SessionID == "" {
		auth.SessionID = a.generateSessionID(auth.Customer.ID)
	}
	
	return auth, nil
}

func (a *Authenticator) parseDuration(durationStr string) time.Duration {
	// Parse: 30m, 1h, 60s, 120 (seconds)
	re := regexp.MustCompile(`^(\d+)([smhd]?)$`)
	matches := re.FindStringSubmatch(durationStr)
	
	if len(matches) != 3 {
		return 30 * time.Minute // default
	}
	
	value, _ := strconv.Atoi(matches[1])
	unit := matches[2]
	
	switch unit {
	case "s", "":
		return time.Duration(value) * time.Second
	case "m":
		return time.Duration(value) * time.Minute
	case "h":
		return time.Duration(value) * time.Hour
	case "d":
		return time.Duration(value) * 24 * time.Hour
	default:
		return 30 * time.Minute
	}
}

func (a *Authenticator) validateToken(token string) (*Customer, error) {
	// Check token in database
	query := `
		SELECT u.id, u.email, COALESCE(up.gb_balance, 0), p.name
		FROM tokens t
		JOIN users u ON t.user_id = u.id  
		LEFT JOIN user_plans up ON u.id = up.user_id AND up.status = 'active'
		LEFT JOIN plans p ON up.plan_id = p.id
		WHERE t.token_hash = $1 AND t.expires_at > NOW() AND t.is_active = true
	`
	
	hasher := sha256.New()
	hasher.Write([]byte(token))
	tokenHash := hex.EncodeToString(hasher.Sum(nil))
	
	var customer Customer
	err := a.db.QueryRow(query, tokenHash).Scan(
		&customer.ID, &customer.Email, &customer.GBBalance, &customer.Plan)
	
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}
	
	customer.Active = true
	return &customer, nil
}

func (a *Authenticator) validateIPWhitelist(userID, clientIP string) (*Customer, error) {
	// Check if IP is whitelisted for user
	query := `
		SELECT u.id, u.email, COALESCE(up.gb_balance, 0), p.name
		FROM ip_whitelist iw
		JOIN users u ON iw.user_id = u.id
		LEFT JOIN user_plans up ON u.id = up.user_id AND up.status = 'active'  
		LEFT JOIN plans p ON up.plan_id = p.id
		WHERE u.id = $1 AND (iw.ip_address = $2 OR iw.ip_range >>= $2::inet)
		AND iw.is_active = true
	`
	
	var customer Customer
	err := a.db.QueryRow(query, userID, clientIP).Scan(
		&customer.ID, &customer.Email, &customer.GBBalance, &customer.Plan)
		
	if err != nil {
		return nil, fmt.Errorf("IP not whitelisted")
	}
	
	customer.Active = true
	return &customer, nil
}

func (a *Authenticator) validateSignature(userID, timestamp, signature string) (*Customer, error) {
	// Get user's secret key
	var secretKey string
	err := a.db.QueryRow("SELECT api_secret FROM users WHERE id = $1", userID).Scan(&secretKey)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	
	// Compute expected signature: HMAC-SHA256(userID + timestamp, secret)
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(userID + timestamp))
	expected := hex.EncodeToString(mac.Sum(nil))
	
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return nil, fmt.Errorf("invalid signature")
	}
	
	// Get customer details
	return a.getCustomer(userID)
}

func (a *Authenticator) generateSessionID(customerID string) string {
	timestamp := time.Now().Unix()
	data := fmt.Sprintf("%s_%d", customerID, timestamp)
	hasher := sha256.New()
	hasher.Write([]byte(data))
	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

func (a *Authenticator) getCustomer(userID string) (*Customer, error) {
	var customer Customer
	query := `
		SELECT id, email, COALESCE(up.gb_balance, 0), p.name, status = 'active'
		FROM users u
		LEFT JOIN user_plans up ON u.id = up.user_id AND up.status = 'active'
		LEFT JOIN plans p ON up.plan_id = p.id  
		WHERE u.id = $1
	`
	
	err := a.db.QueryRow(query, userID).Scan(
		&customer.ID, &customer.Email, &customer.GBBalance, &customer.Plan, &customer.Active)
		
	if err != nil {
		return nil, fmt.Errorf("customer not found")
	}
	
	return &customer, nil
}