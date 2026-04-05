package logrender

import "time"

// makeTime parses an RFC3339 string into a time.Time, panicking on failure.
// Test-only helper.
func makeTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic("makeTime: " + err.Error())
	}
	return t
}

// intPtr returns a pointer to the given int. Test-only helper.
func intPtr(n int) *int { return &n }

// feedRecords sends recs into a channel and closes it.
func feedRecords(recs []Record) <-chan Record {
	ch := make(chan Record, len(recs))
	for _, r := range recs {
		ch <- r
	}
	close(ch)
	return ch
}
