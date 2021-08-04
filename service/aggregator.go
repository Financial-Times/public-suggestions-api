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

	var aggregateResp = SuggestionsResponse{Suggestions: make([]Suggestion, 0)}
	var responseMap = map[int][]Suggestion{}
	type suggestionFailure struct {
		name string
		err  error
	}
	suggestFails := []suggestionFailure{}

	var mutex = sync.Mutex{}
	var wg = sync.WaitGroup{}

	for key, suggesterDelegate := range s.Suggesters {
		wg.Add(1)
		go func(i int, delegate Suggester) {
			defer wg.Done()
			result, err := getSuggestions(delegate, s.Concordance, tid, payload) //nolint: govet

			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				responseMap[i] = []Suggestion{}
				suggestFails = append(suggestFails, suggestionFailure{name: delegate.GetName(), err: err})
			} else {
				responseMap[i] = result
			}
		}(key, suggesterDelegate)
	}

	var blacklist Blacklist
	var err error
	wg.Add(1)
	go func(b Blacklist) {
		defer wg.Done()
		blacklist, err = s.Blacklister.GetBlacklist(tid)
		if err != nil {
			logEntry.WithError(err).Errorf("Error retrieving concept blacklist, filtering disabled")
		}
	}(blacklist)

	wg.Wait()

	var nonSuggestErr error
	for _, fail := range suggestFails {
		var sErr *SuggesterErr
		if !errors.As(fail.err, &sErr) {
			nonSuggestErr = fail.err
			continue
		}
		msg := "error calling " + fail.name
		errEntry := logEntry.WithField("suggestions_service", fail.name).WithError(sErr)
		if errors.Is(sErr, NoContentError) || errors.Is(sErr, BadRequestError) {
			errEntry.Warn(msg)
		} else {
			errEntry.Error(msg)
		}
	}
	if nonSuggestErr != nil {
		return aggregateResp, nonSuggestErr
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

// getSuggestions requests suggestions from the Suggester delegate for the provided payload.
// It enriches the suggestions with concept data gathered from the ConcordanceService.
// If the delegate fails to provide suggestions, this function returns suggesterErr error that wraps the delegate error
// This is done in order to distinguish between errors coming from the Suggester and the ones from ConcordanceService
func getSuggestions(delegate Suggester, concordance *ConcordanceService, tid string, payload []byte) ([]Suggestion, error) {
	resp, err := delegate.GetSuggestions(payload, tid)
	if err != nil {
		return nil, err
	}

	result, err := enrichSuggestionsWithConceptData(concordance, tid, resp.Suggestions)
	if err != nil {
		return nil, err
	}
	result = delegate.FilterSuggestions(result)

	return result, nil
}

// enrichSuggestionsWithConceptData uses ConcordanceService to gather more information for the suggested concepts.
func enrichSuggestionsWithConceptData(concordance *ConcordanceService, tid string, suggestions []Suggestion) ([]Suggestion, error) {
	ids := []string{}
	for _, suggestion := range suggestions {
		ids = append(ids, fp.Base(suggestion.Concept.ID))
	}

	ids = dedup(ids)
	if len(ids) == 0 {
		return nil, nil
	}

	concorded, err := concordance.getConcordances(ids, tid)
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
