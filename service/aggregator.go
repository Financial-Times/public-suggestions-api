package service

import (
	fp "path/filepath"
	"sync"

	log "github.com/Financial-Times/go-logger"
)

const idsParamName = "ids"

type AggregateSuggester struct {
	DefaultSource map[string]string
	Concordance   *ConcordanceService
	Suggesters    []Suggester
}

func NewAggregateSuggester(concordance *ConcordanceService, defaultTypesSources map[string]string, suggesters ...Suggester) *AggregateSuggester {
	return &AggregateSuggester{
		Concordance:   concordance,
		DefaultSource: defaultTypesSources,
		Suggesters:    suggesters,
	}
}

func (suggester *AggregateSuggester) GetSuggestions(payload []byte, tid string, flags SourceFlags) (SuggestionsResponse, error) {
	data, err := getXmlSuggestionRequestFromJson(payload)
	if flags.Debug != "" {
		log.WithTransactionID(tid).WithField("debug", flags.Debug).Info(string(data))
	}
	if err != nil {
		data = payload
	}
	var aggregateResp = SuggestionsResponse{Suggestions: make([]Suggestion, 0)}

	var mutex = sync.Mutex{}
	var wg = sync.WaitGroup{}

	var responseMap = map[int][]Suggestion{}
	for key, suggesterDelegate := range suggester.Suggesters {
		wg.Add(1)
		go func(i int, delegate Suggester) {
			resp, err := delegate.GetSuggestions(data, tid, flags)
			if err != nil {
				if err == NoContentError || err == BadRequestError {
					log.WithTransactionID(tid).WithField("tid", tid).Warn(err.Error())
				} else {
					log.WithTransactionID(tid).WithField("tid", tid).WithError(err).Errorf("Error calling %v", delegate.GetName())
				}
			}
			mutex.Lock()
			responseMap[i] = resp.Suggestions
			mutex.Unlock()
			wg.Done()
		}(key, suggesterDelegate)
	}
	wg.Wait()

	responseMap, err = suggester.filterByInternalConcordances(responseMap, tid, flags.Debug)
	if err != nil {
		return aggregateResp, err
	}

	for key, suggesterDelegate := range suggester.Suggesters {
		if len(responseMap[key]) > 0 {
			responseMap[key] = suggesterDelegate.FilterSuggestions(responseMap[key], flags)
		}
	}

	// preserve results order
	for i := 0; i < len(suggester.Suggesters); i++ {
		aggregateResp.Suggestions = append(aggregateResp.Suggestions, responseMap[i]...)
	}
	return aggregateResp, nil
}

func (suggester *AggregateSuggester) filterByInternalConcordances(s map[int][]Suggestion, tid string, debugFlag string) (map[int][]Suggestion, error) {
	if debugFlag != "" {
		log.WithTransactionID(tid).WithField("debug", debugFlag).Info("Calling internal concordances")
	}
	var filtered = map[int][]Suggestion{}
	var concorded ConcordanceResponse

	var ids []string
	for i := 0; i < len(s); i++ {
		for _, suggestion := range s[i] {
			ids = append(ids, fp.Base(suggestion.Concept.ID))
		}
	}

	ids = dedup(ids)

	if len(ids) == 0 {
		log.WithTransactionID(tid).Info("No suggestions for calling internal concordances!")
		return filtered, nil
	}

	concorded, err := suggester.Concordance.getConcordances(ids, tid, debugFlag)
	if err != nil {
		return filtered, err
	}

	total := 0
	for index, suggestions := range s {
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

	if debugFlag != "" {
		log.WithTransactionID(tid).WithField("debug", debugFlag).Infof("Retained %v of %v concepts using concordances", total, len(ids))
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
