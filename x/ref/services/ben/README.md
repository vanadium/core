# Ben: μBenchmark archival

- [Description](https://docs.google.com/document/d/1v-iKwej3eYT_RNhPwQ81A9fa8H15Q6RzNyv2rrAeAUc/edit?usp=sharing)

## TODOs
- Do not assume that the code is under a [jiri](https://godoc.org/v.io/jiri) project
  (i.e., detect ben.SourceCode based on the git repository state?)
- Page to list out all benchmark results (across all scenarios) for a particular (BenchmarkName, SourceCode) pair
  (e.g., results for foo/bar.BenchmarkBaz across linux, darwin, arm, amd64 for a particular snapshot of the code)
- Render times in local timezone (and convert timezones in the query to the server)
- Ability to query results by time range
