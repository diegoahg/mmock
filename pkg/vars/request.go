package vars

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/jmartin82/mmock/v3/pkg/mock"

	urlmatcher "github.com/azer/url-router"
	xj "github.com/basgys/goxml2json"
	"github.com/tidwall/gjson"
)

type Request struct {
	Mock    *mock.Definition
	Request *mock.Request
}

func (rp Request) Fill(holders []string) map[string][]string {

	vars := make(map[string][]string)
	for _, tag := range holders {
		found := false
		s := ""
		if tag == "request.body" && rp.Request.Body != "" {
			s = rp.Request.Body
			found = true
		} else if tag == "request.scheme" {
			s, found = rp.Request.Scheme, true
		} else if tag == "request.port" {
			s, found = rp.Request.Port, true
		} else if tag == "request.url" {
			s, found = rp.getUrl()
		} else if tag == "request.authority" {
			s, found = rp.getAuthority()
		} else if tag == "request.hostname" {
			s, found = rp.Request.Host, true
		} else if tag == "request.path" {
			s, found = rp.Request.Path, true
		} else if tag == "request.fragment" {
			s, found = rp.Request.Fragment, true
		} else if strings.HasPrefix(tag, "request.body.") {
			s, found = rp.getBodyParam(tag[13:])
		} else if strings.HasPrefix(tag, "request.query.") {
			s, found = rp.getQueryStringParam(tag[14:])
		} else if strings.HasPrefix(tag, "request.path.") {
			s, found = rp.getPathParam(tag[13:])
		} else if strings.HasPrefix(tag, "request.cookie.") {
			s, found = rp.getCookieParam(tag[15:])
		} else if strings.HasPrefix(tag, "request.header.") {
			s, found = rp.getHeaderParam(tag[15:])
		} else if strings.HasPrefix(tag, "env.") {
			s, found = os.LookupEnv(tag[4:])
		}

		if found {
			vars[tag] = append(vars[tag], s)
		}

	}
	return vars
}

func (rp Request) getAuthority() (string, bool) {
	if len(rp.Request.Port) == 0 || rp.Request.Port == "80" {
		return fmt.Sprintf("%s://%s", rp.Request.Scheme, rp.Request.Host), true
	}

	return fmt.Sprintf("%s://%s:%s", rp.Request.Scheme, rp.Request.Host, rp.Request.Port), true
}

func (rp Request) getUrl() (string, bool) {
	value, f := rp.getAuthority()

	if !f {
		return "", false
	}

	path := rp.Request.Path

	if len(path) != 0 {
		value += rp.Request.Path
	}

	queryStringParams := rp.Request.QueryStringParameters

	if len(queryStringParams) != 0 {
		queryKeys := []string{}
		queryVars := []string{}

		//make predictable
		for key := range queryStringParams {
			queryKeys = append(queryKeys, key)
		}
		sort.Strings(queryKeys)

		for _, key := range queryKeys {
			for _, value := range queryStringParams[key] {
				queryVars = append(queryVars, fmt.Sprintf("%s=%s", key, value))
			}
		}
		value += "?" + strings.Join(queryVars, "&")
	}

	if len(rp.Request.Fragment) != 0 {
		value += "#" + rp.Request.Fragment
	}

	return value, true
}

func (rp Request) getPathParam(name string) (string, bool) {

	routes := urlmatcher.New(rp.Mock.Request.Path)
	mparm := routes.Match(rp.Request.Path)

	value, f := mparm.Params[name]
	if !f {
		return "", false
	}

	return value, true
}

func (rp Request) getQueryStringParam(name string) (string, bool) {

	if len(rp.Request.QueryStringParameters) == 0 {
		return "", false
	}
	value, f := rp.Request.QueryStringParameters[name]
	if !f {
		return "", false
	}

	return value[0], true
}

func (rp Request) getCookieParam(name string) (string, bool) {

	if len(rp.Request.Cookies) == 0 {
		return "", false
	}
	value, f := rp.Request.Cookies[name]
	if !f {
		return "", false
	}

	return value, true
}

func (rp Request) getHeaderParam(name string) (string, bool) {

	value, f := rp.Request.HttpHeaders.Headers[name]
	if !f || len(rp.Request.HttpHeaders.Headers) == 0 {
		return "", false
	}
	if len(value) == 0 {
		return "", false
	}

	return value[0], true
}

func (rp Request) getBodyParam(name string) (string, bool) {
	contentType, found := rp.Request.Headers["Content-Type"]
	if !found {
		return "", false
	}
	if strings.HasPrefix(contentType[0], "application/x-www-form-urlencoded") {
		return rp.getUrlEncodedFormBodyParam(name)
	} else if strings.HasPrefix(contentType[0], "application/xml") || strings.HasPrefix(contentType[0], "text/xml") {
		return rp.getXmlBodyParam(name)
	} else if strings.HasPrefix(contentType[0], "application/json") {
		return rp.getJsonBodyParam(name)
	}

	return "", false
}

func (rp Request) getXmlBodyParam(name string) (string, bool) {
	xml := strings.NewReader(rp.Request.Body)
	json, err := xj.Convert(xml)
	if err != nil {
		return "", false
	}

	value := gjson.Get(json.String(), name)
	if !value.Exists() {
		return "", false
	}

	//TODO: Add support to complex types extraction like arrays or maps
	if value.Type == gjson.JSON {
		return "", false
	}

	return value.String(), true
}

func (rp Request) getJsonBodyParam(name string) (string, bool) {
	value := gjson.Get(rp.Request.Body, name)
	if !value.Exists() {
		return "", false
	}
	return value.String(), true
}

func (rp Request) getUrlEncodedFormBodyParam(name string) (string, bool) {

	values, err := url.ParseQuery(rp.Request.Body)
	if err != nil {
		return "", false
	}

	value := values.Get(name)
	if value == "" {
		return "", false
	}

	return value, true
}
