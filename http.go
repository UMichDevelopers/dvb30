package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
)

type httpServer struct {
	authenticator *googleAuthenticator
	bot           *discordBot
}

func newHTTPServer(authenticator *googleAuthenticator, bot *discordBot) *httpServer {
	return &httpServer{
		authenticator: authenticator,
		bot:           bot,
	}
}

func (s *httpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("http %s %s", r.Method, r.URL.Path)

	switch r.URL.Path {
	case "/":
		s.handleIndex(w, r)
	case "/auth":
		s.handleAuth(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *httpServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	writePage(
		w,
		http.StatusOK,
		"Discord Verification",
		"Use the /verify command in Discord. The bot will send you a Google sign-in link. After you sign in with your umich.edu account, this service will assign your verified role automatically.",
	)
}

func (s *httpServer) handleAuth(w http.ResponseWriter, r *http.Request) {
	if errText := r.URL.Query().Get("error"); errText != "" {
		desc := r.URL.Query().Get("error_description")
		log.Printf("google oauth error: %s: %s", errText, desc)
		writePage(w, http.StatusBadRequest, "Verification Failed", "Google rejected the verification request.")
		return
	}

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		writePage(w, http.StatusBadRequest, "Verification Failed", "Missing OAuth parameters.")
		return
	}

	tokenState, err := s.authenticator.verifyCode(r.Context(), state, code)
	if err != nil {
		log.Printf("verify callback: %v", err)
		writePage(w, http.StatusUnauthorized, "Verification Failed", "Could not verify your umich.edu account.")
		return
	}

	if err := s.bot.addVerifiedRole(tokenState.GuildID, tokenState.UserID); err != nil {
		log.Printf("assign verified role: %v", err)
		writePage(w, http.StatusInternalServerError, "Verification Failed", "Your account was verified, but the Discord role could not be assigned.")
		return
	}

	log.Printf("verification complete for user_id=%d guild_id=%d", tokenState.UserID, tokenState.GuildID)
	writePage(w, http.StatusOK, "Verification Complete", "Your Discord account is now verified.")
}

func writePage(w http.ResponseWriter, status int, title string, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(
		w,
		"<!doctype html><html><head><title>%s</title></head><body><h1>%s</h1><p>%s</p></body></html>",
		html.EscapeString(title),
		html.EscapeString(title),
		html.EscapeString(body),
	)
}
