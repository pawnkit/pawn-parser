package parser

import (
	"fmt"
	"math"
)

// SourceFile contains a logical source file and its stable ID.
type SourceFile struct {
	ID   FileID
	Name string
	Data []byte
}

// FileSet owns logical source files and assigns deterministic IDs in insertion
// order. Logical names can be used instead of host paths in generated output.
type FileSet struct {
	files  []SourceFile
	lastID FileID
}

// Add copies data into the set and returns the newly added source file.
func (s *FileSet) Add(name string, data []byte) SourceFile {
	if s.lastID == math.MaxUint32 {
		panic("parser: too many source files")
	}
	s.lastID++

	file := SourceFile{
		ID:   s.lastID,
		Name: name,
		Data: append([]byte(nil), data...),
	}
	s.files = append(s.files, file)
	return file
}

// File returns the source file with id, if it exists.
func (s *FileSet) File(id FileID) (SourceFile, bool) {
	if id == 0 || int(id) > len(s.files) {
		return SourceFile{}, false
	}
	return s.files[id-1], true
}

// Files returns a copy of the set's source-file list.
func (s *FileSet) Files() []SourceFile {
	return append([]SourceFile(nil), s.files...)
}

// Require returns the source file with id or an error if it does not exist.
func (s *FileSet) Require(id FileID) (SourceFile, error) {
	file, ok := s.File(id)
	if !ok {
		return SourceFile{}, fmt.Errorf("unknown source file %d", id)
	}
	return file, nil
}
