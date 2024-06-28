package main

import (
	"encoding/json"
	"fmt"
	"os"
	"unicode"
)

func decode(b string, st int) (x interface{}, i int, e error) {

	switch {
	case unicode.IsDigit(rune(b[st])):
		return decodeString(b, st)
	case b[st] == 'i':
		return decodeInt(b, st)
	case b[st] == 'l':
		return decodeList(b, st)

	default:
		return nil, st, fmt.Errorf("unexpected value: %q", b[i])
	}
}

func decodeInt(b string, st int) (xint, i int, err error) {
	i = st + 1 // move 'i'
	l := 0

	neg := false

	if b[i] == '-' {
		neg = true
		i++
	}

	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		l = l*10 + (int(b[i]) - '0')
		i++
	}

	if i == len(b) || b[i] != 'e' {
		return 0, st, fmt.Errorf("invalid interger format")
	}

	i++

	if err != nil {
		return 0, st, fmt.Errorf("error parsing number")
	}

	if neg {
		l = -l
	}

	return l, i, nil
}

func decodeString(b string, st int) (x string, i int, err error) {
	i = st

	l := 0

	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		l = l*10 + (int(b[i]) - '0')
		i++
	}

	if b[i] != ':' {
		return "", st, fmt.Errorf("Bad string")
	}

	i++

	if i+l > len(b) {
		return "", st, fmt.Errorf("bad string: out of bounds")
	}

	x = b[i : i+l]

	i += l

	return x, i, nil
}

func decodeList(b string, st int) (l []interface{}, i int, err error) {

	i = st
	i++ // Move 'l'

	l = make([]interface{}, 0)

	for {
		if i > len(b) {
			return nil, st, fmt.Errorf("bad list")
		}

		if b[i] == 'e' { // End of list
			break
		}

		var x interface{}

		x, i, err = decode(b, i)

		if err != nil {
			return nil, i, err
		}

		l = append(l, x)

	}

	return l, i + 1, nil
}

func main() {

	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		decoded, _, err := decode(bencodedValue, 0)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
