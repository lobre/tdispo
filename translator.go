package main

import (
	"encoding/csv"
	"errors"
	"io"
	"io/fs"
	"os"
)

type Translator struct {
	index   map[string]string
	csvPath string
}

func NewTranslator(csvPath string) (*Translator, error) {
	tr := Translator{
		csvPath: csvPath,
	}

	tr.index = make(map[string]string)

	if _, err := fs.Stat(fsys, csvPath); os.IsNotExist(err) {
		return &tr, nil
	}

	f, err := fsys.Open(csvPath)
	if err != nil {
		return nil, err
	}

	r := csv.NewReader(f)
	for {
		line, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if len(line) != 2 {
			return nil, errors.New("error reading csv file")
		}

		tr.index[line[0]] = line[1]
	}

	return &tr, nil
}

func (tr *Translator) Translate(input string) string {
	if output, ok := tr.index[input]; ok {
		return output
	}
	return input
}
