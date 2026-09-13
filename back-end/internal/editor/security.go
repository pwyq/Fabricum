package editor

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

func localRequest(request *http.Request) bool {
	host := request.Host
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return false
	}
	if origin := request.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme != "http" || parsed.Host != request.Host {
			return false
		}
	}
	return true
}
