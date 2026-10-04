package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
)

// beeSuggestionLimit is how many pending suggestions one review page reads.
const beeSuggestionLimit = 100

// isDotSegment reports whether the name is "." or "..", which the API's router would collapse out of a path.
func isDotSegment(name string) bool {
	return name == "." || name == ".."
}

func loadBeeSuggestions(a adminAPI, focusList bool) (components.BeeSuggestionsData, error) {
	list, err := adminGet[struct {
		Suggestions []string `json:"suggestions"`
	}](a, "/bee-name-generator/suggestion/"+strconv.Itoa(beeSuggestionLimit), "Failed to load suggestions")
	if err != nil {
		return components.BeeSuggestionsData{}, err
	}
	return components.BeeSuggestionsData{Names: list.Suggestions, FocusList: focusList}, nil
}

func beeSuggestionsHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	data, err := loadBeeSuggestions(a, false)
	if err != nil {
		failFragment(w, r, err)
		return
	}
	renderAll(w, r, components.BeeSuggestions(data))
}

func beeReviewHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	name := r.PostForm.Get("name")
	method := http.MethodDelete
	fallback := "Failed to reject the suggestion"
	switch r.PostForm.Get("action") {
	case "accept":
		method = http.MethodPut
		fallback = "Failed to accept the suggestion"
	case "reject":
	default:
		failFragment(w, r, invalidInput("Choose accept or reject"))
		return
	}
	if strings.TrimSpace(name) == "" || isDotSegment(name) {
		failFragment(w, r, invalidInput("That name can't be reviewed here"))
		return
	}
	if _, err := adminSend[struct{}](a, method, "/bee-name-generator/suggestion/"+url.PathEscape(name), nil, fallback); err != nil {
		failFragment(w, r, err)
		return
	}
	data, err := loadBeeSuggestions(a, true)
	if err != nil {
		failFragment(w, r, afterWrite(err))
		return
	}
	renderAll(w, r, components.BeeSuggestions(data))
}
