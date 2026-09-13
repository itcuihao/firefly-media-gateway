package media

import (
	"encoding/binary"
	"os"
	"testing"

	qtfaststart "github.com/qkzsky/go-qt-faststart"
)

func makeBox(boxType string, payload []byte) []byte {
	size := uint32(8 + len(payload))
	b := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(b[0:4], size)
	copy(b[4:8], []byte(boxType))
	copy(b[8:], payload)
	return b
}

func TestFastStartMP4Streaming(t *testing.T) {
	// Build a valid synthetic MP4 where moov is AFTER mdat (not faststart)
	ftyp := makeBox("ftyp", []byte("isom\x00\x00\x02\x00isommp41"))
	mdatPayload := make([]byte, 1024)
	for i := range mdatPayload {
		mdatPayload[i] = byte(i % 128)
	}
	mdat := makeBox("mdat", mdatPayload)

	// Minimal valid moov box with mvhd box inside
	mvhdPayload := make([]byte, 100)
	mvhd := makeBox("mvhd", mvhdPayload)
	moov := makeBox("moov", mvhd)

	rawMP4 := append(ftyp, mdat...)
	rawMP4 = append(rawMP4, moov...)

	tmp, err := os.CreateTemp("", "test-mp4-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := tmp.Write(rawMP4); err != nil {
		t.Fatalf("write temp: %v", err)
	}

	// Verify before fastStartMP4, FastStart is disabled
	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	f, err := qtfaststart.Read(tmp)
	if err != nil {
		t.Fatalf("qtfaststart read: %v", err)
	}
	if f.FastStartEnabled() {
		t.Fatalf("expected initial MP4 to NOT have faststart enabled")
	}

	// Call fastStartMP4 (which uses streaming io.Copy)
	convertedFile, err := fastStartMP4(tmp)
	if err != nil {
		t.Fatalf("fastStartMP4 failed: %v", err)
	}
	defer os.Remove(convertedFile.Name())
	defer convertedFile.Close()

	if convertedFile == tmp {
		t.Fatalf("expected new converted file, got original file")
	}

	// Verify converted file now has FastStart enabled
	if _, err := convertedFile.Seek(0, 0); err != nil {
		t.Fatalf("seek converted: %v", err)
	}
	f2, err := qtfaststart.Read(convertedFile)
	if err != nil {
		t.Fatalf("qtfaststart read converted: %v", err)
	}
	if !f2.FastStartEnabled() {
		t.Fatalf("expected converted file to have faststart enabled")
	}
}
