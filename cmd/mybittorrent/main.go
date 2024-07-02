package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/jackpal/bencode-go"
)

func main() {

	command := os.Args[1]

	if command == "decode" {
		decode(os.Args[2])
	} else if command == "info" {
		info(os.Args[2])
	} else if command == "peers" {

		torrent, err := NewTorrent(os.Args[2])
		if err != nil {
			log.Fatalf("could not read file: %v", err)
		}
		client := NewTorrentClient(torrent)
		client.Peers()

	} else if command == "handshake" {
		torrent, err := NewTorrent(os.Args[2])
		if err != nil {
			log.Fatalf("could not read file: %v", err)
		}

		client := NewTorrentClient(torrent)
		client.Handshake(torrent, os.Args[3])

	} else if command == "download_piece" {

		filePath := os.Args[3]
		torrentFile := os.Args[4]
		pieceId, err := strconv.Atoi(os.Args[5])

		if err != nil {
			log.Fatalf("invalid piece id: %v", err)
		}

		client, peers, err := createClient(torrentFile)

		if err != nil {
			log.Fatalf("error creating client: %v", err)
		}

		pieceData, err := client.DownloadPiece(peers[0], pieceId, filePath)

		if err != nil {
			log.Fatalf("Error downloading piece %d : %v", pieceId, err)
		}

		SaveToDisk(bytes.NewReader(pieceData), filePath, string(pieceId))

	} else if command == "download" {
		filePath := os.Args[3]
		torrentFile := os.Args[4]

		client, peers, err := createClient(torrentFile)

		if err != nil {
			log.Fatalf("handshake error: %v", err)
		}

		fileReader, err := client.Download(peers[0], filePath)

		if err != nil {
			log.Fatalf("error downloading file: %v", err)
		}

		SaveToDisk(fileReader, filePath, torrentFile)

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

	torrent, err := readTorrent(torrentPath)

	if err != nil {
		log.Fatalf("error reading torrent: %v", err)
	}

	if err := parseTorrent(torrent); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

func parseTorrent(torrent *TorrentFile) error {

	tracker := torrent.Announce

	h := sha1.New()

	err := bencode.Marshal(h, torrent.Info)

	if err != nil {
		return fmt.Errorf("error marshaling meta info")
	}

	infoHash := h.Sum(nil)

	hash := fmt.Sprintf("%x\n", infoHash)

	fmt.Println("Tracker URL:", tracker)
	fmt.Println("Length:", torrent.Info.Length)
	fmt.Println("Info Hash:", hash)
	fmt.Println("Piece Length:", torrent.Info.PieceLength)
	fmt.Println("Piece Hashes:")

	for i := 0; i < len(torrent.Info.Pieces); i += 20 {
		fmt.Printf("%x\n", torrent.Info.Pieces[i:i+20])
	}

	return nil
}

func createClient(torrentPath string) (client *TorrentClient, peers []string, err error) {

	torrent, err := NewTorrent(torrentPath)

	if err != nil {
		log.Fatalf("could not read file: %v", err)
	}

	client = NewTorrentClient(torrent)

	peers, err = client.Peers()

	if err != nil {
		return nil, nil, err
	}

	err = client.Handshake(torrent, peers[0])

	if err != nil {
		return nil, nil, err
	}

	client.InitTransfer(peers[0])

	return client, peers, nil
}
