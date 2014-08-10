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

// структура для парсинга ответа от api
type AllResponse stru	ct {
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

// и опять настраиваемые параметры
var (
	yourIpUrl = flag.String("url", "https://cydev.ru/ip", "Yourip service url")
	domain    = flag.String("domain", "cydev.ru", "Cloudflare domain")
	target    = flag.String("target", "me.cydev.ru", "Target domain")
	email     = flag.String("email", "ernado@ya.ru", "The e-mail address associated with the API key")
	token     = flag.String("token", "-", "This is the API key made available on your Account page")
	ttl       = flag.Int("ttl", 120, "TTL of record in seconds. 1 = Automatic, otherwise, value must in between 120 and 86400 seconds")
	// http клиент - у него есть метод .Do, который нам пригодится
	client = http.Client{}
)

// зададим заранее некоторые поля, чтобы не повторяться
func Url() (u url.URL) {
	u.Host = "www.cloudflare.com"
	u.Scheme = "https"
	u.Path = "api_json.html"
	return
}

// SetIp устанавливает значение записи с заданным id
func SetIp(ip string, id int) error {
	u := Url()
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
		return errors.New(fmt.Sprintf("bad status %d", res.StatusCode))
	}
	return nil
}

// GetDnsId вовзращает id записи и её текущее значение
func GetDnsId() (int, string, error) {
	log.Println("getting dns record id")
	// начнем собирать url
	u := Url()
	// добавим дополнительные параметры
	values := u.Query()
	values.Add("email", *email)
	values.Add("tkn", *token)
	values.Add("a", "rec_load_all")
	values.Add("z", *domain)
	u.RawQuery = values.Encode()
	reqUrl := u.String()
	// создадим запрос, выполним его и проверим результат
	log.Println("POST", reqUrl)
	req, err := http.NewRequest("POST", reqUrl, nil)
	res, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	if res.StatusCode != http.StatusOK {
		return 0, "", errors.New(fmt.Sprintf("bad status %d", res.StatusCode))
	}
	response := &AllResponse{}
	// создадим декодер
	decoder := json.NewDecoder(res.Body)
	// и распарсим ответ сервера в нашу структуру
	err = decoder.Decode(response)
	if err != nil {
		return 0, "", err
	}
	// пройдемся по всем записям
	for _, v := range response.Response.Records.Objects {
		// и найдем запись нужного типа и имени
		if v.Name == *target && v.Type == "A" {
			// конвертируем из строки в число идентификатор
			id, _ := strconv.Atoi(v.Id)
			return id, v.Content, nil
		}
	}
	// нужная нам запись не найдена
	return 0, "", errors.New("not found")
}

// GetIp() обращается к yourip сервису и возвращает наш ip адрес
func GetIp() (string, error) {
	res, err := client.Get(*yourIpUrl)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", errors.New(fmt.Sprintf("bad status %d", res.StatusCode))
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func main() {
	flag.Parse()
	id, previousIp, err := GetDnsId()
	if err != nil {
		log.Fatalln("unable to get dns record id:", err)
	}
	log.Println("found record", id, "=", previousIp)
	// создадим тикер, который позволит нам удобно каждые
	// 5 секунд проверять ip адрес
	ticker := time.NewTicker(time.Second * 5)
	// начнем наш бесконечный цикл
	for _ = range ticker.C {
		ip, err := GetIp()
		if err != nil {
			log.Println("err", err)
			continue
		}
		if previousIp != ip {
			err = SetIp(ip, id)
			if err != nil {
				log.Println("unable to set ip:", err)
				continue
			}
		}
		log.Println("updated to", ip)
		previousIp = ip
	}
}
