package main

import (
	"encoding/csv"
	"errors"
	"io"
	"os"
	"strings"
)

const ircAccountsFile = "./irc_accounts.csv"

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

		if strings.EqualFold(record[0], nick) {
			if strings.EqualFold(record[1], account) {
				return true, nil
			} else {
				return false, nil
			}
		}
	}

	return false, errors.New("Ask gluon to add your QuakeNet account name to the database.")
}
