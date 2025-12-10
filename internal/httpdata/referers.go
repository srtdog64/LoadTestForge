package httpdata

import "math/rand"

// Referers contains common referrer URLs including global and regional sites.
var Referers = []string{
	"https://www.google.com/",
	"https://www.google.co.kr/",
	"https://www.bing.com/",
	"https://www.naver.com/",
	"https://www.daum.net/",
	"https://www.youtube.com/",
	"https://www.facebook.com/",
	"https://twitter.com/",
	"https://www.instagram.com/",
	"https://www.tiktok.com/",
	"https://www.amazon.com/",
	"https://www.netflix.com/",
	"https://github.com/",
	"https://www.reddit.com/",
	"https://www.linkedin.com/",
	"https://www.apple.com/",
	"https://www.microsoft.com/",
}

// RandomReferer returns a random referrer URL.
func RandomReferer() string {
	return Referers[rand.Intn(len(Referers))]
}

// RandomRefSource returns a random referrer source identifier for query parameters.
// Used in ?ref=google style tracking.
func RandomRefSource() string {
	sources := []string{
		"google", "facebook", "twitter", "linkedin", "reddit",
		"bing", "yahoo", "duckduckgo", "instagram", "tiktok",
		"naver", "daum", "direct", "email", "newsletter",
		"organic", "referral",
	}
	return sources[rand.Intn(len(sources))]
}
