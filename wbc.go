package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const apiURL = "https://wbc.gluonspace.com/api/"
const usersFile = "/home/gluon/src/github.com/vascocosta/wbc/data/users.csv"

type WBC struct {
	url string
}

func NewWBC() WBC {
	return WBC{
		url: apiURL,
	}
}

func (wbc WBC) currentGuess(nick string) string {
	resp, err := http.Get(fmt.Sprintf("%s?format=irc&username=%s", wbc.url+"guesses", nick))
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

func (wbc WBC) play(nick string, drivers []string) error {
	apiKey, err := wbc.token(nick)
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

	req, err := http.NewRequest(http.MethodPost, apiURL+"play", bytes.NewBuffer(jsonData))
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

func (wbc WBC) token(nick string) (string, error) {
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
