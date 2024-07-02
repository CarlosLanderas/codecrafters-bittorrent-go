package main

import (
	"bytes"
	"crypto/sha1"
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

func NewTorrent(torrentPath string) (*TorrentFile, error) {
	torrent, err := readTorrent(torrentPath)

	if err != nil {
		return nil, err
	}

	return torrent, nil
}

func (tf *TorrentFile) PieceLength(pieceId int) int64 {

	rest := tf.Info.Length - (tf.Info.PieceLength * int64(pieceId))

	if rest >= tf.Info.PieceLength {
		return int64(tf.Info.PieceLength)
	}

	return rest
}

func (tf *TorrentFile) InfoHash() ([20]byte, error) {

	var buff bytes.Buffer

	err := bencode.Marshal(&buff, tf.Info)

	if err != nil {
		return [20]byte{}, err
	}

	return sha1.Sum(buff.Bytes()), nil
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
