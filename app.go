package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var client *http.Client

type jobFetch struct {
	Job      Job
	JobIndex int
}

type byTimestamp []Job

func (s byTimestamp) Len() int {
	return len(s)
}

func (s byTimestamp) Swap(x, y int) {
	xObj := s[x]
	yObj := s[y]
	s[x] = yObj
	s[y] = xObj
}

func (s byTimestamp) Less(x, y int) bool {
	var xVal int64
	var yVal int64

	if s[x].Overview != nil && s[x].Overview.LastBuild.Overview != nil {
		xVal = s[x].Overview.LastBuild.Overview.Timestamp
	}
	if s[y].Overview != nil && s[y].Overview.LastBuild.Overview != nil {
		yVal = s[y].Overview.LastBuild.Overview.Timestamp
	}

	return xVal < yVal
}

func getStaleJobs(response *JenkinsResponse) []string {
	target := []string{}

	sort.Sort(byTimestamp(response.Jobs))

	for _, job := range response.Jobs {

		if job.Overview.LastBuild.Number > 0 &&
			job.Overview.LastBuild.Overview.Timestamp > 0 {

			stamp := time.Unix(job.Overview.LastBuild.Overview.Timestamp/1000, 0)

			// One week ago
			past := time.Now().Add((-24 * 7) * time.Hour)

			if stamp.Before(past) {
				duration := time.Now().Sub(stamp)
				target = append(target, job.Name+" "+strconv.Itoa(int(duration.Hours()/24))+" days ago")
			}
		}
	}
	return target
}

func getNeverPassed(response *JenkinsResponse) []string {
	target := []string{}

	for _, job := range response.Jobs {
		if job.Overview.LastBuild.Number > 0 && job.Overview.LastSuccessfulBuild.Number == 0 {
			target = append(target, job.Name)
		}
	}
	return target
}

func getNeverRun(response *JenkinsResponse) []string {
	target := []string{}

	for _, job := range response.Jobs {

		if job.Overview.LastBuild.Number == 0 && job.Overview.LastSuccessfulBuild.Number == 0 {
			target = append(target, job.Name)
		}
	}
	return target
}

func showJobsOutsideViews(response *JenkinsResponse) []string {
	found := map[string]bool{}

	for _, view := range response.Views {

		if view.Name != "All" {
			for _, job := range view.Detail.Jobs {
				if found[job.Name] == false {
					found[job.Name] = true
				}
			}
		}
	}
	unallocated := []string{}
	for _, job := range response.Jobs {
		if found[job.Name] == false {
			unallocated = append(unallocated, job.Name)
		}
	}
	return unallocated
}

func getJobs(url string) (*JenkinsResponse, error) {
	var err error

	val, err := get(url + "api/json")
	res := JenkinsResponse{}

	if err == nil {
		err := json.Unmarshal(val, &res)
		if err != nil {
			return nil, err
		}

		var wg sync.WaitGroup
		workers := 4
		workQueueSize := workers * 3

		workItems := make(chan jobFetch, workQueueSize)

		for i := 0; i < workers; i++ {
			wg.Add(1)

			go func() {
				for work := range workItems {
					log.Println("-")

					jobOverview, fetchErr := fetchJob(work.Job)
					if fetchErr != nil {
						fmt.Println(fetchErr)
						return
					}

					res.Jobs[work.JobIndex].Overview = jobOverview

					if jobOverview.LastBuild.Number > 0 {
						build, fetchBuildErr := fetchBuild(jobOverview.LastBuild.Number, work.Job)
						if fetchBuildErr != nil {
							fmt.Println(fetchErr)
							return
						}
						jobOverview.LastBuild.Overview = build
					}
					log.Println("]")
				}

				wg.Done()
			}()
		}

		for i, job := range res.Jobs {
			workItems <- jobFetch{
				Job:      job,
				JobIndex: i,
			}
			log.Println("[")
		}
		close(workItems)

		wg.Wait()

		for i, view := range res.Views {

			viewDetail, viewErr := getView(view.URL)
			if viewErr != nil {
				return nil, viewErr
			}
			// fmt.Printf("%s - %d jobs\n", viewDetail.Name, len(viewDetail.Jobs))
			res.Views[i].Detail = viewDetail
		}
	}

	return &res, err
}

func fetchBuild(build int, job Job) (*BuildOverview, error) {
	var err error
	var jobOverview BuildOverview

	res, err := get(job.URL + strconv.Itoa(build) + "/api/json")
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(res, &jobOverview)

	return &jobOverview, err
}

func fetchJob(job Job) (*JobOverview, error) {
	var err error
	var jobOverview JobOverview

	res, err := get(job.URL + "api/json")
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(res, &jobOverview)

	return &jobOverview, err
}

func getView(viewURL string) (*ViewDetails, error) {

	var err error
	var viewDetails ViewDetails

	res, err := get(viewURL + "api/json")
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(res, &viewDetails)

	return &viewDetails, err
}

func get(address string) ([]byte, error) {
	var err error

	parsedURL, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	req := http.Request{
		Method: "GET",
		URL:    parsedURL,
	}
	log.Println("Fetch: ", parsedURL)

	response, err := client.Do(&req)
	if err != nil {
		log.Printf("%s\n", err.Error())
	}

	if response.Body != nil {
		defer response.Body.Close()
	}

	bodyBytes, err := ioutil.ReadAll(response.Body)

	return bodyBytes, err
}

// go run *.go -url http://jenkins:8080/

func main() {
	var url string
	var saveJobs bool

	flag.StringVar(&url, "url", "", "Give a Jenkins server URL")
	flag.BoolVar(&saveJobs, "saveJobs", false, "Save jobs to disk")
	flag.Parse()

	if len(url) == 0 {
		fmt.Println("Please pass the url of your Jenkins server via -url")
		return
	}
	if url[len(url)-1:] != "/" {
		fmt.Println("-url should end in trailing slash.")
		return
	}

	client = &http.Client{
		Timeout: time.Second * 5,
	}

	jobResponse, err := getJobs(url)
	if err != nil {
		fmt.Printf("Error %s\n", err.Error())
		return
	}

	if saveJobs {
		for _, view := range jobResponse.Views {
			if strings.ToLower(view.Name) == "all" {
				continue
			}

			fmt.Println("Working on view: " + view.Name)

			folder := fmt.Sprintf("./jobs/%s", view.Name)
			fmt.Println("Creating: " + folder)
			err := os.MkdirAll(folder, 0755)
			if err != nil {
				fmt.Println("Cannot create folder: " + folder)
			}

			if view.Detail != nil {
				for _, job := range view.Detail.Jobs {
					configURL := fmt.Sprintf("%sconfig.xml", job.URL)

					path := fmt.Sprintf("%s/%s.xml", folder, job.Name)
					fmt.Printf("Saving: %s\n", path)
					bytesOut, err := get(configURL)
					if err != nil {
						fmt.Println(err)
					} else {
						writeErr := ioutil.WriteFile(path, bytesOut, 0700)
						if writeErr != nil {
							fmt.Println(writeErr)
						}
					}
				}
			}
		}
	} else {
		jobsOutsideViews := showJobsOutsideViews(jobResponse)
		if len(jobsOutsideViews) > 0 {
			fmt.Println("No view specified:")
			for _, item := range jobsOutsideViews {
				fmt.Printf("- %s\n", item)
			}
			fmt.Println()
		}

		neverRun := getNeverRun(jobResponse)
		if len(neverRun) > 0 {
			fmt.Println("Jobs never run:")
			for _, item := range neverRun {
				fmt.Printf("- %s\n", item)
			}
			fmt.Println()
		}

		neverPassed := getNeverPassed(jobResponse)
		if len(neverPassed) > 0 {
			fmt.Println("Jobs never passed:")
			for _, item := range neverPassed {
				fmt.Printf("- %s\n", item)
			}
			fmt.Println()
		}

		stale := getStaleJobs(jobResponse)
		if len(stale) > 0 {
			fmt.Println("Stale jobs:")

			for _, item := range stale {
				fmt.Printf("- %s\n", item)
			}
			fmt.Println()
		}
	}

	// for _, job := range jobResponse.Jobs {
	// 	fmt.Println(job.Name)

	// 	if job.Overview != nil && len(job.Overview.HealthReport) > 0 {
	// 		fmt.Printf("Health Report Score: %d / 100\n", job.Overview.HealthReport[0].Score)
	// 	}
	// }
}
