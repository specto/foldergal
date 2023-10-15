package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const (
	QueryDisplay = iota
	QueryOrder
	QuerySort
)

const (
	QueryDisplayShow    = "slideshow"
	QueryDisplayFile    = "direct"
	QueryDisplayDefault = QueryDisplayShow
	QueryOrderAsc       = "asc"
	QueryOrderDesc      = "desc"
	QueryOrderDefault   = QueryOrderDesc
	QuerySortName       = "name"
	QuerySortDate       = "date"
	QuerySortDefault    = QuerySortDate
)

var (
	QueryParams = map[queryParam]string{
		QueryDisplay: "display",
		QueryOrder:   "order",
		QuerySort:    "sort",
	}
)

type queryParam int

func (s queryParam) String() string {
	return QueryParams[s]
}

type RequestSettings struct {
	Sort    string `json:"sort"`
	Order   bool   `json:"order"`
	Display string `json:"display"`
}

// Serializes to base64 encoded json
func (cs *RequestSettings) Marshal() (string, error) {
	val, _ := json.Marshal(cs)
	return base64.StdEncoding.EncodeToString(val), nil
}

// Deserializes from base64 encoded json string
func (cs *RequestSettings) Unmarshal(from string) (err error) {
	s, err := base64.StdEncoding.DecodeString(from)
	if err != nil {
		return
	}
	err = json.Unmarshal(s, &cs)
	return
}

func (cs *RequestSettings) QueryString() string {
	qs := []string{}
	if cs.Display != QueryDisplayDefault {
		val := fmt.Sprintf("%s/%s", QueryParams[QueryDisplay], cs.Display)
		qs = append(qs, val)
	}
	if !cs.Order && QueryOrderDesc == QueryOrderDefault {
		val := fmt.Sprintf("%s/%s", QueryParams[QueryOrder], QueryOrderAsc)
		qs = append(qs, val)
	}
	if cs.Sort != QuerySortDefault {
		val := fmt.Sprintf("%s/%s", QueryParams[QuerySort], cs.Sort)
		qs = append(qs, val)
	}
	result, _ := url.PathUnescape(strings.Join(qs, "/"))
	if result == "" {
		return result
	}
	return "?" + result
}

func NewRequestSettings() RequestSettings {
	return RequestSettings{Sort: QuerySortDefault, Order: false, Display: QueryDisplayDefault}
}

func RequestSettingsFromQuery(q url.Values) RequestSettings {
	opts := NewRequestSettings()
	reqOrder := q.Get(QueryParams[QueryOrder])
	if reqOrder != "" {
		// true - desc, false - asc
		opts.Order = reqOrder == QueryOrderDesc
	}
	if reqSort := q.Get(QueryParams[QuerySort]); reqSort != "" {
		opts.Sort = reqSort
		if reqOrder == "" {
			// Set default order for date sorting to descending otherwise - asc
			opts.Order = reqSort == QuerySortDate
		}
	}
	if reqDisplay := q.Get(QueryParams[QueryDisplay]); reqDisplay != "" {
		opts.Display = reqDisplay
	}
	return opts
}
