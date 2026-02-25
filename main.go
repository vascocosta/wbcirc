package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	irc "github.com/thoj/go-ircevent"
)

const apiURL = "https://wbc.gluonspace.com/api/"
const channel = "#formula1"
const guessEndpoint = "guesses"
const ircAccountsFile = "./irc_accounts.csv"
const nick = "WBC"
const playEndpoint = "play"
const server = "irc.quakenet.org:6667"
const usersFile = "/home/gluon/src/github.com/vascocosta/wbc/data/users.csv"

func main() {
	var mutex sync.Mutex
	var parts []string
	authRequests := make(map[string]string)

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
			guess := currentGuess(nick)
			con.Privmsg(channel, guess)
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

		err = play(nick, parts[1:])
		if err != nil {
			con.Privmsg(channel, fmt.Sprintf("%s: %s", nick, err.Error()))
			return
		}

		con.Privmsg(channel, fmt.Sprintf("%s: your bet was updated.", nick))
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

func token(nick string) (string, error) {
	f, err := os.Open(usersFile)
	if err != nil {
		return "", errors.New("Could not open users file.")
	}
	defer f.Close()

	r := csv.NewReader(f)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", errors.New(("Could not read from users file."))
		}
		if len(record) == 4 && strings.EqualFold(record[1], nick) {
			return record[0], nil
		}
	}

	return "", errors.New("Token not found.")
}

func currentGuess(nick string) string {
	resp, err := http.Get(fmt.Sprintf("%s?format=irc&username=%s", apiURL+guessEndpoint, nick))
	if err != nil {
		return "Cannot not get your current bet."
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Cannot get your current bet."
	}

	return string(body)
}

func play(nick string, drivers []string) error {
	apiKey, err := token(nick)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"race":     "",
		"username": nick,
		"p1":       drivers[0],
		"p2":       drivers[1],
		"p3":       drivers[2],
		"p4":       drivers[3],
		"p5":       drivers[4],
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return errors.New("Could not parse data.")
	}

	req, err := http.NewRequest(http.MethodPost, apiURL+playEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return errors.New(string(body))
}

func isNickAuthorized(nick string, account string) (bool, error) {
	f, err := os.Open(ircAccountsFile)
	if err != nil {
		return false, errors.New("Could not verify IRC account.")
	}
	defer f.Close()

	r := csv.NewReader(f)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return false, errors.New("Could not verify IRC account.")
		}

		if len(record) != 2 {
			return false, errors.New("Could not verify IRC account.")
		}

		if strings.EqualFold(record[0], nick) && strings.EqualFold(record[1], account) {
			return true, nil
		}
	}

	return false, nil
}
