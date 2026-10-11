package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
)

const beeSuggestionLimit = 100

// isDotSegment reports whether the name is "." or "..", which the API's router would collapse out of a path.
func isDotSegment(name string) bool {
	return name == "." || name == ".."
}

func loadBeeSuggestions(a apiSession, focusList bool) (components.BeeSuggestionsData, error) {
	list, err := apiGet[struct {
		Suggestions []string `json:"suggestions"`
	}](a, "/bee-name-generator/suggestion/"+strconv.Itoa(beeSuggestionLimit), "Failed to load suggestions")
	if err != nil {
		return components.BeeSuggestionsData{}, err
	}
	return components.BeeSuggestionsData{Names: list.Suggestions, FocusList: focusList}, nil
}

func beeSuggestionsHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
	data, err := loadBeeSuggestions(a, false)
	if err != nil {
		return err
	}
	renderAll(w, r, components.BeeSuggestions(data))
	return nil
}

func beeReviewHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
	name := r.Form.Get("name")
	var method, fallback string
	switch r.Form.Get("action") {
	case "accept":
		method, fallback = http.MethodPut, "Failed to accept the suggestion"
	case "reject":
		method, fallback = http.MethodDelete, "Failed to reject the suggestion"
	default:
		return invalidInput("Choose accept or reject")
	}
	if strings.TrimSpace(name) == "" || isDotSegment(name) {
		return invalidInput("That name cannot be reviewed here")
	}
	if _, err := apiSend[struct{}](a, method, "/bee-name-generator/suggestion/"+url.PathEscape(name), nil, fallback); err != nil {
		return err
	}
	data, err := loadBeeSuggestions(a, true)
	if err != nil {
		return pageFail(err).afterWrite()
	}
	renderAll(w, r, components.BeeSuggestions(data))
	return nil
}
