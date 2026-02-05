package headers

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"proxy-gateway/internal/auth"
	"proxy-gateway/internal/session"
)

type HeaderManager struct {
	profiles map[string]BrowserProfile
	rand     *rand.Rand
}

type BrowserProfile struct {
	Name            string            `json:"name"`
	UserAgents      []string          `json:"user_agents"`
	DefaultHeaders  map[string]string `json:"default_headers"`
	AcceptLanguages []string          `json:"accept_languages"`
	Encodings       []string          `json:"encodings"`
	DNTValues       []string          `json:"dnt_values"`
	ConnectionTypes []string          `json:"connection_types"`
}

func NewHeaderManager() *HeaderManager {
	hm := &HeaderManager{
		profiles: make(map[string]BrowserProfile),
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	
	hm.initializeProfiles()
	return hm
}

func (hm *HeaderManager) initializeProfiles() {
	// Chrome Windows Profile
	hm.profiles["chrome-win"] = BrowserProfile{
		Name: "chrome-win",
		UserAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		},
		DefaultHeaders: map[string]string{
			"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
			"Accept-Encoding": "gzip, deflate, br",
			"Cache-Control": "max-age=0",
			"Sec-Ch-Ua": `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
			"Sec-Ch-Ua-Mobile": "?0",
			"Sec-Ch-Ua-Platform": `"Windows"`,
			"Sec-Fetch-Dest": "document",
			"Sec-Fetch-Mode": "navigate",
			"Sec-Fetch-Site": "none",
			"Sec-Fetch-User": "?1",
			"Upgrade-Insecure-Requests": "1",
		},
		AcceptLanguages: []string{
			"en-US,en;q=0.9",
			"en-US,en;q=0.9,es;q=0.8",
			"en-GB,en;q=0.9",
		},
		Encodings: []string{
			"gzip, deflate, br",
			"gzip, deflate, br, zstd",
		},
		DNTValues: []string{"1", "0"},
		ConnectionTypes: []string{"keep-alive"},
	}
	
	// Firefox Windows Profile
	hm.profiles["firefox-win"] = BrowserProfile{
		Name: "firefox-win",
		UserAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:119.0) Gecko/20100101 Firefox/119.0",
		},
		DefaultHeaders: map[string]string{
			"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
			"Accept-Encoding": "gzip, deflate, br",
			"Cache-Control": "no-cache",
			"Pragma": "no-cache",
			"Sec-Fetch-Dest": "document",
			"Sec-Fetch-Mode": "navigate",
			"Sec-Fetch-Site": "none",
			"Sec-Fetch-User": "?1",
			"Upgrade-Insecure-Requests": "1",
		},
		AcceptLanguages: []string{
			"en-US,en;q=0.5",
			"en-GB,en;q=0.5",
			"en-US,en;q=0.5,es-ES;q=0.3",
		},
		Encodings: []string{
			"gzip, deflate, br",
		},
		DNTValues: []string{"1", "0"},
		ConnectionTypes: []string{"keep-alive"},
	}
	
	// Safari macOS Profile
	hm.profiles["safari-mac"] = BrowserProfile{
		Name: "safari-mac",
		UserAgents: []string{
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		},
		DefaultHeaders: map[string]string{
			"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Accept-Encoding": "gzip, deflate, br",
			"Cache-Control": "max-age=0",
		},
		AcceptLanguages: []string{
			"en-US,en;q=0.9",
			"en-GB,en;q=0.9",
		},
		Encodings: []string{
			"gzip, deflate, br",
		},
		DNTValues: []string{"1"},
		ConnectionTypes: []string{"keep-alive"},
	}
	
	// Mobile Chrome iOS Profile
	hm.profiles["mobile-ios"] = BrowserProfile{
		Name: "mobile-ios",
		UserAgents: []string{
			"Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/121.0.6167.138 Mobile/15E148 Safari/604.1",
			"Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/120.0.6099.119 Mobile/15E148 Safari/604.1",
			"Mozilla/5.0 (iPhone; CPU iPhone OS 16_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/121.0.6167.138 Mobile/15E148 Safari/604.1",
		},
		DefaultHeaders: map[string]string{
			"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Accept-Encoding": "gzip, deflate, br",
			"Cache-Control": "max-age=0",
		},
		AcceptLanguages: []string{
			"en-US,en;q=0.9",
			"en-GB,en;q=0.9",
		},
		Encodings: []string{
			"gzip, deflate, br",
		},
		DNTValues: []string{"1", "0"},
		ConnectionTypes: []string{"keep-alive"},
	}
	
	// Mobile Chrome Android Profile  
	hm.profiles["mobile-android"] = BrowserProfile{
		Name: "mobile-android",
		UserAgents: []string{
			"Mozilla/5.0 (Linux; Android 14; SM-G998B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Mobile Safari/537.36",
			"Mozilla/5.0 (Linux; Android 13; SM-A515F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Mobile Safari/537.36",
			"Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Mobile Safari/537.36",
		},
		DefaultHeaders: map[string]string{
			"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
			"Accept-Encoding": "gzip, deflate, br",
			"Cache-Control": "max-age=0",
			"Sec-Ch-Ua": `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
			"Sec-Ch-Ua-Mobile": "?1",
			"Sec-Ch-Ua-Platform": `"Android"`,
			"Sec-Fetch-Dest": "document",
			"Sec-Fetch-Mode": "navigate",
			"Sec-Fetch-Site": "none", 
			"Sec-Fetch-User": "?1",
			"Upgrade-Insecure-Requests": "1",
		},
		AcceptLanguages: []string{
			"en-US,en;q=0.9",
			"en-GB,en;q=0.9",
		},
		Encodings: []string{
			"gzip, deflate, br",
		},
		DNTValues: []string{"1", "0"},
		ConnectionTypes: []string{"keep-alive"},
	}
}

func (hm *HeaderManager) GenerateHeaders(session *session.Session, originalHeaders http.Header, request *http.Request) http.Header {
	// Start with original headers
	headers := make(http.Header)
	for k, v := range originalHeaders {
		headers[k] = v
	}
	
	// Get browser profile
	profile := hm.getProfile(session.Profile)
	
	// Generate User-Agent if not specified or using profile
	if session.UserAgent != "" {
		headers.Set("User-Agent", session.UserAgent)
	} else if userAgent := hm.generateUserAgent(profile, session); userAgent != "" {
		headers.Set("User-Agent", userAgent)
	}
	
	// Apply profile default headers
	hm.applyProfileHeaders(headers, profile, session)
	
	// Apply geographic headers
	hm.applyGeographicHeaders(headers, session)
	
	// Apply custom session headers
	for key, value := range session.Headers {
		headers.Set(key, value)
	}
	
	// Apply context-aware headers
	hm.applyContextHeaders(headers, request, session)
	
	// Randomize some headers for fingerprint resistance
	hm.randomizeHeaders(headers, profile)
	
	return headers
}

func (hm *HeaderManager) getProfile(profileName string) BrowserProfile {
	if profile, exists := hm.profiles[profileName]; exists {
		return profile
	}
	return hm.profiles["chrome-win"] // default
}

func (hm *HeaderManager) generateUserAgent(profile BrowserProfile, session *session.Session) string {
	if len(profile.UserAgents) == 0 {
		return ""
	}
	
	// Use consistent randomization based on session ID for stickiness
	sessionSeed := int64(0)
	for _, char := range session.ID {
		sessionSeed += int64(char)
	}
	
	sessionRand := rand.New(rand.NewSource(sessionSeed))
	return profile.UserAgents[sessionRand.Intn(len(profile.UserAgents))]
}

func (hm *HeaderManager) applyProfileHeaders(headers http.Header, profile BrowserProfile, session *session.Session) {
	for key, value := range profile.DefaultHeaders {
		// Don't override if already set
		if headers.Get(key) == "" {
			headers.Set(key, value)
		}
	}
	
	// Set Accept-Language based on country
	if headers.Get("Accept-Language") == "" {
		acceptLang := hm.getAcceptLanguageForCountry(session.Country, profile)
		if acceptLang != "" {
			headers.Set("Accept-Language", acceptLang)
		}
	}
}

func (hm *HeaderManager) applyGeographicHeaders(headers http.Header, session *session.Session) {
	// Add geographic hints where appropriate
	if session.Country != "" {
		// Some sites check for specific headers
		switch session.Country {
		case "CN":
			if headers.Get("Accept-Language") == "" {
				headers.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
			}
		case "JP":
			if headers.Get("Accept-Language") == "" {
				headers.Set("Accept-Language", "ja,en;q=0.9")
			}
		case "KR":
			if headers.Get("Accept-Language") == "" {
				headers.Set("Accept-Language", "ko,en;q=0.9")
			}
		case "DE":
			if headers.Get("Accept-Language") == "" {
				headers.Set("Accept-Language", "de-DE,de;q=0.9,en;q=0.8")
			}
		case "FR":
			if headers.Get("Accept-Language") == "" {
				headers.Set("Accept-Language", "fr-FR,fr;q=0.9,en;q=0.8")
			}
		case "ES":
			if headers.Get("Accept-Language") == "" {
				headers.Set("Accept-Language", "es-ES,es;q=0.9,en;q=0.8")
			}
		case "BR":
			if headers.Get("Accept-Language") == "" {
				headers.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")
			}
		case "RU":
			if headers.Get("Accept-Language") == "" {
				headers.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")
			}
		}
	}
}

func (hm *HeaderManager) applyContextHeaders(headers http.Header, request *http.Request, session *session.Session) {
	// Add request-specific headers
	if request != nil {
		// Set appropriate Sec-Fetch-* headers based on request type
		if strings.Contains(request.URL.Path, ".js") {
			headers.Set("Sec-Fetch-Dest", "script")
		} else if strings.Contains(request.URL.Path, ".css") {
			headers.Set("Sec-Fetch-Dest", "style")
		} else if isImagePath(request.URL.Path) {
			headers.Set("Sec-Fetch-Dest", "image")
		}
		
		// Set Referer policy
		if headers.Get("Referer") == "" && request.Referer() != "" {
			headers.Set("Referer", request.Referer())
		}
	}
	
	// Set Connection type
	if headers.Get("Connection") == "" {
		headers.Set("Connection", "keep-alive")
	}
}

func (hm *HeaderManager) randomizeHeaders(headers http.Header, profile BrowserProfile) {
	// Randomize DNT (Do Not Track)
	if len(profile.DNTValues) > 0 && headers.Get("DNT") == "" {
		dnt := profile.DNTValues[hm.rand.Intn(len(profile.DNTValues))]
		headers.Set("DNT", dnt)
	}
	
	// Randomly add or omit some optional headers
	if hm.rand.Float32() < 0.7 { // 70% chance
		headers.Set("Cache-Control", "no-cache")
	}
	
	if hm.rand.Float32() < 0.3 { // 30% chance
		headers.Set("Pragma", "no-cache")
	}
}

func (hm *HeaderManager) getAcceptLanguageForCountry(country string, profile BrowserProfile) string {
	if country == "" {
		if len(profile.AcceptLanguages) > 0 {
			return profile.AcceptLanguages[0]
		}
		return ""
	}
	
	// Country-specific Accept-Language mappings
	countryLanguages := map[string]string{
		"US": "en-US,en;q=0.9",
		"GB": "en-GB,en;q=0.9",
		"CA": "en-CA,en;q=0.9,fr;q=0.8",
		"AU": "en-AU,en;q=0.9",
		"DE": "de-DE,de;q=0.9,en;q=0.8",
		"FR": "fr-FR,fr;q=0.9,en;q=0.8",
		"ES": "es-ES,es;q=0.9,en;q=0.8",
		"IT": "it-IT,it;q=0.9,en;q=0.8",
		"BR": "pt-BR,pt;q=0.9,en;q=0.8",
		"RU": "ru-RU,ru;q=0.9,en;q=0.8",
		"CN": "zh-CN,zh;q=0.9,en;q=0.8",
		"JP": "ja,en;q=0.9",
		"KR": "ko,en;q=0.9",
		"IN": "hi,en;q=0.9",
		"MX": "es-MX,es;q=0.9,en;q=0.8",
		"NL": "nl-NL,nl;q=0.9,en;q=0.8",
		"SE": "sv-SE,sv;q=0.9,en;q=0.8",
		"NO": "no,en;q=0.9",
		"DK": "da,en;q=0.9",
		"FI": "fi,en;q=0.9",
		"PL": "pl,en;q=0.9",
		"TR": "tr,en;q=0.9",
		"TH": "th,en;q=0.9",
		"VN": "vi,en;q=0.9",
		"ID": "id,en;q=0.9",
		"MY": "ms,en;q=0.9",
		"SG": "en-SG,en;q=0.9,zh;q=0.8",
		"HK": "zh-HK,zh;q=0.9,en;q=0.8",
		"TW": "zh-TW,zh;q=0.9,en;q=0.8",
		"ZA": "en-ZA,en;q=0.9,af;q=0.8",
		"EG": "ar,en;q=0.9",
		"SA": "ar-SA,ar;q=0.9,en;q=0.8",
		"AE": "ar,en;q=0.9",
		"IL": "he,en;q=0.9",
		"GR": "el,en;q=0.9",
		"PT": "pt-PT,pt;q=0.9,en;q=0.8",
		"CZ": "cs,en;q=0.9",
		"HU": "hu,en;q=0.9",
		"RO": "ro,en;q=0.9",
		"BG": "bg,en;q=0.9",
		"HR": "hr,en;q=0.9",
		"SI": "sl,en;q=0.9",
		"SK": "sk,en;q=0.9",
		"LT": "lt,en;q=0.9",
		"LV": "lv,en;q=0.9",
		"EE": "et,en;q=0.9",
		"UA": "uk,en;q=0.9,ru;q=0.8",
		"BY": "be,ru;q=0.9,en;q=0.8",
		"KZ": "kk,ru;q=0.9,en;q=0.8",
	}
	
	if lang, exists := countryLanguages[country]; exists {
		return lang
	}
	
	// Fall back to profile default
	if len(profile.AcceptLanguages) > 0 {
		return profile.AcceptLanguages[0]
	}
	
	return "en-US,en;q=0.9"
}

func isImagePath(path string) bool {
	imageExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".ico", ".bmp"}
	lowerPath := strings.ToLower(path)
	
	for _, ext := range imageExtensions {
		if strings.HasSuffix(lowerPath, ext) {
			return true
		}
	}
	return false
}

// GetProfileList returns available browser profiles
func (hm *HeaderManager) GetProfileList() []string {
	profiles := make([]string, 0, len(hm.profiles))
	for name := range hm.profiles {
		profiles = append(profiles, name)
	}
	return profiles
}

// AddCustomProfile allows adding custom browser profiles
func (hm *HeaderManager) AddCustomProfile(name string, profile BrowserProfile) {
	hm.profiles[name] = profile
}

// Header quality scoring for analytics
func (hm *HeaderManager) ScoreHeaders(headers http.Header) int {
	score := 0
	
	// Essential headers
	if headers.Get("User-Agent") != "" { score += 20 }
	if headers.Get("Accept") != "" { score += 15 }
	if headers.Get("Accept-Language") != "" { score += 15 }
	if headers.Get("Accept-Encoding") != "" { score += 10 }
	
	// Security headers
	if headers.Get("Sec-Fetch-Dest") != "" { score += 10 }
	if headers.Get("Sec-Fetch-Mode") != "" { score += 10 }
	if headers.Get("Sec-Fetch-Site") != "" { score += 10 }
	
	// Quality indicators
	if headers.Get("Cache-Control") != "" { score += 5 }
	if headers.Get("Connection") != "" { score += 5 }
	
	return score
}