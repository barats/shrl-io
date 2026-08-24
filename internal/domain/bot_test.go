package domain

import "testing"

func TestIsBot(t *testing.T) {
	bots := []string{
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)",
		"TelegramBot (like TwitterBot)",
		"curl/8.7.1",
		"python-requests/2.31.0",
		"facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
		"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		"PostmanRuntime/7.36.0",
	}
	humans := []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (X11; Linux x86_64) Firefox/121.0",
	}
	for _, ua := range bots {
		if !IsBot(ua) {
			t.Errorf("expected %q to be classified as a bot", ua)
		}
	}
	for _, ua := range humans {
		if IsBot(ua) {
			t.Errorf("expected %q to be classified as human", ua)
		}
	}
}
