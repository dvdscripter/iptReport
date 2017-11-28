package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/zieckey/goini"
)

type iptResult struct {
	msg  [][]string
	name string
	err  error
}

// Resource type will hold the final json object
type Resource struct {
	Logo            string // 0
	Name            string // 1
	Link            string
	Organization    string // 2
	Type            string // 3
	Subtype         string // 4
	Events          int
	Measurements    int
	Occurrences     int       // 5
	LastModified    time.Time // 6
	LastPublication time.Time // 7
	NextPublication time.Time // 8
	Visibility      string    // 9
	Author          string    // 10
}

// IPT is our main struct to describe each IPT
type IPT struct {
	Name      string
	Resources []Resource
	Err       error
}

func (r *Resource) bind(resource []string) (err error) {
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
		if err = r.crawlResource(); err != nil {
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

func (r *Resource) crawlResource() (err error) {

	doc, err := goquery.NewDocument(r.Link)
	if err != nil {
		return
	}

	// HACK: class of div change because of js the real value of class for numbers is grey_bar
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

// substitute ', ", \
func escapeJSON(s string) string {

	reSlash := regexp.MustCompile(`\\[^"]`)
	regattr := regexp.MustCompile(`([^\\\s\[])"([^,\]])`)

	s = reSlash.ReplaceAllString(s, "\\\\")
	s = strings.Replace(s, "'", "\"", -1)
	s = strings.Replace(regattr.ReplaceAllString(s, "$1\\\"$2"), "'", "\"", -1)

	return s
}

func crawlIPT(url, alias string, result chan iptResult) {
	resp, err := http.Get(url)
	if err != nil {
		result <- iptResult{msg: nil, name: alias, err: err}
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		result <- iptResult{msg: nil, name: alias, err: err}
		return
	}
	defer resp.Body.Close()

	r := regexp.MustCompile(`(?s)var aDataSet = (\[.*?\]);`)
	if match := r.FindStringSubmatch(string(body)); match != nil {
		resources := [][]string{}
		if err := json.Unmarshal([]byte(escapeJSON(match[1])), &resources); err != nil {
			result <- iptResult{msg: nil, name: alias, err: err}
			return
		}
		result <- iptResult{msg: resources, name: alias, err: nil}
		return
	}

	result <- iptResult{msg: nil, name: alias, err: fmt.Errorf("No json found at %s", url)}

}

func main() {

	IPTs := []IPT{}
	iniFile := flag.String("file", "ipts.ini", "path to ipts.ini")

	flag.Parse()

	ini := goini.New()
	err := ini.ParseFile(*iniFile)
	if err != nil {
		log.Fatal(err)
	}

	result := make(chan iptResult)
	count := 0

	for alias, ipt := range ini.GetAll() {

		if alias == "" {
			continue
		}

		go crawlIPT(ipt["url"], alias, result)
		count++
	}

	for i := 0; i < count; i++ {
		select {
		case r := <-result:
			ipt := IPT{Name: r.name}
			if r.err != nil {
				ipt.Err = r.err
			} else {
				for _, resource := range r.msg {
					col := Resource{}
					if err := col.bind(resource); err != nil {
						log.Println(err)
					}
					ipt.Resources = append(ipt.Resources, col)
				}
			}
			IPTs = append(IPTs, ipt)
		}
	}

	titles := []string{
		"IPT",
		"Resource Name",
		"Link",
		"Logo",
		"Organization",
		"Type",
		"Subtype",
		"Events",
		"Measurements",
		"Occurrences",
		"LastModified",
		"LastPublication",
		"NextPublication",
		"Visibility",
		"Author",
		"Error",
	}

	report := csv.NewWriter(os.Stdout)

	if err := report.Write(titles); err != nil {
		log.Fatal(err)
	}
	for _, ipt := range IPTs {
		line := make([]string, len(titles))
		line[0] = ipt.Name
		if ipt.Err != nil {
			line[len(titles)-1] = ipt.Err.Error()
			if err := report.Write(line); err != nil {
				log.Fatal(err)
			}
			continue
		}
		for _, resource := range ipt.Resources {
			line[1] = resource.Name
			line[2] = resource.Link
			line[3] = resource.Logo
			line[4] = resource.Organization
			line[5] = resource.Type
			line[6] = resource.Subtype

			line[7] = strconv.Itoa(resource.Events)
			line[8] = strconv.Itoa(resource.Measurements)
			line[9] = strconv.Itoa(resource.Occurrences)

			line[10] = resource.LastModified.String()
			line[11] = resource.LastPublication.String()
			if resource.NextPublication.After(time.Date(0001, time.January, 1, 0, 0, 0, 0, time.UTC)) {
				line[12] = resource.NextPublication.String()
			}

			line[13] = resource.Author
			line[14] = resource.Visibility
			line[15] = ""
			if err := report.Write(line); err != nil {
				log.Fatal(err)
			}

		}
	}

	report.Flush()

}
