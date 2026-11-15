package handlers

import (
	"net/http"
	"strings"
	"time"

	"pbs_help_desk/internal/auth"
)

const (
	AccessTokenCookieName  = auth.AccessTokenCookieName
	RefreshTokenCookieName = "refresh_token"
)

func isSecureRequest(r *http.Request) bool {
	return r.TLS != nil ||
		strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") ||
		strings.EqualFold(r.Header.Get("CF-Visitor"), `{"scheme":"https"}`)
}

func cookieSettings(r *http.Request, cookieDomain string) (secure bool, domain string, sameSite http.SameSite) {
	secure = isSecureRequest(r)
	domain = strings.TrimSpace(cookieDomain)
	sameSite = http.SameSiteLaxMode
	if secure && domain == "" {
		sameSite = http.SameSiteNoneMode
	}
	return secure, domain, sameSite
}

func setAuthCookies(
	w http.ResponseWriter,
	r *http.Request,
	cookieDomain string,
	accessToken string,
	accessExpires time.Time,
	refreshToken string,
	refreshExpires time.Time,
) {
	secure, domain, sameSite := cookieSettings(r, cookieDomain)

	http.SetCookie(w, &http.Cookie{
		Name:     AccessTokenCookieName,
		Value:    accessToken,
		Path:     "/",
		Domain:   domain,
		Expires:  accessExpires,
		MaxAge:   max(0, int(time.Until(accessExpires).Seconds())),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    refreshToken,
		Path:     "/",
		Domain:   domain,
		Expires:  refreshExpires,
		MaxAge:   max(0, int(time.Until(refreshExpires).Seconds())),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

func clearAuthCookies(w http.ResponseWriter, r *http.Request, cookieDomain string) {
	secure, domain, sameSite := cookieSettings(r, cookieDomain)

	for _, name := range []string{AccessTokenCookieName, RefreshTokenCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Domain:   domain,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   secure,
			SameSite: sameSite,
		})
	}
}
