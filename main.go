package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("gm", "Guerrilla Mail CLI.")

	new = app.Command("new", "Create new session.")

	info = app.Command("info", "Current session info.")

	list = app.Command("list", "List emails.")

	sync = app.Command("sync", "Sync emails.")

	read   = app.Command("read", "Read email <id>.")
	readID = read.Arg("id", "Email ID.").Required().String()

	del      = app.Command("delete", "Delete email <id>.")
	deleteID = del.Arg("id", "Email ID.").Required().String()
)

type Session struct {
	SID     string `json:"sid"`
	Address string `json:"address"`
	Alias   string `json:"alias"`
}

type Email struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type Config struct {
	Session Session          `json:"session"`
	Emails  map[string]Email `json:"emails"`
}

type GuerrillaSession struct {
	SID     string `json:"sid_token"`
	Address string `json:"email_addr"`
	Alias   string `json:"alias"`
}

type GuerrillaEmail struct {
	ID      interface{} `json:"mail_id"`
	From    string      `json:"mail_from"`
	Subject string      `json:"mail_subject"`
	Body    string      `json:"mail_body"`
}

type GuerrillaEmails struct {
	SID    string           `json:"sid_token"`
	Emails []GuerrillaEmail `json:"list"`
}

func main() {
	app.Version("0.0.1")

	client := &http.Client{}

	config := loadConfig()

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case info.FullCommand():
		if config.Session.Address == "" {
			config.Session = getNewSession(client)
		}
		json.NewEncoder(os.Stdout).Encode(config.Session)
		break
	case new.FullCommand():
		config.Session = getNewSession(client)
		json.NewEncoder(os.Stdout).Encode(config.Session)
		break
	case list.FullCommand():
		json.NewEncoder(os.Stdout).Encode(values(config.Emails))
		break
	case sync.FullCommand():
		if config.Session.Address == "" {
			config.Session = getNewSession(client)
		}
		checkEmails(client, config)
		config.Emails = getEmails(client, config)
		json.NewEncoder(os.Stdout).Encode(values(config.Emails))
		break
	case read.FullCommand():
		email := readEmail(client, config)
		json.NewEncoder(os.Stdout).Encode(email)
		break
	case del.FullCommand():
		deleted := deleteEmail(client, config)
		fmt.Println(deleted)
		break
	}

	saveConfig(config)
}

func values(emailMap map[string]Email) []Email {
	emails := []Email{}
	for _, v := range emailMap {
		emails = append(emails, v)
	}
	return emails
}

func loadConfig() Config {
	var config Config

	sessionFilepath := fmt.Sprintf("%s/.guerrillamail", os.Getenv("HOME"))
	if _, err := os.Stat(sessionFilepath); err == nil {
		file, err := os.Open(sessionFilepath)

		if err != nil {
			log.Fatal(err)
			return config
		}

		defer file.Close()

		err = json.NewDecoder(file).Decode(&config)

		if err != nil {
			log.Fatal(err)
			return config
		}
	}

	return config
}

func saveConfig(config Config) {
	sessionFilepath := fmt.Sprintf("%s/.guerrillamail", os.Getenv("HOME"))
	file, err := os.OpenFile(sessionFilepath, os.O_CREATE|os.O_WRONLY, 0666)

	if err != nil {
		log.Fatal(err)
		return
	}

	defer file.Close()

	err = json.NewEncoder(file).Encode(&config)

	if err != nil {
		log.Fatal(err)
		return
	}
}

func getNewSession(client *http.Client) Session {
	req, err := http.NewRequest(
		"GET",
		"http://api.guerrillamail.com/ajax.php",
		nil,
	)
	values := req.URL.Query()
	values.Add("f", "get_email_address")
	values.Add("lang", "en")
	req.URL.RawQuery = values.Encode()

	resp, err := client.Do(req)

	var session Session

	if err != nil {
		log.Print(`{"error": "Error fetching email address."}`)
		return session
	}

	defer resp.Body.Close()

	var gsession GuerrillaSession
	err = json.NewDecoder(resp.Body).Decode(&gsession)
	if err != nil {
		log.Fatal(err)
		return session
	}

	session.SID = gsession.SID
	session.Address = gsession.Address
	session.Alias = gsession.Alias
	return session
}

func checkEmails(client *http.Client, config Config) {
	req, err := http.NewRequest(
		"GET",
		"http://api.guerrillamail.com/ajax.php",
		nil,
	)
	values := req.URL.Query()
	values.Add("f", "check_email")
	values.Add("seq", "0")
	values.Add("sid_token", config.Session.SID)
	req.URL.RawQuery = values.Encode()

	resp, err := client.Do(req)

	if err != nil {
		log.Fatal(err)
		return
	}

	defer resp.Body.Close()
}

func getEmails(client *http.Client, config Config) map[string]Email {
	req, err := http.NewRequest(
		"GET",
		"http://api.guerrillamail.com/ajax.php",
		nil,
	)
	values := req.URL.Query()
	values.Add("f", "get_email_list")
	values.Add("sid_token", config.Session.SID)
	values.Add("offset", "0")
	req.URL.RawQuery = values.Encode()

	resp, err := client.Do(req)

	emails := map[string]Email{}

	if err != nil {
		log.Fatal(err)
		return emails
	}

	defer resp.Body.Close()

	var gemails GuerrillaEmails
	err = json.NewDecoder(resp.Body).Decode(&gemails)
	if err != nil {
		log.Fatal(err)
		return emails
	}

	for _, gemail := range gemails.Emails {
		var ok bool
		var id string
		if id, ok = gemail.ID.(string); ok == false {
			id = "1"
		}
		emails[id] = Email{
			id,
			gemail.From,
			gemail.Subject,
			gemail.Body,
		}
	}

	return emails
}

func readEmail(client *http.Client, config Config) Email {
	email := config.Emails[*readID]
	if email.Body != "" {
		return email
	}

	req, err := http.NewRequest(
		"GET",
		"http://api.guerrillamail.com/ajax.php",
		nil,
	)
	values := req.URL.Query()
	values.Add("f", "fetch_email")
	values.Add("email_id", *readID)
	values.Add("sid_token", config.Session.SID)
	req.URL.RawQuery = values.Encode()

	resp, err := client.Do(req)

	if err != nil {
		log.Fatal(err)
		return email
	}

	defer resp.Body.Close()

	var gemail GuerrillaEmail
	err = json.NewDecoder(resp.Body).Decode(&gemail)
	if err != nil {
		log.Fatal(err)
		return email
	}

	var ok bool
	var id string
	if id, ok = gemail.ID.(string); ok == false {
		id = "1"
	}
	email = Email{
		id,
		gemail.From,
		gemail.Subject,
		gemail.Body,
	}

	config.Emails[*readID] = email
	return email
}

func deleteEmail(client *http.Client, config Config) bool {
	if _, ok := config.Emails[*deleteID]; ok == false {
		return false
	}

	req, err := http.NewRequest(
		"GET",
		"http://api.guerrillamail.com/ajax.php",
		nil,
	)
	values := req.URL.Query()
	values.Add("f", "del_email")
	values.Add("email_ids[]", *deleteID)
	values.Add("sid_token", config.Session.SID)
	req.URL.RawQuery = values.Encode()

	resp, err := client.Do(req)

	if err != nil {
		log.Fatal(err)
		return false
	}

	defer resp.Body.Close()

	deletedIDs := struct {
		DeletedIDs []string `json:"deleted_ids"`
	}{}
	err = json.NewDecoder(resp.Body).Decode(&deletedIDs)

	if err != nil {
		log.Fatal(err)
		return false
	}

	for _, id := range deletedIDs.DeletedIDs {
		delete(config.Emails, id)
	}

	if len(deletedIDs.DeletedIDs) > 0 {
		return true
	}

	return false
}
