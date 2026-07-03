package options

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type LiteralStringer string

func (l LiteralStringer) String() string {
	return string(l)
}

// Select prompts the user which
// of the available options is the desired
// through STDIN and returns the selected one
func Select(msg string, opts []fmt.Stringer) (interface{}, error) {
	return SelectWithDefault(msg, opts, -1)
}

// SelectWithDefault behaves like Select but, when defaultIdx is a valid index
// (>= 0), marks that option as the default and returns it when the user submits
// an empty line (just presses Enter). A negative defaultIdx means no default,
// in which case empty input is rejected just like Select historically did.
func SelectWithDefault(msg string, opts []fmt.Stringer, defaultIdx int) (interface{}, error) {
	if len(opts) == 1 {
		return opts[0], nil
	}
	fmt.Printf("\n%s\n", msg)
	for i, o := range opts {
		if i == defaultIdx {
			fmt.Printf("\n [%d] %s (default)", i+1, o)
		} else {
			fmt.Printf("\n [%d] %s", i+1, o)
		}
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		if defaultIdx >= 0 {
			fmt.Printf("\n Select an option [%d]: ", defaultIdx+1)
		} else {
			fmt.Printf("\n Select an option: ")
		}
		line, err := reader.ReadString('\n')
		input := strings.TrimSpace(line)

		if input == "" {
			if defaultIdx >= 0 {
				return opts[defaultIdx], nil
			}
			if err == io.EOF {
				return nil, err
			}
			fmt.Printf("Invalid option")
			continue
		}

		opt, convErr := strconv.Atoi(input)
		if convErr != nil || opt < 1 || opt > len(opts) {
			fmt.Printf("Invalid option")
			continue
		}

		return opts[opt-1], nil
	}
}

// SelectCustom prompts the user which
// of the available options is the desired
// through STDIN and returns the selected or a custom one
func SelectCustom(msg string, opts []fmt.Stringer) (interface{}, error) {
	if len(opts) == 1 {
		return opts[0], nil
	}
	fmt.Printf("\n%s\n", msg)
	for i, o := range opts {
		fmt.Printf("\n [%d] %s", i+1, o)
	}

	var opt string
	var v int
	var err error
	for {
		fmt.Printf("\n Select an option or type a custom value: ")
		_, err = fmt.Scanln(&opt)

		if err != nil {
			return nil, err
		}

		v, err = strconv.Atoi(opt)
		if err != nil {
			return LiteralStringer(opt), nil
		}

		if err != nil || v < 1 || v > len(opts) {
			if err != nil {
				if err == io.EOF {
					return nil, err
				}
			}
			fmt.Printf("Invalid option")
			continue
		}
		break

	}

	return opts[v-1], nil
}
