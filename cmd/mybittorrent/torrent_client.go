package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
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

type TorrentClient struct {
	peers   map[string]*net.Conn
	torrent *TorrentFile
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
	UNCHOKE    = 1
	INTERESTED = 2
	BITFIELD   = 5
	REQUEST    = 6
	PIECE      = 7
	BLOCK_SIZE = 16 * 1024
)

func NewTorrentClient(torrent *TorrentFile) *TorrentClient {
	return &TorrentClient{
		peers:   make(map[string]*net.Conn),
		torrent: torrent,
	}
}

func (tc *TorrentClient) InitTransfer(peer string) {
	conn := tc.peers[peer]
	bitField(conn)
	writeInterested(conn)
	unchoke(conn)
}

func (tc *TorrentClient) PeerConn(address string) *net.Conn {
	return tc.peers[address]
}

func (tc *TorrentClient) Peers() ([]string, error) {
	tracker, err := getTrackerResponse(tc.torrent.Announce, tc.torrent.Info)

	if err != nil {
		return nil, err
	}

	peerList := make([]string, 0)

	for i := 0; i < len(tracker.Peers); i += 6 {
		peer := tracker.Peers[i : i+6]

		ip := net.IP(peer[0:4])
		port := binary.BigEndian.Uint16([]byte(peer[4:6]))

		fmt.Printf("%s:%v\n", ip, port)
		peerList = append(peerList, fmt.Sprintf("%s:%v", ip, port))
	}

	return peerList, nil
}

func (tc *TorrentClient) Download(peer, filePath string) (io.Reader, error) {
	var fileBuf bytes.Buffer
	for i, _ := range getPieces(&tc.torrent.Info) {

		data, err := tc.DownloadPiece(peer, i, filePath)

		if err != nil {
			return nil, err
		}

		if _, err := fileBuf.Write(data); err != nil {
			return nil, fmt.Errorf("error writing to buffer")
		}
	}

	return bytes.NewReader(fileBuf.Bytes()), nil
}

func (tc *TorrentClient) DownloadPiece(peer string, pieceId int, filePath string) ([]byte, error) {
	conn := tc.peers[peer]

	pieceHash := getPieces(&tc.torrent.Info)[pieceId]

	fmt.Printf("PieceHash for id: %d --> %x\n", pieceId, pieceHash)

	count := 0

	fullBlocks := tc.torrent.PieceLength(pieceId) / BLOCK_SIZE
	lastBlockLength := tc.torrent.PieceLength(pieceId) % BLOCK_SIZE

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
			log.Fatalf("error sending REQUEST message: %v", err)
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
			log.Fatalf("error sending REQUEST message: %v", err)
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

	return buffer.Bytes(), nil
}

func (tc *TorrentClient) SaveToDisk(buffer io.Reader, filePath, identifier string) {
	buf, err := io.ReadAll(buffer)

	if err != nil {
		log.Fatalf("error reading file buffer: %v", err)
	}
	err = os.WriteFile(filePath, buf, os.ModePerm)

	if err != nil {
		log.Fatalf("error writing file: %v", err)
	}

	fmt.Printf("Piece %s downloaded to %s", identifier, filePath)
}

func getPieces(info *MetaInfo) []string {
	pieces := make([]string, len(info.Pieces)/20)

	for i := 0; i < len(info.Pieces)/20; i++ {
		piece := info.Pieces[i*20 : (i*20)+20]
		pieces[i] = piece
	}

	return pieces
}

func createPeerMessage(messageId uint8, payload []byte) []byte {
	messageData := make([]byte, 4+1+len(payload))
	binary.BigEndian.PutUint32(messageData[0:4], uint32(1+len(payload)))
	messageData[4] = messageId

	copy(messageData[5:], payload)

	return messageData
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

func bitField(cnn *net.Conn) {
	waitForMessage(*cnn, BITFIELD)
}

func writeInterested(cnn *net.Conn) {
	(*cnn).Write(createPeerMessage(INTERESTED, []byte{}))

}

func unchoke(cnn *net.Conn) {
	waitForMessage(*cnn, UNCHOKE)
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

func (tc *TorrentClient) Handshake(address string) error {

	var err error

	conn, err := net.Dial("tcp", address)

	if err != nil {
		return err
	}

	tc.peers[address] = &conn

	infoHash := torrentInfoHash(tc.torrent.Info)

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

	return nil
}
