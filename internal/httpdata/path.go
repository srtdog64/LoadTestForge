package httpdata

import (
	"fmt"
	"math/rand"
	"net/url"
	"time"
)

// PathRandomizer provides realistic URL path and query string randomization
// for cache bypass and load distribution.
type PathRandomizer struct {
	AddTimestamp  bool
	AddRandom     bool
	AddVersion    bool
	AddRefSource  bool
	AddUserID     bool
	AddSession    bool
	AddUTMSource  bool
	AddDeviceType bool
}

// DefaultPathRandomizer returns a randomizer with common cache-busting options.
func DefaultPathRandomizer() *PathRandomizer {
	return &PathRandomizer{
		AddTimestamp:  true,
		AddRandom:     true,
		AddVersion:    true,
		AddRefSource:  true,
		AddUserID:     false,
		AddSession:    false,
		AddUTMSource:  false,
		AddDeviceType: false,
	}
}

// FullPathRandomizer returns a randomizer with all options enabled.
func FullPathRandomizer() *PathRandomizer {
	return &PathRandomizer{
		AddTimestamp:  true,
		AddRandom:     true,
		AddVersion:    true,
		AddRefSource:  true,
		AddUserID:     true,
		AddSession:    true,
		AddUTMSource:  true,
		AddDeviceType: true,
	}
}

// RandomizePath adds random query parameters to a URL path.
// This is useful for bypassing caches and CDNs.
func (p *PathRandomizer) RandomizePath(basePath string) string {
	if basePath == "" {
		basePath = "/"
	}

	params := url.Values{}

	if p.AddTimestamp {
		params.Set("_", fmt.Sprintf("%d", time.Now().UnixMilli()))
	}

	if p.AddRandom {
		params.Set("r", fmt.Sprintf("%d", rand.Intn(1000000)))
	}

	if p.AddVersion {
		params.Set("v", fmt.Sprintf("%d", rand.Intn(100)+1))
	}

	if p.AddRefSource {
		params.Set("ref", RandomRefSource())
	}

	if p.AddUserID && rand.Float32() < 0.2 {
		params.Set("user_id", fmt.Sprintf("%d", rand.Intn(9000)+1000))
	}

	if p.AddDeviceType && rand.Float32() < 0.2 {
		params.Set("device", RandomDeviceType())
	}

	if p.AddSession && rand.Float32() < 0.15 {
		params.Set("session", GenerateSessionID())
	}

	if p.AddUTMSource && rand.Float32() < 0.1 {
		params.Set("utm_source", RandomUTMSource())
	}

	// Add cache bypass parameter
	cacheOptions := []string{"true", "false", "1", "0"}
	params.Set("cache", cacheOptions[rand.Intn(len(cacheOptions))])

	return basePath + "?" + params.Encode()
}

// RandomizeURL adds random query parameters to a parsed URL.
func (p *PathRandomizer) RandomizeURL(baseURL *url.URL) string {
	u := *baseURL
	q := u.Query()

	if p.AddTimestamp {
		q.Set("_", fmt.Sprintf("%d", time.Now().UnixMilli()))
	}

	if p.AddRandom {
		q.Set("r", fmt.Sprintf("%d", rand.Intn(1000000)))
	}

	if p.AddVersion {
		q.Set("v", fmt.Sprintf("%d", rand.Intn(100)+1))
	}

	if p.AddRefSource {
		q.Set("ref", RandomRefSource())
	}

	if p.AddUserID && rand.Float32() < 0.2 {
		q.Set("user_id", fmt.Sprintf("%d", rand.Intn(9000)+1000))
		q.Set("device", RandomDeviceType())
	}

	if p.AddSession && rand.Float32() < 0.15 {
		q.Set("session", GenerateSessionID())
	}

	if p.AddUTMSource && rand.Float32() < 0.1 {
		q.Set("utm_source", RandomUTMSource())
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// SelectPath chooses between the original path or a random form endpoint.
// probability determines the chance of selecting a random endpoint (0.0-1.0).
func SelectPath(originalPath string, randomizePath bool, probability float32) string {
	if !randomizePath {
		return originalPath
	}

	if rand.Float32() < probability {
		return RandomFormEndpoint()
	}

	if originalPath == "" {
		return "/"
	}
	return originalPath
}

// RandomUTMParams returns a set of random UTM parameters.
func RandomUTMParams() map[string]string {
	sources := []string{"google", "facebook", "twitter", "linkedin", "email"}
	mediums := []string{"cpc", "organic", "social", "email", "referral"}
	campaigns := []string{"brand", "product", "promo", "seasonal", "launch"}

	return map[string]string{
		"utm_source":   sources[rand.Intn(len(sources))],
		"utm_medium":   mediums[rand.Intn(len(mediums))],
		"utm_campaign": campaigns[rand.Intn(len(campaigns))],
	}
}
