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
		w, `
<!doctype html>
<html>
	<head>
		<meta charset="utf-8">
		<title>%s</title>
	</head>
	<body>
		<h1>%s</h1>
		<p>%s</p>
		<footer>
			<p>
				Provided by
				<a href="https://github.com/UMichDevelopers/dvb30">dvb30</a>.
				Copyright © 2026
				<a href="https://runxiyu.org">Runxi Yu</a>.
			</p>
			<details>
				<summary>3-clause BSD license</summary>
				<p>
					Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
				</p>
				<ol>
					<li>Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.</li>
					<li>Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.</li>
					<li>The name of the author may not be used to endorse or promote products derived from this software without specific prior written permission.</li>
				</ol>
				<p>THIS SOFTWARE IS PROVIDED BY THE AUTHOR “AS IS” AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.</p>
			</details>
		</footer>
	</body>
</html>`,
		html.EscapeString(title),
		html.EscapeString(title),
		html.EscapeString(body),
	)
}
