package buildinfo

import "fmt"

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func String() string {
	return fmt.Sprintf("modelcairn %s (commit=%s date=%s)", Version, Commit, Date)
}
