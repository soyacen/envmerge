package cli

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/soyacen/envmerge/pkg/envmerge"
)

// sensitiveMarkers identify variables whose values must never be printed.
var sensitiveMarkers = []string{"PASSWORD", "SECRET", "TOKEN", "PRIVATE", "KEY", "CERT"}

// isSensitive reports whether a variable key is considered sensitive.
func isSensitive(key string) bool {
	k := strings.ToUpper(key)
	for _, m := range sensitiveMarkers {
		if strings.Contains(k, m) {
			return true
		}
	}
	return false
}

// reportOverrides prints override diagnostics when --verbose is set. Only
// variable names and source positions are printed, never values, so sensitive
// variables are protected by construction (design §12.3).
func reportOverrides(c *cli.Command, res *envmerge.Result) {
	if !verbose(c) || len(res.Overrides) == 0 {
		return
	}
	for _, o := range res.Overrides {
		fmt.Fprintf(stderr, "[WARN] Variable %s overridden\n", o.Key)
		fmt.Fprintf(stderr, "       From: %s\n", o.Previous.String())
		fmt.Fprintf(stderr, "       To:   %s\n", o.Current.String())
	}
}
