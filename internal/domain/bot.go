package domain

import "strings"

// botSubstrings match crawlers, link-preview unfurlers, and monitoring
// clients. Bot visits are excluded from analytics rollups at aggregation time.
var botSubstrings = []string{
	"googlebot", "bingbot", "duckduckbot", "baiduspider", "yandexbot",
	"facebookexternalhit", "twitterbot", "linkedinbot", "slackbot",
	"telegrambot", "whatsapp", "discordbot", "skypeuripreview",
	"pinterest", "instagram", "snapchat", "viber",
	"curl", "wget", "python-requests", "go-http-client", "java/",
	"node-fetch", "axios", "postmanruntime",
	"uptimerobot", "pingdom", "datadog", "newrelic",
}

// IsBot reports whether a user-agent looks like a crawler, link-preview
// unfurler, or monitoring client.
func IsBot(ua string) bool {
	u := strings.ToLower(ua)
	for _, b := range botSubstrings {
		if strings.Contains(u, b) {
			return true
		}
	}
	return false
}
