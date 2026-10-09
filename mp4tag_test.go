package mp4tag

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func makeBox(name string, payload []byte) []byte {
	size := len(payload) + 8
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(size))
	buf = append(buf, []byte(name)...)
	buf = append(buf, payload...)
	return buf
}

func createTestMP4(t *testing.T) string {
	t.Helper()

	// ftyp
	ftypPayload := append([]byte("M4A "), []byte{0x00, 0x00, 0x00, 0x00}...)
	ftypPayload = append(ftypPayload, []byte("M4A ")...)
	ftyp := makeBox("ftyp", ftypPayload)

	// stco: 4 bytes ver/flags + 4 bytes count (0)
	stco := makeBox("stco", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	stbl := makeBox("stbl", stco)
	minf := makeBox("minf", stbl)
	mdia := makeBox("mdia", minf)
	trak := makeBox("trak", mdia)

	// ilst inside meta inside udta
	ilst := makeBox("ilst", nil)
	meta := makeBox("meta", append([]byte{0x00, 0x00, 0x00, 0x00}, ilst...))
	udta := makeBox("udta", meta)

	// moov
	moov := makeBox("moov", append(trak, udta...))

	// mdat
	mdat := makeBox("mdat", []byte{0x00, 0x00, 0x00, 0x00})

	fullData := append(ftyp, append(moov, mdat...)...)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.m4a")
	err := os.WriteFile(filePath, fullData, 0644)
	if err != nil {
		t.Fatalf("failed to write test MP4 file: %v", err)
	}
	return filePath
}

func TestOverwriteTagsCustom(t *testing.T) {
	mergedTags := &MP4Tags{
		Custom: map[string][]string{
			"EXISTING": {"val1", "val2"},
			"OLD":      {"remove_me"},
		},
	}

	tags := &MP4Tags{
		Custom: map[string][]string{
			"ARTIST":   {"Cure, The", "CHVRCHES"},
			"existing": {"new_val"},
		},
	}

	delStrings := []string{"custom:old"}

	result := overwriteTags(mergedTags, tags, delStrings)

	if _, ok := result.Custom["OLD"]; ok {
		t.Errorf("expected 'OLD' to be deleted, but found in result")
	}

	expectedArtist := []string{"Cure, The", "CHVRCHES"}
	if !reflect.DeepEqual(result.Custom["ARTIST"], expectedArtist) {
		t.Errorf("expected ARTIST = %v, got %v", expectedArtist, result.Custom["ARTIST"])
	}

	expectedExisting := []string{"new_val"}
	if !reflect.DeepEqual(result.Custom["existing"], expectedExisting) {
		t.Errorf("expected existing = %v, got %v", expectedExisting, result.Custom["existing"])
	}

	// Test allcustom deletion
	resultAllCustom := overwriteTags(result, &MP4Tags{}, []string{"allcustom"})
	if len(resultAllCustom.Custom) != 0 {
		t.Errorf("expected all custom tags deleted, got: %v", resultAllCustom.Custom)
	}
}

func TestCustomTagsReadWrite(t *testing.T) {
	filePath := createTestMP4(t)

	mp4, err := Open(filePath)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer mp4.Close()

	// Initial read: custom should be empty/nil
	tags, err := mp4.Read()
	if err != nil {
		t.Fatalf("failed to read tags: %v", err)
	}
	if len(tags.Custom) != 0 {
		t.Errorf("expected empty custom tags initially, got %v", tags.Custom)
	}

	// Write custom tags: one single-value, one multi-value
	writeTags := &MP4Tags{
		Custom: map[string][]string{
			"ARTIST": {"Cure, The", "CHVRCHES"},
			"LABEL":  {"Fiction Records"},
		},
	}

	err = mp4.Write(writeTags, nil)
	if err != nil {
		t.Fatalf("failed to write custom tags: %v", err)
	}

	// Read back and verify
	readTags, err := mp4.Read()
	if err != nil {
		t.Fatalf("failed to read back custom tags: %v", err)
	}

	expectedArtist := []string{"Cure, The", "CHVRCHES"}
	if !reflect.DeepEqual(readTags.Custom["ARTIST"], expectedArtist) {
		t.Errorf("expected ARTIST %v, got %v", expectedArtist, readTags.Custom["ARTIST"])
	}

	expectedLabel := []string{"Fiction Records"}
	if !reflect.DeepEqual(readTags.Custom["LABEL"], expectedLabel) {
		t.Errorf("expected LABEL %v, got %v", expectedLabel, readTags.Custom["LABEL"])
	}

	// Delete specific custom tag
	err = mp4.Write(&MP4Tags{}, []string{"custom:label"})
	if err != nil {
		t.Fatalf("failed to delete label custom tag: %v", err)
	}

	readTags2, err := mp4.Read()
	if err != nil {
		t.Fatalf("failed to read tags after delete: %v", err)
	}
	if _, ok := readTags2.Custom["LABEL"]; ok {
		t.Errorf("expected LABEL to be deleted, but still present")
	}
	if !reflect.DeepEqual(readTags2.Custom["ARTIST"], expectedArtist) {
		t.Errorf("expected ARTIST to still be present %v, got %v", expectedArtist, readTags2.Custom["ARTIST"])
	}

	// Delete all custom tags
	err = mp4.Write(&MP4Tags{}, []string{"allcustom"})
	if err != nil {
		t.Fatalf("failed to delete allcustom: %v", err)
	}

	readTags3, err := mp4.Read()
	if err != nil {
		t.Fatalf("failed to read tags after allcustom: %v", err)
	}
	if len(readTags3.Custom) != 0 {
		t.Errorf("expected no custom tags after allcustom, got %v", readTags3.Custom)
	}
}

func TestUpperCustom(t *testing.T) {
	filePath := createTestMP4(t)

	mp4, err := Open(filePath)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer mp4.Close()

	mp4.UpperCustom(false)

	writeTags := &MP4Tags{
		Custom: map[string][]string{
			"my_lowercase_tag": {"val1", "val2"},
		},
	}

	err = mp4.Write(writeTags, nil)
	if err != nil {
		t.Fatalf("failed to write custom tag with UpperCustom(false): %v", err)
	}

	readTags, err := mp4.Read()
	if err != nil {
		t.Fatalf("failed to read custom tags: %v", err)
	}

	if !reflect.DeepEqual(readTags.Custom["my_lowercase_tag"], []string{"val1", "val2"}) {
		t.Errorf("expected tag name 'my_lowercase_tag' preserved, got: %v", readTags.Custom)
	}
}
