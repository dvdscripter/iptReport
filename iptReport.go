// Package iptReport provides means for large countries to monitor biodiversity
// progress at many IPTs. At cmd/report2csv you will find a example generate a
// csv report.
package iptReport

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// IPTResult is a placeholder struct to receive data coming from crawlIPT
// coroutines.
type IPTResult struct {
	Msg  [][]string
	Name string
	Err  error
}

// Resource type will hold the final json object.
type Resource struct {
	Logo            string
	Name            string
	Link            string
	Organization    string
	Type            string
	Subtype         string
	Events          int
	Measurements    int
	Occurrences     int
	LastModified    time.Time
	LastPublication time.Time
	NextPublication time.Time
	Visibility      string
	Author          string
}

// IPT is our main struct to describe each IPT.
type IPT struct {
	Name      string
	Resources []Resource
	Err       error
}

// Bind bind all unmarshal elements to Resource.
func (r *Resource) Bind(resource []string) (err error) {
	regLogo := regexp.MustCompile(`src\s*=\s*"([^"]+)`)
	if match := regLogo.FindStringSubmatch(resource[0]); match != nil {
		r.Logo = match[1]
	}

	regName := regexp.MustCompile(`<if>([^<]+)`)
	if match := regName.FindStringSubmatch(resource[1]); match != nil {
		r.Name = match[1]
	}

	regLink := regexp.MustCompile(`href\s*=\s*"([^"]+)`)
	if match := regLink.FindStringSubmatch(resource[1]); match != nil {
		r.Link = match[1]
	}

	r.Organization = resource[2]
	r.Type = resource[3]
	r.Subtype = resource[4]

	regRecords := regexp.MustCompile(`.+?>([^<]+).+`)
	if match := regRecords.FindStringSubmatch(resource[5]); match != nil {
		r.Occurrences, err = strconv.Atoi(strings.Replace(match[1], ",", "", -1))
		if err != nil {
			return err
		}
		if err = r.CrawlResource(); err != nil {
			return err
		}

	} else {
		r.Occurrences, err = strconv.Atoi(strings.Replace(resource[5], ",", "", -1))
		if err != nil {
			return err
		}
	}

	if resource[6] != "--" {
		r.LastModified, err = time.Parse("2006-01-02", resource[6])
		if err != nil {
			return err
		}
	}
	if resource[7] != "--" {
		r.LastPublication, err = time.Parse("2006-01-02", resource[7])
		if err != nil {
			return err
		}
	}
	if resource[8] != "--" {
		r.NextPublication, err = time.Parse("2006-01-02 15:04:05", resource[8])
		if err != nil {
			return err
		}
	}

	r.Author = resource[9]
	r.Visibility = resource[10]

	return nil
}

// CrawlResource seek information about number of occurreces, events and
// measurements to fill Resource.occurreces, Resource.Events and
// Resource.Measurements.
func (r *Resource) CrawlResource() (err error) {

	doc, err := goquery.NewDocument(r.Link)
	if err != nil {
		return
	}

	// HACK: class of div change because of js the real value of class for
	// numbers is grey_bar
	// FIXME:  we're ignoring error from number conversion!!!
	doc.Find(".no_bullets > li").Each(func(i int, s *goquery.Selection) {
		section := s.Find("span").Text()
		switch {
		case strings.HasPrefix(section, "Event"):
			value, err := strconv.Atoi(s.Find(".grey_bar").Text())
			if err == nil {
				r.Events = value
			}
		case strings.HasPrefix(section, "MeasurementOrFact"):
			value, err := strconv.Atoi(s.Find(".grey_bar").Text())
			if err == nil {
				r.Measurements = value
			}
		case strings.HasPrefix(section, "Occurrence"):
			value, err := strconv.Atoi(s.Find(".grey_bar").Text())
			if err == nil {
				r.Occurrences = value
			}

		}

	})

	return
}

// EscapeJSON escape occurrences of  [', ", \] into valid json.
func EscapeJSON(s string) string {

	reSlash := regexp.MustCompile(`\\[^"]`)
	regattr := regexp.MustCompile(`([^\\\s\[])"([^,\]])`)

	s = reSlash.ReplaceAllString(s, "\\\\")
	s = strings.Replace(s, "'", "\"", -1)
	s = strings.Replace(regattr.ReplaceAllString(s, "$1\\\"$2"), "'", "\"", -1)

	return s
}

// CrawlIPT crawl ipt at url with identifying alias storing in IPTResult.
func CrawlIPT(url, alias string, result chan IPTResult) {
	resp, err := http.Get(url)
	if err != nil {
		result <- IPTResult{Msg: nil, Name: alias, Err: err}
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		result <- IPTResult{Msg: nil, Name: alias, Err: err}
		return
	}
	defer resp.Body.Close()

	r := regexp.MustCompile(`(?s)var aDataSet = (\[.*?\]);`)
	if match := r.FindStringSubmatch(string(body)); match != nil {
		resources := [][]string{}
		if err := json.Unmarshal([]byte(EscapeJSON(match[1])), &resources); err != nil {
			result <- IPTResult{Msg: nil, Name: alias, Err: err}
			return
		}
		result <- IPTResult{Msg: resources, Name: alias, Err: nil}
		return
	}

	result <- IPTResult{Msg: nil, Name: alias, Err: fmt.Errorf("No json found at %s", url)}

}
