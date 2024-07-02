package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

func SaveToDisk(buffer io.Reader, filePath, identifier string) {
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
