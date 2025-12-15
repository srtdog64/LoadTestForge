package httpdata

import "math/rand"

// Referers contains common referrer URLs including global and regional sites.
var Referers = []string{
	// Search Engines
	"https://www.google.com/",
	"https://www.google.com/search?q=",
	"https://www.bing.com/",
	"https://search.yahoo.com/",
	"https://duckduckgo.com/",
	"https://www.google.co.kr/",
	"https://www.baidu.com/",

	// Social Media
	"https://www.facebook.com/",
	"https://twitter.com/",
	"https://www.reddit.com/",
	"https://www.linkedin.com/",
	"https://www.instagram.com/",
	"https://www.tiktok.com/",
	"https://www.youtube.com/",

	// News & Portals
	"https://news.google.com/",
	"https://www.nytimes.com/",
	"https://www.bbc.com/",
	"https://www.cnn.com/",
	"https://www.naver.com/",
	"https://www.daum.net/",

	// Direct/Internal (Empty or same domain - handled dynamically)
	"",
}

// RandomReferer returns a random referer from the list.
func RandomReferer() string {
	ref := Referers[rand.Intn(len(Referers))]
	if ref == "https://www.google.com/search?q=" {
		return ref + RandomSearchTerm()
	}
	return ref
}

var SearchTerms = []string{
	"best", "review", "buy", "price", "news", "weather",
	"login", "register", "download", "tutorial", "how to",
}

func RandomSearchTerm() string {
	return SearchTerms[rand.Intn(len(SearchTerms))]
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
