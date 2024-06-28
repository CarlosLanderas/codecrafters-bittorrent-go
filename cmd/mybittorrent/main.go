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
	case b[st] == 'd':
		return decodeDictionary(b, st)

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

func decodeDictionary(b string, st int) (d map[string]interface{}, i int, err error) {
	i = st
	i++ // move dict

	d = make(map[string]interface{})

	for {

		if b[i] == 'e' {
			break
		}

		var key, val interface{}

		key, i, err = decode(b, i)

		if err != nil {
			return nil, i, err
		}

		k, ok := key.(string)

		if !ok {
			return nil, i, fmt.Errorf("dict key is not a string: %q", key)
		}

		val, i, err = decode(b, i)

		if err != nil {
			return nil, i, err
		}

		d[k] = val

	}

	return d, i, nil
}

func parseTorrent(torrent string) (string, error) {

	data, _, err := decodeDictionary(torrent, 0)
	if err != nil {
		return "", err
	}

	tracker := data["announce"]

	info, ok := data["info"].(map[string]interface{})

	if !ok {
		return "", fmt.Errorf("could not parse info section")
	}

	return fmt.Sprintf("Tracker URL: %s\nLength: %v", tracker, info["length"]), nil
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

	} else if command == "info" {
		torrentFile := os.Args[2]
		b, err := os.ReadFile(torrentFile)

		if err != nil {
			fmt.Println("File not found: ", torrentFile)
			os.Exit(1)
		}

		info, err := parseTorrent(string(b))

		if err != nil {
			fmt.Println("Error parsing torrent: ", err.Error())
		}

		fmt.Println(info)

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}

}
