package repl

import (
	"fmt"
	"time"
)

// Spinner runs a visual indicator while the caller performs work.
// Usage:
//
//	stop := spinner.Start("⏳ 思考中")
//	result, err := aiClient.Generate(...)
//	stop()
type Spinner struct {
	frames []string
	msg    string
	stopCh chan struct{}
	doneCh chan struct{}
}

var defaultFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// StartSpinner begins animating the spinner with the given message.
// Call the returned stop function to clear it and stop the goroutine.
func StartSpinner(msg string) func() {
	s := &Spinner{
		frames: defaultFrames,
		msg:    msg,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	go s.run()

	return func() {
		close(s.stopCh)
		<-s.doneCh
		// Clear the line
		fmt.Print("\r\033[K")
	}
}

func (s *Spinner) run() {
	defer close(s.doneCh)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			frame := s.frames[i%len(s.frames)]
			i++
			// \r returns to column 0, \033[K clears to end of line
			fmt.Printf("\r  %s %s", frame, s.msg)
		}
	}
}
