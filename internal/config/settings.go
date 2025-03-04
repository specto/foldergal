package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type queryParam int

const (
	QKeyDisplay queryParam = iota
	QKeyOrder
	QKeySort
)

type (
	QTypeOrder   string
	QTypeSort    string
	QTypeDisplay string
)

var queryParams = map[queryParam]string{
	// NOTE: values must be the same as RequestSettings json names
	QKeySort:    "s",
	QKeyOrder:   "o",
	QKeyDisplay: "y",
}

func (s queryParam) String() string { return queryParams[s] }

type RequestSettings struct {
	// NOTE: json names must be the same as in queryParams
	Sort    QTypeSort    `json:"s"`
	Order   QTypeOrder   `json:"o"`
	Display QTypeDisplay `json:"y"`
}

const (
	// NOTE: values below are only defined here and can safely be changed
	//  	 as long as they remain unique
	QueryDisplayShow      QTypeDisplay = "w"
	QueryDisplayFile      QTypeDisplay = "f"
	QueryDisplayDefault   QTypeDisplay = QueryDisplayShow
	QueryOrderAsc         QTypeOrder   = "a"
	QueryOrderDesc        QTypeOrder   = "z"
	QueryOrderDateDefault QTypeOrder   = QueryOrderDesc
	QueryOrderNameDefault QTypeOrder   = QueryOrderAsc
	QuerySortName         QTypeSort    = "n"
	QuerySortDate         QTypeSort    = "d"
	QuerySortDefault      QTypeSort    = QuerySortDate
	QueryOrderDefault     QTypeOrder   = QueryOrderDateDefault
)

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

func (cs RequestSettings) WithOrder(order QTypeOrder) *RequestSettings {
	return &RequestSettings{Sort: cs.Sort, Order: order, Display: cs.Display}
}

func (cs RequestSettings) WithSort(sort QTypeSort) *RequestSettings {
	return &RequestSettings{Sort: sort, Order: cs.Order, Display: cs.Display}
}

// QueryString composes a string with parameters needed for sorting and display
// settings for use in URIs. It only puts those that are not needed by default.
func (cs *RequestSettings) QueryString() string {
	qs := []string{}
	if cs.Display != QueryDisplayDefault {
		val := fmt.Sprintf("%s/%s", QKeyDisplay, cs.Display)
		qs = append(qs, val)
	}
	var orderString QTypeOrder
	if cs.Sort == QuerySortDate && cs.Order == QueryOrderAsc {
		orderString = QueryOrderAsc
	}
	if cs.Sort == QuerySortName && cs.Order == QueryOrderDesc {
		orderString = QueryOrderDesc
	}
	if orderString != "" {
		qs = append(qs, fmt.Sprintf("%s/%s", QKeyOrder, orderString))
	}
	if cs.Sort != QuerySortDefault {
		qs = append(qs, fmt.Sprintf("%s/%s", QKeySort, cs.Sort))
	}
	result, _ := url.PathUnescape(strings.Join(qs, "/"))
	if result == "" {
		return result
	}
	return "?" + result
}

// QueryFull returns a string with all display parameters for use in URIs.
// For shortened version see: QueryString()
func (cs *RequestSettings) QueryFull() string {
	return fmt.Sprintf("?%s/%s/%s/%s/%s/%s",
		QKeyDisplay, cs.Display,
		QKeyOrder, cs.Order,
		QKeySort, cs.Sort)
}

func NewRequestSettings() RequestSettings {
	return RequestSettings{
		Sort:    QuerySortDefault,
		Order:   QueryOrderDefault,
		Display: QueryDisplayDefault}
}

func RequestSettingsFromQuery(q url.Values) RequestSettings {
	opts := NewRequestSettings()
	reqOrder := q.Get(QKeyOrder.String())
	if reqOrder != "" {
		opts.Order = QTypeOrder(reqOrder)
	}
	if reqSort := q.Get(QKeySort.String()); reqSort != "" {
		opts.Sort = QTypeSort(reqSort)
		if reqOrder == "" {
			if opts.Sort == QuerySortDate {
				opts.Order = QueryOrderDateDefault
			} else {
				opts.Order = QueryOrderNameDefault
			}
		}
	}
	if reqDisplay := q.Get(QKeyDisplay.String()); reqDisplay != "" {
		opts.Display = QTypeDisplay(reqDisplay)
	}
	return opts
}
