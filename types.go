package main

type BuildOverview struct {
	Timestamp int64 `json:"timestamp"`
	Duration  int64 `json:"duration"`
}

type BuildEntry struct {
	Number   int    `json:"number"`
	URL      string `json:"url"`
	Overview *BuildOverview
}

type HealthReport struct {
	Score       int    `json:"score"`
	Description string `json:"description"`
}

type View struct {
	Name   string `json:"name"`
	Color  string `json:"color"`
	URL    string `json:"url"`
	Detail *ViewDetails
}

type ViewDetails struct {
	Description string `json:"description"`
	Jobs        []Job  `json:"jobs"`
	Name        string `json:"name"`
}

type JobOverview struct {
	Name                string         `json:"name"`
	URL                 string         `json:"url"`
	Color               string         `json:"color"`
	LastBuild           BuildEntry     `json:"lastBuild"`
	LastSuccessfulBuild BuildEntry     `json:"lastSuccessfulBuild"`
	HealthReport        []HealthReport `json:"healthReport"`
}

type Job struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Color string `json:"color"`

	Overview *JobOverview
}

type JenkinsResponse struct {
	Jobs  []Job  `json:"jobs"`
	Views []View `json:"views"`
}
