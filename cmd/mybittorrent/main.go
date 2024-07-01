package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

type RequestPayload struct {
	Index uint32
	Begin uint32
	Block uint32
}

const (
	BITFIELD   = 5
	INTERESTED = 2
	UNCHOKE    = 1
	REQUEST    = 6
	PIECE      = 7
	BLOCK_SIZE = 16 * 1024
)

func main() {

	command := os.Args[1]

	if command == "decode" {
		decode(os.Args[2])
	} else if command == "info" {
		info(os.Args[2])
	} else if command == "peers" {
		torrent, err := readTorrent(os.Args[2])
		if err != nil {
			log.Fatalf("could not read file: %v", err)
		}
		peers(torrent)
	} else if command == "handshake" {
		torrent, err := readTorrent(os.Args[2])
		if err != nil {
			log.Fatalf("could not read file: %v", err)
		}

		handshake(torrent, os.Args[3])

	} else if command == "download_piece" {
		torrent, err := readTorrent(os.Args[4])
		if err != nil {
			log.Fatalf("could not read file: %v", err)
		}

		filePath := os.Args[3]
		pieceId, err := strconv.Atoi(os.Args[5])

		if err != nil {
			log.Fatal("invalid piece id: %v", err)
		}

		peerList := peers(torrent)

		conn := handshake(torrent, peerList[0])

		waitForMessage(*conn, BITFIELD)

		// Write interested
		(*conn).Write(createPeerMessage(INTERESTED, []byte{}))

		waitForMessage(*conn, UNCHOKE)

		pieceHash := getPieces(&torrent.Info)[pieceId]

		fmt.Printf("PieceHash for id: %d --> %x\n", pieceId, pieceHash)

		count := 0

		fmt.Println("Torrent length:", torrent.Info.Length)
		fmt.Println("Piece length:", torrent.Info.PieceLength)

		fullBlocks := pieceLength(torrent.Info, pieceId) / BLOCK_SIZE
		lastBlockLength := pieceLength(torrent.Info, pieceId) % BLOCK_SIZE

		byteOffset := 0

		for i := 0; i < int(fullBlocks); i++ {

			payload := RequestPayload{
				Index: uint32(pieceId),
				Begin: uint32(byteOffset),
				Block: uint32(BLOCK_SIZE),
			}

			var buf bytes.Buffer
			binary.Write(&buf, binary.BigEndian, payload)

			_, err := (*conn).Write(createPeerMessage(REQUEST, buf.Bytes()))

			if err != nil {
				log.Fatal("error sending REQUEST message: %v", err)
			}

			count++
			byteOffset += BLOCK_SIZE
		}

		if lastBlockLength > 0 {

			payload := RequestPayload{
				Index: uint32(pieceId),
				Begin: uint32(byteOffset),
				Block: uint32(lastBlockLength),
			}

			var buf bytes.Buffer

			binary.Write(&buf, binary.BigEndian, payload)

			_, err := (*conn).Write(createPeerMessage(REQUEST, buf.Bytes()))

			if err != nil {
				log.Fatal("error sending REQUEST message: %v", err)
			}

			count++

		}

		buffer := new(bytes.Buffer)

		for i := 0; i < count; i++ {
			data := waitForMessage(*conn, PIECE)

			index := binary.BigEndian.Uint32(data[0:4])

			if index != uint32(pieceId) {
				log.Fatalf("error: pieceId does not match: %d", index)
			}

			block := data[8:]

			buffer.Write(block)
		}

		err = os.WriteFile(filePath, buffer.Bytes(), os.ModePerm)
		if err != nil {
			log.Fatalf("error writing file: %v", err)
		}

		fmt.Printf("Piece %d downloaded to %s", pieceId, filePath)

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

func createPeerMessage(messageId uint8, payload []byte) []byte {
	messageData := make([]byte, 4+1+len(payload))
	binary.BigEndian.PutUint32(messageData[0:4], uint32(1+len(payload)))
	messageData[4] = messageId

	copy(messageData[5:], payload)

	return messageData
}

func pieceLength(info MetaInfo, piece int) int64 {
	rest := info.Length - (info.PieceLength * int64(piece))

	if rest >= info.PieceLength {
		return int64(info.PieceLength)
	}

	return rest
}

func getPieces(info *MetaInfo) []string {
	pieces := make([]string, len(info.Pieces)/20)

	for i := 0; i < len(info.Pieces)/20; i++ {
		piece := info.Pieces[i*20 : (i*20)+20]
		pieces[i] = piece
	}

	return pieces
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

func handshake(torrent *TorrentFile, address string) *net.Conn {

	conn, err := net.Dial("tcp", address)

	if err != nil {
		log.Fatalf("could not connect remote address: %q", address)
	}

	infoHash := torrentInfoHash(torrent.Info)

	protoLen := byte(19)
	protoStr := []byte("BitTorrent protocol")
	reserved := make([]byte, 8)

	handshake := append([]byte{protoLen}, protoStr...)
	handshake = append(handshake, reserved...)
	handshake = append(handshake, infoHash[:]...)
	handshake = append(handshake, []byte{0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9}...)

	_, err = conn.Write(handshake)

	if err != nil {
		log.Fatalf("error writing handshake: %v", err)
	}

	buf := make([]byte, 68)
	_, err = conn.Read(buf)

	if err != nil {
		log.Fatalf("error receiving response: %v", err)
	}

	fmt.Printf("Peer ID: %s\n", hex.EncodeToString(buf[48:]))

	return &conn
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

func peers(torrent *TorrentFile) []string {

	tracker, err := getTrackerResponse(torrent.Announce, torrent.Info)

	if err != nil {
		log.Fatal(err)
	}

	peerList := make([]string, 0)

	for i := 0; i < len(tracker.Peers); i += 6 {
		peer := tracker.Peers[i : i+6]

		ip := net.IP(peer[0:4])
		port := binary.BigEndian.Uint16([]byte(peer[4:6]))

		fmt.Printf("%s:%v\n", ip, port)
		peerList = append(peerList, fmt.Sprintf("%s:%v", ip, port))
	}

	return peerList
}

func getTrackerResponse(announceUrl string, info MetaInfo) (*TrackerResponse, error) {
	infoHash := torrentInfoHash(info)

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

func torrentInfoHash(info MetaInfo) [20]byte {
	var buff bytes.Buffer

	err := bencode.Marshal(&buff, info)

	if err != nil {
		log.Fatalf("error marshalling: %v", err)
	}

	return sha1.Sum(buff.Bytes())
}

func waitForMessage(conn net.Conn, message uint8) []byte {

	fmt.Printf("waiting for message: %d\n", message)

	prefix := make([]byte, 4)
	_, err := conn.Read(prefix)

	if err != nil {
		log.Fatal(err)
	}

	messageLength := binary.BigEndian.Uint32(prefix)
	fmt.Printf("messageLength %v\n", messageLength)

	receivedMsgId := make([]byte, 1)

	_, err = conn.Read(receivedMsgId)

	if err != nil {
		log.Fatalf("error reading message")
	}

	var messageId uint8
	binary.Read(bytes.NewReader(receivedMsgId), binary.BigEndian, &messageId)

	payload := make([]byte, messageLength-1)

	size, err := io.ReadFull(conn, payload)

	if err != nil {
		log.Fatalf("error reading payload: %v", err)
	}

	fmt.Printf("Payload: %d, size = %d\n", len(payload), size)

	if messageId == message {
		fmt.Printf("Return for MessageId: %d\n", messageId)
		return payload
	}

	return nil
}
