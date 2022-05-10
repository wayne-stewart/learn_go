package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
)

func main() {
	url := flag.String("url", "http://localhost:8080", "url is required")
	flag.Parse()
	resp, err := http.Get(*url)
	if err != nil {
		log.Fatalln(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	sb := string(body)

	log.Println(sb)
}
