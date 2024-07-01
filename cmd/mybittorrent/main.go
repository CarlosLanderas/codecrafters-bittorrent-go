package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"

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

type TrackerResponse struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

type Peer struct {
	Ip   net.IP
	Port uint16
}

func main() {

	command := os.Args[1]

	if command == "decode" {
		decode(os.Args[2])
	} else if command == "info" {
		info(os.Args[2])
	} else if command == "peers" {
		peers(os.Args[2])
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

func readTorrent(torrentPath string) (*TorrentFile, error) {
	b, err := os.ReadFile(torrentPath)

	if err != nil {
		return nil, fmt.Errorf("could not read file %v", torrentPath)
	}

	torrent := TorrentFile{}

	err = bencode.Unmarshal(bytes.NewBuffer([]byte(b)), &torrent)

	if err != nil {
		return nil, fmt.Errorf("error unmarshaling torrent content")
	}

	return &torrent, nil

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

func peers(torrentPath string) {
	torrent, err := readTorrent(torrentPath)

	if err != nil {
		log.Fatalf("error reading torrent: %v", torrentPath)
	}

	tracker, err := getTrackerResponse(torrent.Announce, torrent.Info)

	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < len(tracker.Peers); i += 6 {
		peer := tracker.Peers[i : i+6]

		ip := net.IP(peer[0:4])
		port := binary.BigEndian.Uint16([]byte(peer[4:6]))

		fmt.Printf("%s:%v\n", ip, port)

	}
}

func getTrackerResponse(announceUrl string, info MetaInfo) (*TrackerResponse, error) {
	var buff bytes.Buffer

	err := bencode.Marshal(&buff, info)

	if err != nil {
		log.Fatalf("error marshalling: %v", err)
	}
	infoHash := sha1.Sum(buff.Bytes())

	length := strconv.Itoa(int(info.Length))

	values := url.Values{}
	values.Add("info_hash", string(infoHash[:]))
	values.Add("peer_id", "00112233445566778899")
	values.Add("port", "6881")
	values.Add("uploaded", "0")
	values.Add("downloaded", "0")
	values.Add("left", length)
	values.Add("compact", "1")

	reqUrl := fmt.Sprintf("%s?%s", announceUrl, values.Encode())

	resp, err := http.Get(reqUrl)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	defer resp.Body.Close()

	tracker := new(TrackerResponse)

	bencode.Unmarshal(resp.Body, &tracker)

	return tracker, nil
}
