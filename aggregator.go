package main

type SuggestionsAggregator struct {
}

type suggestionsType struct {
	Predicate      string `json:"predicate"`
	Id             string `json:"id"`
	ApiUrl         string `json:"apiUrl"`
	PrefLabel      string `json:"prefLabel"`
	SuggestionType string `json:"type"`
	IsFTAuthor     bool   `json:"isFTAuthor"`
}

func (aggregator *SuggestionsAggregator) sampleMessage() []suggestionsType {
	return []suggestionsType{
		{
			Predicate:      "http://www.ft.com/ontology/annotation/mentions",
			Id:             "http://www.ft.com/thing/6f14ea94-690f-3ed4-98c7-b926683c735a",
			ApiUrl:         "http://api.ft.com/people/6f14ea94-690f-3ed4-98c7-b926683c735a",
			PrefLabel:      "Donald Kaberuka",
			SuggestionType: "http://www.ft.com/ontology/person/Person",
			IsFTAuthor:     false,
		},
		{
			Predicate:      "http://www.ft.com/ontology/annotation/mentions",
			Id:             "http://www.ft.com/thing/9a5e3b4a-55da-498c-816f-9c534e1392bd",
			ApiUrl:         "http://api.ft.com/people/9a5e3b4a-55da-498c-816f-9c534e1392bd",
			PrefLabel:      "Lawrence Summers",
			SuggestionType: "http://www.ft.com/ontology/person/Person",
			IsFTAuthor:     true,
		},
	}
}
