package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type ConceptBlacklister interface {
	IsBlacklisted(uuid string, bl Blacklist) bool
	GetBlacklist(tid string) (Blacklist, error)
}

type Blacklister struct {
	baseUrl  string
	endpoint string
	client   Client
}

type Blacklist struct {
	Uuids []string `json:"uuids"`
}

func NewConceptBlacklister(baseUrl string, endpoint string, client Client) ConceptBlacklister {
	return &Blacklister{baseUrl, endpoint, client}
}

func (b *Blacklister) IsBlacklisted(uuid string, bl Blacklist) bool {
	return contains(bl.Uuids, uuid)
}

func (b *Blacklister) GetBlacklist(tid string) (Blacklist, error) {
	req, err := http.NewRequest("GET", b.baseUrl+b.endpoint, nil)
	if err != nil {
		return Blacklist{}, err
	}

	req.Header.Add("User-Agent", "UPP public-suggestions-api")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-Request-Id", tid)

	resp, err := b.client.Do(req)
	if err != nil {
		return Blacklist{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Blacklist{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return Blacklist{}, fmt.Errorf("concept-suggestions-blacklister returned HTTP %v", resp.StatusCode)
	}

	var blacklist Blacklist
	err = json.Unmarshal(body, &blacklist)
	if err != nil {
		return Blacklist{}, err
	}
	return blacklist, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
