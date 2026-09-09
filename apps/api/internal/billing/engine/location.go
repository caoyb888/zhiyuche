package engine

import "time"

// Location is the business time zone (Asia/Shanghai) used for time-of-day
// multipliers, month bounds and settlement periods.
func Location() *time.Location { return shanghai }
