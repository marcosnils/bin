package prompt

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestConfirm(t *testing.T) {
	writeToPrompt := func(answer []byte) error {
		tmpfile, err := ioutil.TempFile("", "tmp")
		if err != nil {
			return err
		}

		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write(answer)
		if err != nil {
			return err
		}

		_, err = tmpfile.Seek(0, 0)
		if err != nil {
			return err
		}

		stdin := os.Stdin
		defer func() {
			os.Stdin = stdin
		}()

		os.Stdin = tmpfile
		err = Confirm("Do you want to continue?")
		if err != nil {
			return err
		}

		if err := tmpfile.Close(); err != nil {
			return err
		}
		return nil
	}

	t.Run("User confirms that wants to continue", func(t *testing.T) {
		answers := [][]byte{
			[]byte("Y\n"),
			[]byte("y\n"),
			[]byte("\n"),
		}

		for _, answer := range answers {
			err := writeToPrompt(answer)
			if err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("User does not want to continue", func(t *testing.T) {
		answer := []byte("n\n")
		err := writeToPrompt(answer)
		if err == nil {
			t.Fail()
		}
	})
}
