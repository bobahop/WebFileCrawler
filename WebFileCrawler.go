package main

import (
	"FileCrawler/search"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

//Page holds Title and Greeting
type Page struct {
	Title      string
	Greeting   string
	Body       string
	SearchTerm string
	FoundCount int
	StartLoc   string
}

func loadStartPage(pageName string, title string) (*Page, error) {
	greeting := "Welcome to the Web version of FileCrawler! Search the contents of files for your search term or regular expression..."
	return &Page{Title: title, Greeting: greeting}, nil
}

func loadSearchPage(pageName string, title string, term string, body string, foundcount int, startloc string) (*Page, error) {
	greeting := "The results are in!"
	return &Page{Title: title, Greeting: greeting, SearchTerm: term, Body: body,
		FoundCount: foundcount, StartLoc: startloc}, nil
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page, templates *template.Template) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func startHandler(w http.ResponseWriter, r *http.Request, templates *template.Template) {
	p, err := loadStartPage("start", "WebFileCrawler")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	renderTemplate(w, "start", p, templates)
}

func searchHandler(w http.ResponseWriter, r *http.Request, templates *template.Template) {
	foundfiles := ""
	foundcount := 0
	var searchterm = "UNKNOWN"
	mytitle := "Search Results"
	var searchfunc func(string, os.FileInfo, error) error
	var echoback []string
	foundlimit := 250
	var err error
	var reg *regexp.Regexp
	//default is case-insensitive
	caseterm := "(?i)"
	var caseflag string
	var root string
	logname := "echoback"
	var extensions []string
	var regexpterm string
	var maxval string
	if err = r.ParseForm(); err != nil {
		foundfiles = err.Error()
		mytitle = "Error parsing request form"
		goto loadpage
	}

	searchterm = r.Form.Get("myterm")
	regexpterm = r.Form.Get("myreg")
	root = r.Form.Get("myloc")
	maxval = r.Form.Get("mymax")
	if maxval != "" {
		if foundlimit, err = strconv.Atoi(maxval); err != nil {
			foundfiles = err.Error()
			mytitle = "Error parsing max files found limit"
			goto loadpage
		}
	}

	extensions = strings.Split(r.Form.Get("mytypes"), ",")
	if len(extensions) == 0 || (len(extensions) == 1 && extensions[0] == "") {
		extensions = []string{"txt"}
	}

	if regexpterm != "" {
		if reg, err = regexp.Compile(regexpterm); err != nil {
			foundfiles = err.Error()
			mytitle = "Error compiling search term"
			goto loadpage
		}
		searchterm = regexpterm
	} else {
		caseflag = r.Form.Get("mycase")
		if caseflag == "on" {
			caseterm = ""
		}
		if reg, err = regexp.Compile(caseterm + searchterm); err != nil {
			foundfiles = err.Error()
			mytitle = "Error compiling search term"
			goto loadpage
		}
	}

	echoback = make([]string, foundlimit, foundlimit)
	for i := range echoback {
		echoback[i] = "!"
	}

	if searchfunc, err = search.Factory(extensions, reg, logname, foundlimit,
		&foundcount, echoback); err != nil {
		foundfiles = err.Error()
		mytitle = "Error creating search function"
		goto loadpage
	}

	if err = filepath.Walk(root, searchfunc); err != nil {
		foundfiles = err.Error()
		mytitle = "Error walking filepath"
		goto loadpage
	}

	for _, foundfile := range echoback {
		if foundfile == "!" {
			break
		}
		foundfiles += fmt.Sprintf("%s%s", foundfile, "\n")
	}

loadpage:
	var p *Page
	if p, err = loadSearchPage("search", mytitle, searchterm, foundfiles, foundcount, root); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	renderTemplate(w, "search", p, templates)
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, *template.Template), validPath *regexp.Regexp,
	templates *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, templates)
	}
}

func main() {
	portptr := flag.String("port", "", "The port for localhost to listen on. Example: -post=8099")
	flag.Parse()
	port := *portptr
	if port == "" {
		fmt.Println("Port is not set. Example: WebFileCrawler -port=8099")
		os.Exit(1)
	}
	validPath := regexp.MustCompile("(/)|(/search)")
	templates := template.Must(template.ParseFiles("start.html", "search.html"))
	http.HandleFunc("/", makeHandler(startHandler, validPath, templates))
	http.HandleFunc("/search", makeHandler(searchHandler, validPath, templates))
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", fmt.Sprintf("http://localhost:%v/", port))
	if err := cmd.Start(); err != nil {
		fmt.Println(fmt.Sprintf("Problem launching default browser: %v", err.Error()))
		os.Exit(1)
	}
	http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
}
