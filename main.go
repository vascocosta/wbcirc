package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	irc "github.com/thoj/go-ircevent"
)

const channel = "#formula1"
const nick = "WBC"
const server = "irc.quakenet.org:6667"

func main() {
	var mutex sync.Mutex
	var parts []string
	authRequests := make(map[string]string)

	wbc := NewWBC()

	con := irc.IRC(nick, nick)
	con.VerboseCallbackHandler = false
	con.Debug = false

	err := con.Connect(server)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	con.AddCallback("001", func(e *irc.Event) {
		con.Join(channel)
	})

	con.AddCallback("PRIVMSG", func(e *irc.Event) {
		msg := strings.ToLower(e.Message())
		nick := e.Nick
		target := e.Arguments[0]

		if !strings.HasPrefix(msg, "!bet") {
			return
		}

		parts = strings.Fields(msg)

		if len(parts) == 1 {
			guess := wbc.currentGuess(nick)
			if guess == "" {
				con.Privmsg(channel, fmt.Sprintf("%s: You haven't placed a bet for the current race yet.", nick))
			} else {
				con.Privmsg(channel, fmt.Sprintf("%s: Your current bet for the %s", nick, guess))
			}
			return
		}

		if len(parts) != 6 {
			con.Privmsg(target, "Usage: !bet first second third fourth fifth")
			return
		}

		// Store pending auth check.
		mutex.Lock()
		authRequests[nick] = target
		mutex.Unlock()

		// Request WHOIS.
		con.Whois(nick)
	})

	// If user is authed (numeric 330)
	con.AddCallback("330", func(e *irc.Event) {
		// Format: <you> <nick> <account> :is authed as
		if len(e.Arguments) < 3 {
			return
		}

		nick := e.Arguments[1]
		account := e.Arguments[2]

		mutex.Lock()
		channel, ok := authRequests[nick]
		delete(authRequests, nick)
		mutex.Unlock()

		if !ok {
			return
		}

		// The nick is authed, now we check if the nick is authorized (usual account).

		ok, err := isNickAuthorized(nick, account)
		if err != nil {
			con.Privmsg(channel, fmt.Sprintf("%s %v", err, nick))
			return
		}

		if !ok {
			con.Privmsg(channel, fmt.Sprintf("%s you were caught trying to play as another user. Shame!", account))
			return
		}

		if len(parts) != 6 {
			con.Privmsg(channel, "Usage: !bet first second third fourth fifth")
			return
		}

		err = wbc.play(nick, parts[1:])
		if err != nil {
			con.Privmsg(channel, fmt.Sprintf("%s: %s", nick, err.Error()))
			return
		}

		con.Privmsg(channel, fmt.Sprintf("%s: Your bet was successfully updated.", nick))
	})

	// End of WHOIS (318) -> means user was NOT authed.
	con.AddCallback("318", func(e *irc.Event) {
		// Format: <you> <nick> :End of WHOIS
		if len(e.Arguments) < 2 {
			return
		}

		nick := e.Arguments[1]

		mutex.Lock()
		channel, ok := authRequests[nick]
		if ok {
			delete(authRequests, nick)
		}
		mutex.Unlock()

		if !ok {
			return
		}

		con.Privmsg(channel, fmt.Sprintf("%s you are not authenticated. Please authenticate to play WBC.", nick))
	})

	// Optional safety cleanup if WHOIS never returns.
	go func() {
		for {
			time.Sleep(60 * time.Second)

			mutex.Lock()
			authRequests = make(map[string]string)
			mutex.Unlock()
		}
	}()

	con.Loop()
}
