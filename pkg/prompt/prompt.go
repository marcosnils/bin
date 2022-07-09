package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

var stdin io.Reader = os.Stdin

// Confirm prints a confirmation prompt
// for the given message and waits for the
// users input.
func Confirm(message string) error {
	fmt.Printf("\n%s [Y/n] ", message)
	reader := bufio.NewReader(stdin)
	var response string

	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("invalid input")
	}

	switch strings.ToLower(strings.TrimSpace(response)) {
	case "", "y", "yes":
	default:
		return fmt.Errorf("command aborted")
	}

	return nil
}
