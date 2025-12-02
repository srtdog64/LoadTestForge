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

// RefSources contains referrer source identifiers for query parameters.
var RefSources = []string{
	"google",
	"naver",
	"daum",
	"facebook",
	"direct",
	"bing",
	"yahoo",
	"twitter",
}

// UTMSources contains common UTM source values for marketing tracking simulation.
var UTMSources = []string{
	"google",
	"facebook",
	"newsletter",
	"direct",
	"twitter",
	"naver",
	"instagram",
	"linkedin",
}

// RandomReferer returns a random referrer URL.
func RandomReferer() string {
	return Referers[rand.Intn(len(Referers))]
}

// RandomRefSource returns a random referrer source identifier.
func RandomRefSource() string {
	return RefSources[rand.Intn(len(RefSources))]
}

// RandomUTMSource returns a random UTM source.
func RandomUTMSource() string {
	return UTMSources[rand.Intn(len(UTMSources))]
}
