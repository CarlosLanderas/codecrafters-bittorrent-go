package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jackpal/bencode-go"
)

type TorrentFile struct {
	Announce string   `bencode:"announce"`
	Info     MetaInfo `bencode:"info"`
}

type MetaInfo struct {
	Name        string `bencode:"name"`
	Pieces      string `bencode:"pieces"`
	Length      int64  `bencode:"length"`
	PieceLength int64  `bencode:"piece length"`
}

func main() {

	command := os.Args[1]

	if command == "decode" {
		decode(os.Args[2])
	} else if command == "info" {
		info(os.Args[2])
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

func decode(bencodedValue string) {
	decoded, err := bencode.Decode(bytes.NewBuffer([]byte(bencodedValue)))
	if err != nil {
		fmt.Println(err)
		return
	}

	jsonOutput, _ := json.Marshal(decoded)
	fmt.Println(string(jsonOutput))
}

func info(torrentPath string) {
	b, err := os.ReadFile(torrentPath)

	if err != nil {
		fmt.Println("File not found: ", torrentPath)
		os.Exit(1)
	}

	if err := parseTorrent(string(b)); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

}

func parseTorrent(content string) error {

	torrent := TorrentFile{}

	err := bencode.Unmarshal(bytes.NewBuffer([]byte(content)), &torrent)
	if err != nil {
		return fmt.Errorf("error unmarshaling torrent content")
	}

	tracker := torrent.Announce

	h := sha1.New()

	err = bencode.Marshal(h, torrent.Info)

	if err != nil {
		return fmt.Errorf("error marshaling meta info")
	}

	infoHash := h.Sum(nil)

	hash := fmt.Sprintf("%x\n", infoHash)

	fmt.Println("Tracker URL:", tracker)
	fmt.Println("Length:", torrent.Info.Length)
	fmt.Println("Info Hash:", hash)

	return nil
}
