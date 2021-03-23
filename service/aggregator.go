package service

import (
	"errors"
	fp "path/filepath"
	"sync"

	"github.com/Financial-Times/go-logger/v2"
)

const PanicGuideURL = "https://runbooks.in.ft.com/"

type AggregateSuggester struct {
	Concordance     *ConcordanceService
	BroaderProvider *BroaderConceptsProvider
	Blacklister     ConceptBlacklister
	Suggesters      []Suggester
	Log             *logger.UPPLogger
}

func NewAggregateSuggester(log *logger.UPPLogger, concordance *ConcordanceService, broaderConceptsProvider *BroaderConceptsProvider, blacklister ConceptBlacklister, suggesters ...Suggester) *AggregateSuggester {
	return &AggregateSuggester{
		Concordance:     concordance,
		Suggesters:      suggesters,
		BroaderProvider: broaderConceptsProvider,
		Blacklister:     blacklister,
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
	suggestErrors := []error{}

	var mutex = sync.Mutex{}
	var wg = sync.WaitGroup{}

	for key, suggesterDelegate := range s.Suggesters {
		wg.Add(1)
		logEntry := logEntry
		go func(i int, delegate Suggester) {
			logEntry = logEntry.WithField("suggestions_service", delegate.GetName())
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

			result, sErr := s.getConcordedSuggestions(tid, resp.Suggestions)
			if sErr != nil {
				logEntry.WithError(sErr).Error("failed to get concordances for suggestions")
			} else {
				result = delegate.FilterSuggestions(result)
			}

			// store result even if its empty
			mutex.Lock()
			responseMap[i] = result
			if sErr != nil {
				suggestErrors = append(suggestErrors, sErr)
			}
			mutex.Unlock()
			wg.Done()
		}(key, suggesterDelegate)

	}

	var blacklist Blacklist
	wg.Add(1)
	go func(b Blacklist) {
		defer wg.Done()
		blacklist, err = s.Blacklister.GetBlacklist(tid)
		if err != nil {
			logEntry.WithError(err).Errorf("Error retrieving concept blacklist, filtering disabled")
		}
	}(blacklist)

	wg.Wait()

	if len(suggestErrors) != 0 {
		return aggregateResp, suggestErrors[0]
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
			if !s.Blacklister.IsBlacklisted(suggestion.ID, blacklist) {
				aggregateResp.Suggestions = append(aggregateResp.Suggestions, suggestion)
			}
		}
	}
	return aggregateResp, nil
}

func (s *AggregateSuggester) getConcordedSuggestions(tid string, suggestions []Suggestion) ([]Suggestion, error) {
	logEntry := s.Log.WithTransactionID(tid)

	logEntry.Debug("Calling internal concordances")

	ids := []string{}
	for _, suggestion := range suggestions {
		ids = append(ids, fp.Base(suggestion.Concept.ID))
	}

	ids = dedup(ids)
	if len(ids) == 0 {
		logEntry.Info("No suggestions for calling internal concordances!")
		return nil, nil
	}

	concorded, err := s.Concordance.getConcordances(ids, tid)
	if err != nil {
		return nil, err
	}
	filtered := []Suggestion{}
	for _, suggestion := range suggestions {
		id := fp.Base(suggestion.Concept.ID)
		c, ok := concorded.Concepts[id]
		if !ok {
			continue
		}
		filtered = append(filtered, Suggestion{
			Predicate: suggestion.Predicate,
			Concept:   c,
		})
	}

	logEntry.Debugf("Retained %d of %d concepts using concordances", len(filtered), len(ids))

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
