package hostname

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ExpandHostnamePattern replaces {Nd} with zero-padded NID and {id} with xname
// Example patterns:
//   - "nid{04d}" with NID=1 => "nid0001"
//   - "dev-s{02d}" with NID=5 => "dev-s05"
//   - "bmc{03d}" with NID=42 => "bmc042"
//   - "{id}" with xname="x3000c0s0b1" => "x3000c0s0b1"
func ExpandHostnamePattern(pattern string, nid int64, id string) string {
	out := strings.ReplaceAll(pattern, "{id}", id)
	re := regexp.MustCompile(`\{0*(\d+)d\}`)
	out = re.ReplaceAllStringFunc(out, func(m string) string {
		nStr := re.FindStringSubmatch(m)[1]
		n, _ := strconv.Atoi(nStr)
		return fmt.Sprintf("%0*d", n, nid)
	})
	return out
}
