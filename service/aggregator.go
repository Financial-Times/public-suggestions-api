package service

import (
	"errors"
	fp "path/filepath"
	"sync"

	"github.com/Financial-Times/go-logger/v2"
)

const PanicGuideURL = "https://runbooks.in.ft.com/"

type ConceptFilter interface {
	IsAllowed(uuid string, bl Blacklist) bool
	GetBlacklist(tid string) (Blacklist, error)
}

type AggregateSuggester struct {
	Concordance     *ConcordanceService
	BroaderProvider *BroaderConceptsProvider
	Filter          ConceptFilter
	Suggesters      []Suggester
	Log             *logger.UPPLogger
}

func NewAggregateSuggester(log *logger.UPPLogger, concordance *ConcordanceService, broaderConceptsProvider *BroaderConceptsProvider, filter ConceptFilter, suggesters ...Suggester) *AggregateSuggester {
	return &AggregateSuggester{
		Concordance:     concordance,
		Suggesters:      suggesters,
		BroaderProvider: broaderConceptsProvider,
		Filter:          filter,
		Log:             log,
	}
}

func (s *AggregateSuggester) GetSuggestions(payload []byte, tid string) (SuggestionsResponse, error) {
	logEntry := s.Log.WithTransactionID(tid)

	data, err := getXmlSuggestionRequestFromJson(payload)
	if err != nil {
		data = payload
	}

	logEntry.Debugf("transformed payload: %s", string(data))

	var aggregateResp = SuggestionsResponse{Suggestions: make([]Suggestion, 0)}
	var responseMap = map[int][]Suggestion{}

	var mutex = sync.Mutex{}
	var wg = sync.WaitGroup{}

	for key, suggesterDelegate := range s.Suggesters {
		wg.Add(1)
		logEntry := logEntry
		go func(i int, delegate Suggester) {
			resp, sErr := delegate.GetSuggestions(data, tid)
			if sErr != nil {
				errMsg := "error calling " + delegate.GetName()
				errEntry := logEntry.WithError(sErr)
				if errors.Is(sErr, NoContentError) || errors.Is(sErr, BadRequestError) {
					errEntry.Warn(errMsg)
				} else {
					errEntry.Error(errMsg)
				}
			}
			mutex.Lock()
			responseMap[i] = resp.Suggestions
			mutex.Unlock()
			wg.Done()
		}(key, suggesterDelegate)

	}

	var blacklist Blacklist
	wg.Add(1)
	go func(b Blacklist) {
		defer wg.Done()
		blacklist, err = s.Filter.GetBlacklist(tid)
		if err != nil {
			logEntry.WithError(err).Errorf("Error retrieving concept blacklist, filtering disabled")
		}
	}(blacklist)

	wg.Wait()

	responseMap, err = s.filterByInternalConcordances(responseMap, tid)
	if err != nil {
		return aggregateResp, err
	}

	for key, suggesterDelegate := range s.Suggesters {
		if len(responseMap[key]) > 0 {
			responseMap[key] = suggesterDelegate.FilterSuggestions(responseMap[key])
		}
	}

	results, err := s.BroaderProvider.excludeBroaderConceptsFromResponse(responseMap, tid)
	if err != nil {
		logEntry.WithError(err).Warn("Couldn't exclude broader concepts. Response might contain broader concepts as well")
	} else {
		responseMap = results
	}

	// preserve results order
	for i := 0; i < len(s.Suggesters); i++ {
		for _, suggestion := range responseMap[i] {
			if s.Filter.IsAllowed(suggestion.ID, blacklist) {
				aggregateResp.Suggestions = append(aggregateResp.Suggestions, suggestion)
			}
		}
	}
	return aggregateResp, nil
}

func (s *AggregateSuggester) filterByInternalConcordances(suggestions map[int][]Suggestion, tid string) (map[int][]Suggestion, error) {
	logEntry := s.Log.WithTransactionID(tid)

	logEntry.Debug("Calling internal concordances")

	var filtered = map[int][]Suggestion{}
	var concorded ConcordanceResponse

	var ids []string
	for i := 0; i < len(suggestions); i++ {
		for _, suggestion := range suggestions[i] {
			ids = append(ids, fp.Base(suggestion.Concept.ID))
		}
	}

	ids = dedup(ids)

	if len(ids) == 0 {
		logEntry.Info("No suggestions for calling internal concordances!")
		return filtered, nil
	}

	concorded, err := s.Concordance.getConcordances(ids, tid)
	if err != nil {
		return filtered, err
	}

	total := 0
	for index, suggestions := range suggestions {
		filtered[index] = []Suggestion{}
		for _, suggestion := range suggestions {
			id := fp.Base(suggestion.Concept.ID)
			c, ok := concorded.Concepts[id]
			if ok {
				filtered[index] = append(filtered[index], Suggestion{
					Predicate: suggestion.Predicate,
					Concept:   c,
				})
			}
		}
		total += len(filtered[index])
	}

	logEntry.Debugf("Retained %v of %v concepts using concordances", total, len(ids))

	return filtered, nil
}

func dedup(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	j := 0
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		s[j] = v
		j++
	}
	return s[:j]
}
