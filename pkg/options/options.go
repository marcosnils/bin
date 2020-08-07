package options

import (
	"fmt"
	"io"
)

type LiteralStringer string

func (l LiteralStringer) String() string {
	return string(l)
}

//Select prompts the user which
//of the available options is the desired
//through STDIN and returns the selected one
func Select(msg string, opts []fmt.Stringer) (interface{}, error) {
	if len(opts) == 1 {
		return opts[0], nil
	}
	fmt.Printf("\n%s\n", msg)
	for i, o := range opts {
		fmt.Printf("\n [%d] %s", i+1, o)
	}

	var opt uint
	var err error
	for {
		fmt.Printf("\n Select an option: ")
		_, err = fmt.Scanln(&opt)
		if err != nil || opt < 1 || int(opt) > len(opts) {
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

	return opts[opt-1], nil
}
