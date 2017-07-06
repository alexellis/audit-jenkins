audit-jenkins
===============

Audits and generates a report on the health and staleness of Jenkins jobs

### Running an audit:

Example output:

```
$ audit-jenkins -url http://jenkins:8080/
```

* No view specified
* Jobs never run
* Jobs never passed
* Stale jobs (not run in 7 days)

### Save / back-up jobs to disk

```
$ audit-jenkins -url http://jenkins:8080/ -saveJobs=true
```

### Requirements

> Requires Golang to build - `go get` / `go install`
