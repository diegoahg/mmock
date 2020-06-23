package match

import (
	"errors"
	"fmt"
	"github.com/jmartin82/mmock/v3/pkg/match/payload"
	"github.com/jmartin82/mmock/v3/pkg/mock"
	"strings"

	urlmatcher "github.com/azer/url-router"

	"github.com/ryanuber/go-glob"
)

var (
	ErrCookiesNotMatch  = errors.New("Cookies not match")
	ErrScenarioNotMatch = errors.New("Scenario state not match")
	ErrPathNotMatch     = errors.New("Path not match")
)

func NewTester(comparator *payload.Comparator, scenario ScenearioStorer) *Request {
	return &Request{scenario: scenario, comparator: comparator}
}

type Request struct {
	scenario   ScenearioStorer
	comparator *payload.Comparator
}

func (mm Request) matchKeyAndValues(reqMap mock.Values, mockMap mock.Values) bool {

	if len(mockMap) > len(reqMap) {

		return false
	}

	for key, mval := range mockMap {
		if rval, exists := reqMap[key]; exists {

			if len(mval) > len(rval) {
				return false
			}

			for i, v := range mval {
				if (!strings.Contains(v, glob.GLOB) && v != rval[i]) || !glob.Glob(v, rval[i]) {
					return false
				}
			}

		} else {
			if rval, exists = mm.findByPartialKey(reqMap, key); exists {

				for i, v := range mval {
					if (!strings.Contains(v, glob.GLOB) && v != rval[i]) || !glob.Glob(v, rval[i]) {
						return false
					}
				}
			} else {
				return false
			}
		}
	}
	return true
}

func (mm Request) findByPartialKey(reqMap mock.Values, partialMatch string) ([]string, bool) {
	if !strings.Contains(partialMatch, glob.GLOB) {
		return []string{}, false
	}

	for key, _ := range reqMap {
		if glob.Glob(partialMatch, key) {
			return reqMap[key], true
		}
	}

	return []string{}, false
}

func (mm Request) matchKeyAndValue(reqMap mock.Cookies, mockMap mock.Cookies) bool {
	if len(mockMap) > len(reqMap) {
		return false
	}
	for key, mval := range mockMap {
		if rval, exists := reqMap[key]; !exists || (!strings.Contains(mval, glob.GLOB) && mval != rval) || !glob.Glob(mval, rval) {
			return false
		}
	}
	return true
}

func (mm Request) matchOnEqualsOrIfEmpty(reqVal string, mockVal string) bool {
	if len(mockVal) == 0 {
		return true
	}
	return strings.ToLower(mockVal) == strings.ToLower(reqVal)
}

func (mm Request) matchOnEqualsOrIfEmptyOrGlob(reqVal string, mockVal string) bool {
	if len(mockVal) == 0 {
		return true
	}
	mockHost := strings.ToLower(mockVal)
	reqHost := strings.ToLower(reqVal)

	return (mockHost == reqHost) || glob.Glob(mockHost, reqHost)
}

func (mm Request) mockIncludesMethod(method string, mock *mock.Request) bool {
	for _, item := range strings.Split(mock.Method, "|") {
		if strings.ToLower(item) == strings.ToLower(method) {
			return true
		}
	}
	return false
}

func (mm Request) matchScenarioState(scenario *mock.Scenario) bool {
	if scenario.Name == "" {
		return true
	}

	currentState := mm.scenario.GetState(scenario.Name)
	for _, r := range scenario.RequiredState {
		if strings.ToLower(r) == currentState {
			return true
		}
	}

	return false
}

//Matcher checks if the received request matches with some specific mock request config.
type Matcher interface {
	Match(req *mock.Request, mock *mock.Definition, scenarioAware bool) (bool, error)
}

func (mm Request) Match(req *mock.Request, mock *mock.Definition, scenarioAware bool) (bool, error) {

	routes := urlmatcher.New(mock.Request.Path)

	if !mm.matchOnEqualsOrIfEmptyOrGlob(req.Host, mock.Request.Host) {
		return false, fmt.Errorf("Host not match. Actual: %s, Expected: %s", req.Host, mock.Request.Host)
	}

	if !mm.matchOnEqualsOrIfEmpty(req.Scheme, mock.Request.Scheme) {
		return false, fmt.Errorf("Scheme not match. Actual: %s, Expected: %s", req.Scheme, mock.Request.Scheme)
	}

	if !mm.matchOnEqualsOrIfEmpty(req.Fragment, mock.Request.Fragment) {
		return false, fmt.Errorf("Fragment not match. Actual: %s, Expected: %s", req.Fragment, mock.Request.Fragment)
	}

	if !glob.Glob(mock.Request.Path, req.Path) && routes.Match(req.Path) == nil {
		return false, fmt.Errorf("Path not match. Actual: %s, Expected: %s", req.Path, mock.Request.Path)
	}

	if !mm.mockIncludesMethod(req.Method, &mock.Request) {
		return false, fmt.Errorf("Method not match. Actual: %s, Expected: %s", req.Method, mock.Request.Method)
	}

	if !mm.matchKeyAndValues(req.QueryStringParameters, mock.Request.QueryStringParameters) {
		return false, fmt.Errorf("Query string not match. Actual: %s, Expected: %s", mm.ValuesToString(req.QueryStringParameters), mm.ValuesToString(mock.Request.QueryStringParameters))
	}

	if !mm.matchKeyAndValue(req.Cookies, mock.Request.Cookies) {
		return false, ErrCookiesNotMatch
	}

	if !mm.matchKeyAndValues(req.Headers, mock.Request.Headers) {
		return false, fmt.Errorf("Headers not match. Actual: %s, Expected: %s", req.Headers, mock.Request.Headers)
	}

	if !mm.bodyMatch(mock.Request, req) {
		return false, fmt.Errorf("Body not match. Actual: %s, Expected: %s", req.Body, mock.Request.Body)
	}

	if scenarioAware && !mm.matchScenarioState(&mock.Control.Scenario) {
		return false, ErrScenarioNotMatch
	}

	return true, nil
}
func (mm Request) bodyMatch(mockReq mock.Request, req *mock.Request) bool {

	if len(mockReq.Body) == 0 {
		return true
	}

	if mockReq.Body == req.Body {
		return true
	}

	if strings.Index(mockReq.Body, glob.GLOB) > -1 && glob.Glob(mockReq.Body, req.Body) {
		return true
	}

	if value, ok := req.Headers["Content-Type"]; ok && len(value) > 0 {
		if comparable, ok := mm.comparator.Compare(value[0], mockReq.Body, req.Body); comparable {
			return ok
		}
	}

	return false
}

func (mm Request) ValuesToString(values mock.Values) string {
	var valuesStr []string

	for name, value := range values {
		name = strings.ToLower(name)
		for _, h := range value {
			valuesStr = append(valuesStr, fmt.Sprintf("%v: %v", name, h))
		}
	}

	return strings.Join(valuesStr, ", ")
}
