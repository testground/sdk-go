package run

// panicHandler is where the top-level main goroutine panic handler is
// listening for panics.
var panicHandler = make(chan interface{})

// HandlePanics should be called in a defer at the top of any goroutine that
// the test plan spawns, so that panics from children goroutine are propagated
// to the main goroutine, where they will be handled by run.Invoke and recorded
// as a CRASH event. The test will end immediately.
func HandlePanics() {
	obj := recover()
	if obj == nil {
		return
	}
	panicHandler <- obj
}
