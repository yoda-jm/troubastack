package app

import "time"

// MaybeBakeForTest drives the AutoBaker's debounce tick from an external test with a
// controlled clock (the production loop is AutoBaker.Run on a ticker).
func (a *AutoBaker) MaybeBakeForTest(now time.Time) { a.maybeBake(now) }
