package main

import (
	"encoding/csv"
	"flag"
	"log"
	"os"
	"strconv"
	"time"

	report "github.com/dvdscripter/iptReport"
	"github.com/zieckey/goini"
)

func main() {

	IPTs := []report.IPT{}
	iniFile := flag.String("file", "ipts.ini", "path to ipts.ini")

	flag.Parse()

	ini := goini.New()
	err := ini.ParseFile(*iniFile)
	if err != nil {
		log.Fatal(err)
	}

	result := make(chan report.IPTResult)
	count := 0

	for alias, ipt := range ini.GetAll() {

		if alias == "" {
			continue
		}

		go report.CrawlIPT(ipt["url"], alias, result)
		count++
	}

	for i := 0; i < count; i++ {
		select {
		case r := <-result:
			ipt := report.IPT{Name: r.Name}
			if r.Err != nil {
				ipt.Err = r.Err
			} else {
				for _, resource := range r.Msg {
					col := report.Resource{}
					if err := col.Bind(resource); err != nil {
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
