package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type AllResponse struct {
	Response struct {
		Records struct {
			Objects []struct {
				Id      string `json:"rec_id"`
				Name    string `json:"name"`
				Type    string `json:"type"`
				Content string `json:"content"`
			} `json:"objs"`
		} `json:"recs"`
	} `json:"response"`
}

var (
	yourIpUrl = flag.String("url", "https://cydev.ru/ip", "Yourip service url")
	domain    = flag.String("domain", "cydev.ru", "Cloudflare domain")
	target    = flag.String("target", "me.cydev.ru", "Target domain")
	email     = flag.String("email", "ernado@ya.ru", "The e-mail address associated with the API key")
	token     = flag.String("token", "-", "This is the API key made available on your Account page")
	ttl       = flag.Int("ttl", 120, "TTL of record in seconds. 1 = Automatic, otherwise, value must in between 120 and 86400 seconds")
	client    = http.Client{}
	ipUpdates = make(chan string)
)

func SetIp(ip string, id int) error {
	log.Println("setting ip")
	u := url.URL{}
	u.Host = "www.cloudflare.com"
	u.Scheme = "https"
	u.Path = "api_json.html"

	values := u.Query()
	values.Add("email", *email)
	values.Add("tkn", *token)
	values.Add("a", "rec_edit")
	values.Add("z", *domain)
	values.Add("type", "A")
	values.Add("name", *target)
	values.Add("service_mode", "0")
	values.Add("content", ip)
	values.Add("id", strconv.Itoa(id))
	values.Add("ttl", fmt.Sprint(*ttl))
	u.RawQuery = values.Encode()

	reqUrl := u.String()
	log.Println("POST", reqUrl)
	req, err := http.NewRequest("POST", reqUrl, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		log.Println("got status", res.StatusCode)
		return errors.New("bad status")
	}

	// body, _ := ioutil.ReadAll(res.Body)
	// log.Println(string(body))

	return nil
}

func GetDnsId() (int, string, error) {
	log.Println("getting dns id")
	u := url.URL{}
	u.Host = "www.cloudflare.com"
	u.Scheme = "https"
	u.Path = "api_json.html"

	values := u.Query()
	values.Add("email", *email)
	values.Add("tkn", *token)
	values.Add("a", "rec_load_all")
	values.Add("z", *domain)
	u.RawQuery = values.Encode()

	reqUrl := u.String()
	log.Println("POST", reqUrl)
	req, err := http.NewRequest("POST", reqUrl, nil)
	res, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	if res.StatusCode != http.StatusOK {
		return 0, "", errors.New("bad code")
	}
	response := &AllResponse{}
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(response)
	if err != nil {
		return 0, "", err
	}
	for _, v := range response.Response.Records.Objects {
		if v.Name == *target && v.Type == "A" {
			id, err := strconv.Atoi(v.Id)
			if err != nil {
				break
			}
			return id, v.Content, nil
		}
	}
	return 0, "", errors.New("not found")
}

func GetIp() (string, error) {
	res, err := client.Get(*yourIpUrl)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", errors.New("bad status")
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func main() {
	log.Println("starting")
	flag.Parse()
	id, previousIp, err := GetDnsId()
	if err != nil {
		log.Fatalln("unable to get dns record id:", err)
	}
	log.Println("found record", id, "=", previousIp)

	ticker := time.NewTicker(time.Second * 5)
	for _ = range ticker.C {
		ip, err := GetIp()
		if err != nil {
			log.Println("err")
			continue
		}
		if previousIp != ip {
			err = SetIp(ip, id)
			if err != nil {
				log.Fatal("unable to set ip:", err)
			}
			log.Println("updated to", ip)
		}
		previousIp = ip
	}
}
