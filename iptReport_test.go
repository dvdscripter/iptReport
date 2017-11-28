package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestBind(t *testing.T) {
	tableCases := []struct {
		input       []string
		output      Resource
		shouldError bool
	}{
		{
			[]string{
				"<img class=\"resourceminilogo\" src=\"http://ipt.sibbr.gov.br/sibbr/logo.do?r=bamba_fish_seamount\" />",
				"<a href=\"https://ipt.sibbr.gov.br/repatriados/resource?r=repatriados\"><if>Repatriation Data for SiBBr</a>",
				"Not registered",
				"Occurrence",
				"--",
				"3,537,502",
				"2017-08-07",
				"2017-08-07",
				"2018-08-04 11:45:15",
				"--",
				"Public",
			},
			Resource{
				Logo:            "http://ipt.sibbr.gov.br/sibbr/logo.do?r=bamba_fish_seamount",
				Name:            "Repatriation Data for SiBBr",
				Link:            "https://ipt.sibbr.gov.br/repatriados/resource?r=repatriados",
				Organization:    "Not registered",
				Type:            "Occurrence",
				Subtype:         "--",
				Occurrences:     3537502,
				LastModified:    time.Date(2017, time.August, 7, 0, 0, 0, 0, time.UTC),
				LastPublication: time.Date(2017, time.August, 7, 0, 0, 0, 0, time.UTC),
				NextPublication: time.Date(2018, time.August, 4, 11, 45, 15, 0, time.UTC),
				Author:          "--",
				Visibility:      "Public",
			},
			false,
		},
		{
			[]string{
				"<img class=\"resourceminilogo\" src=\"http://ipt.sibbr.gov.br/sibbr/logo.do?r=bamba_fish_seamount\" />",
				"<a href=\"https://ipt.sibbr.gov.br/repatriados/resource?r=repatriados\"><if>Repatriation Data for SiBBr</a>",
				"Not registered",
				"Occurrence",
				"--",
				"3,537,502 ERROR INTRODUCED HERE",
				"2017-08-07",
				"2017-08-07",
				"2018-08-04 11:45:15",
				"--",
				"Public",
			},
			Resource{},
			true,
		},
		{
			[]string{
				"<img class=\"resourceminilogo\" src=\"http://ipt.sibbr.gov.br/sibbr/logo.do?r=bamba_fish_seamount\" />",
				"<a href=\"https://ipt.sibbr.gov.br/repatriados/resource?r=repatriados\"><if>Repatriation Data for SiBBr</a>",
				"Not registered",
				"Occurrence",
				"--",
				"3,537,502",
				"2017-08-07 ERROR INTRODUCED HERE",
				"2017-08-07",
				"2018-08-04 11:45:15",
				"--",
				"Public",
			},
			Resource{},
			true,
		},
		{
			[]string{
				"<img class=\"resourceminilogo\" src=\"http://ipt.sibbr.gov.br/sibbr/logo.do?r=bamba_fish_seamount\" />",
				"<a href=\"https://ipt.sibbr.gov.br/repatriados/resource?r=repatriados\"><if>Repatriation Data for SiBBr</a>",
				"Not registered",
				"Occurrence",
				"--",
				"3,537,502",
				"2017-08-07",
				"2017-08-07 ERROR INTRODUCED HERE",
				"2018-08-04 11:45:15",
				"--",
				"Public",
			},
			Resource{},
			true,
		},
		{
			[]string{
				"<img class=\"resourceminilogo\" src=\"http://ipt.sibbr.gov.br/sibbr/logo.do?r=bamba_fish_seamount\" />",
				"<a href=\"https://ipt.sibbr.gov.br/repatriados/resource?r=repatriados\"><if>Repatriation Data for SiBBr</a>",
				"Not registered",
				"Occurrence",
				"--",
				"3,537,502",
				"2017-08-07",
				"2017-08-07",
				"2018-08-04 11:45:15 ERROR INTRODUCED HERE",
				"--",
				"Public",
			},
			Resource{},
			true,
		},
	}

	for _, tt := range tableCases {

		r := Resource{}
		// fatal only if we didn't expect error
		if err := r.bind(tt.input); err != nil && tt.shouldError == false {
			t.Fatal(err)
		} else if r != tt.output && tt.shouldError == false {
			// only check expected output if no error occur
			t.Errorf("got \n%#v, want \n%#v", r, tt.output)
		}

	}
}

func TestCrawIPT(t *testing.T) {
	// mocking success server on loopback
	index := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, _ := os.Open("testdata/home/index.html")
		defer file.Close()
		io.Copy(w, file)
	}))
	defer index.Close()
	// fail to crawl valid page with json struct
	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Should error"))
	}))
	defer fail.Close()
	failJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, _ := os.Open("testdata/home/fail.html")
		defer file.Close()
		io.Copy(w, file)
	}))
	defer failJSON.Close()

	tableCases := []struct {
		input       string
		output      iptResult
		shouldError bool
	}{
		{
			index.URL,
			iptResult{[][]string{
				{
					"--",
					"<a href=\"https://ipt.sibbr.gov.br/repatriados/resource?r=repatriados\"><if>Repatriation Data for SiBBr</a>",
					"Not registered",
					"Occurrence",
					"--",
					"3,537,502",
					"2017-08-07",
					"2017-08-07",
					"--",
					"Public",
					"--",
				},
			},
				"goeldi",
				nil},
			false},
		{
			"THIS URL SHOULDN'T EXIST",
			iptResult{},
			true},
		{
			fail.URL,
			iptResult{},
			true},
		{
			failJSON.URL,
			iptResult{},
			true},
	}

	result := make(chan iptResult)

	for _, tt := range tableCases {
		go crawlIPT(tt.input, "goeldi", result)
		r := <-result
		if r.err != nil && tt.shouldError == false {
			t.Fatal(r.err)
		} else if !reflect.DeepEqual(tt.output, r) && tt.shouldError == false {
			t.Errorf("got \n%#v, want \n%#v", r, tt.output)
		}

	}

}

func TestCrawlResource(t *testing.T) {

	success := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, _ := os.Open("testdata/resource/success.html")
		defer file.Close()
		io.Copy(w, file)
	}))
	defer success.Close()

	tableCases := []struct {
		input       Resource
		output      Resource
		shouldError bool
	}{
		{
			Resource{
				Link: success.URL,
			},
			Resource{
				Link:         success.URL,
				Events:       474,
				Measurements: 4019,
				Occurrences:  4019,
			},
			false,
		},
		{
			Resource{
				Link: "THIS URL SHOULDN'T EXIST",
			},
			Resource{},
			true,
		},
	}

	for _, tt := range tableCases {
		input := tt.input
		if err := input.crawlResource(); err != nil && tt.shouldError == false {
			t.Fatal(err)
		} else if !reflect.DeepEqual(input, tt.output) && tt.shouldError == false {
			t.Errorf("got \n%#v, want \n%#v", input, tt.output)
		}

	}
}

func TestEscapeJSON(t *testing.T) {

	tableCases := []struct {
		input  string
		output string
	}{
		{
			`[
["--',
"<a href='https://ipt.sibbr.gov.br/repatriados/resource?r=repatriados'><if>Repatriation Data for SiBBr</a>",
'Not registered',
'Occurrence',
'--',
'3,537,502',
'2017-08-07',
'2017-08-07',
'--',
'Public',
'--']
]`,
			`[
["--",
"<a href=\"https://ipt.sibbr.gov.br/repatriados/resource?r=repatriados\"><if>Repatriation Data for SiBBr</a>",
"Not registered",
"Occurrence",
"--",
"3,537,502",
"2017-08-07",
"2017-08-07",
"--",
"Public",
"--"]
]`,
		},
	}

	for _, tt := range tableCases {
		if r := escapeJSON(tt.input); r != tt.output {
			t.Errorf("got \n%s, want \n%s", r, tt.output)
		}
	}

}
