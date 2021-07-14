package docconv

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/golang/protobuf/proto"

	TSP "code.sajari.com/docconv/iWork"
	"code.sajari.com/docconv/snappy"
)

// ConvertPages converts a Pages file to text.
func ConvertPages(r io.Reader) (string, map[string]string, error) {
	meta := make(map[string]string)
	var textBody string

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return "", nil, fmt.Errorf("error reading data: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return "", nil, fmt.Errorf("error unzipping data: %v", err)
	}

	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "Preview.pdf") {
			// There is a preview PDF version we can use
			if rc, err := f.Open(); err == nil {
				return ConvertPDF(rc)
			}
		}
		if f.Name == "index.xml" {
			// There's an XML version we can use
			if rc, err := f.Open(); err == nil {
				return ConvertXML(rc)
			}
		}
		if f.Name == "Index/Document.iwa" {
			rc, _ := f.Open()
			defer rc.Close()
			bReader := bufio.NewReader(snappy.NewReader(io.MultiReader(strings.NewReader("\xff\x06\x00\x00sNaPpY"), rc)))

			// Ignore error.
			// NOTE: This error was unchecked. Need to revisit this to see if it
			// should be acted on.
			archiveLength, _ := binary.ReadVarint(bReader)

			// Ignore error.
			// NOTE: This error was unchecked. Need to revisit this to see if it
			// should be acted on.
			archiveInfoData, _ := ioutil.ReadAll(io.LimitReader(bReader, archiveLength))

			archiveInfo := &TSP.ArchiveInfo{}
			err = proto.Unmarshal(archiveInfoData, archiveInfo)
			fmt.Println("archiveInfo:", archiveInfo, err)
		}
	}

	return textBody, meta, nil
}
