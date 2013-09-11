package main

import (
	"errors"
	"fmt"
	"github.com/mrjones/oauth"
	"io/ioutil"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func StripPassword(url url.URL) url.URL {
	url.User = nil
	return url
}

func prompt(msg string) (input string, err error) {
	fmt.Print(msg)
	res := &input
	_, err = fmt.Scanln(res)
	return
}

func requireAuth(u url.URL) (user, password string, err error) {
	user, password, err = keychainAuth(u)
	if err != nil {
		fmt.Println("Error in keychain auth: " + err.Error())
		if user, err = prompt("User: "); err != nil {
			return
		}
		if password, err = prompt("Password: "); err != nil {
			return
		}
	}
	return
}

func keychainAuth(u url.URL) (username, password string, err error) {
	//TODO: Replace this with proper api accessing keychain
	host := findHost(u.Host)
	if host == "" {
		err = errors.New("No Keychain item found")
		return
	}

	securityCmd := "/usr/bin/security"
	securitySubCmd := "find-internet-password"
	cmd := exec.Command(securityCmd, securitySubCmd, "-gs", host)
	b, err := cmd.CombinedOutput()
	output := string(b)
	if err != nil {
		return
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "\"acct\"") {
			username = line[18 : len(line)-1]
		}
		if strings.Contains(line, "password: ") {
			password = line[11 : len(line)-1]
		}
	}
	return
}

func findHost(host string) (result string) {
	securityCmd := "/usr/bin/security"
	securitySubCmd := "dump-keychain"
	cmd := exec.Command(securityCmd, securitySubCmd)
	b, err := cmd.CombinedOutput()
	out := string(b)
	if err != nil {
		return
	}
	r := regexp.MustCompile(`srvr"<blob>="(.*?)"`)
	ms := r.FindAllStringSubmatch(out, -1)
	for _, m := range ms {
		name := m[1]
		// println(host, name)
		if name == host || name == "www."+host {
			// println(name)
			return name
		}
	}
	return ""
}

func handleOauth() (token *oauth.AccessToken, err error) {
	tumbUri, _ := url.Parse("http://tumblr.com")
	user, pass, err := keychainAuth(*tumbUri)
	tumbApi, _ := url.Parse("http://api.tumblr.com")
	key, secret, err := keychainAuth(*tumbApi)
	check(err)
	cons := oauth.NewConsumer(key, secret, oauth.ServiceProvider{
		RequestTokenUrl:   "http://www.tumblr.com/oauth/request_token",
		AuthorizeTokenUrl: "https://www.tumblr.com/oauth/authorize",
		AccessTokenUrl:    "https://www.tumblr.com/oauth/access_token",
	})
	// cons.Debug(true)
	requestToken, userUri, err := cons.GetRequestTokenAndUrl("https://localhost")
	check(err)
	// println(userUri)
	verifier := OAuth(userUri, user, pass)
	// println(verifier)
	token, err = cons.AuthorizeToken(requestToken, verifier)
	check(err)
	return
}

func OAuth(uri, user, pass string) (redirect string) {
	js := `var casper = require("casper").create({
    // verbose: true,
    // logLevel: "debug"
});

casper.start(casper.cli.args[0], function() {
	// TODO: Transfer user and password through stdin
	this.fill("form#signup_form",{
		"user[email]":    casper.cli.args[1],
		"user[password]": casper.cli.args[2]
	}, true);
});

casper.then(function(){
	this.mouseEvent('click', 'button[name=allow]')
});

casper.then(function(response){
	console.log(response.url);
});

casper.run();`
	base, rm := TempDir()
	defer rm()
	jspath := filepath.Join(base, "oauth.js")
	err := ioutil.WriteFile(jspath, []byte(js), 0777)
	check(err)
	c := exec.Command("/usr/bin/env", "casperjs", jspath, uri, user, pass)
	out, _ := c.CombinedOutput()
	check(err)
	u, err := url.Parse(string(out))
	check(err)
	verifier := u.Query().Get("oauth_verifier")
	return verifier
}
