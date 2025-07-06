package runtime

import "fmt"

// toString converts any value to string - shared helper function
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
