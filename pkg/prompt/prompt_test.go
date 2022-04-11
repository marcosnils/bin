package prompt

import (
	"bytes"
	"testing"
)

func TestConfirm(t *testing.T) {
	t.Run("User confirms that wants to continue", func(t *testing.T) {
		answers := [][]byte{
			[]byte("Y\n"),
			[]byte("y\n"),
			[]byte("\n"),
		}

		for _, answer := range answers {
			stdin = bytes.NewReader(answer)
			err := Confirm("Do you want to continue?")
			if err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("User does not want to continue", func(t *testing.T) {
		answer := []byte("n\n")
		stdin = bytes.NewReader(answer)
		err := Confirm("Do you want to continue?")
		if err == nil {
			t.Fail()
		}
	})
}
