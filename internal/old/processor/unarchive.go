package processor

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/benthosdev/benthos/v4/internal/component/metrics"
	"github.com/benthosdev/benthos/v4/internal/component/processor"
	"github.com/benthosdev/benthos/v4/internal/docs"
	"github.com/benthosdev/benthos/v4/internal/interop"
	"github.com/benthosdev/benthos/v4/internal/log"
	"github.com/benthosdev/benthos/v4/internal/message"
)

//------------------------------------------------------------------------------

func init() {
	Constructors[TypeUnarchive] = TypeSpec{
		constructor: func(conf Config, mgr interop.Manager, log log.Modular, stats metrics.Type) (processor.V1, error) {
			p, err := newUnarchive(conf.Unarchive, mgr)
			if err != nil {
				return nil, err
			}
			return processor.NewV2ToV1Processor("unarchive", p, mgr.Metrics()), nil
		},
		Categories: []string{
			"Parsing", "Utility",
		},
		Summary: `
Unarchives messages according to the selected archive [format](#formats) into
multiple messages within a [batch](/docs/configuration/batching).`,
		Description: `
When a message is unarchived the new messages replace the original message in
the batch. Messages that are selected but fail to unarchive (invalid format)
will remain unchanged in the message batch but will be flagged as having failed,
allowing you to [error handle them](/docs/configuration/error_handling).

For the unarchive formats that contain file information (tar, zip), a metadata
field is added to each message called ` + "`archive_filename`" + ` with the
extracted filename.`,
		Config: docs.FieldComponent().WithChildren(
			docs.FieldString("format", "The unarchive [format](#formats) to use.").HasOptions(
				"tar", "zip", "binary", "lines", "json_documents", "json_array", "json_map", "csv",
			),
		),
		Footnotes: `
## Formats

### ` + "`tar`" + `

Extract messages from a unix standard tape archive.

### ` + "`zip`" + `

Extract messages from a zip file.

### ` + "`binary`" + `

Extract messages from a binary blob format consisting of:

- Four bytes containing number of messages in the batch (in big endian)
- For each message part:
  + Four bytes containing the length of the message (in big endian)
  + The content of message

### ` + "`lines`" + `

Extract the lines of a message each into their own message.

### ` + "`json_documents`" + `

Attempt to parse a message as a stream of concatenated JSON documents. Each
parsed document is expanded into a new message.

### ` + "`json_array`" + `

Attempt to parse a message as a JSON array, and extract each element into its
own message.

### ` + "`json_map`" + `

Attempt to parse the message as a JSON map and for each element of the map
expands its contents into a new message. A metadata field is added to each
message called ` + "`archive_key`" + ` with the relevant key from the top-level
map.

### ` + "`csv`" + `

Attempt to parse the message as a csv file (header required) and for each row in 
the file expands its contents into a json object in a new message.`,
	}
}

//------------------------------------------------------------------------------

// UnarchiveConfig contains configuration fields for the Unarchive processor.
type UnarchiveConfig struct {
	Format string `json:"format" yaml:"format"`
}

// NewUnarchiveConfig returns a UnarchiveConfig with default values.
func NewUnarchiveConfig() UnarchiveConfig {
	return UnarchiveConfig{
		Format: "",
	}
}

//------------------------------------------------------------------------------

type unarchiveFunc func(part *message.Part) ([]*message.Part, error)

func tarUnarchive(part *message.Part) ([]*message.Part, error) {
	buf := bytes.NewBuffer(part.Get())
	tr := tar.NewReader(buf)

	var newParts []*message.Part

	// Iterate through the files in the archive.
	for {
		h, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return nil, err
		}

		newPartBuf := bytes.Buffer{}
		if _, err = newPartBuf.ReadFrom(tr); err != nil {
			return nil, err
		}

		newPart := part.Copy()
		newPart.Set(newPartBuf.Bytes())
		newPart.MetaSet("archive_filename", h.Name)
		newParts = append(newParts, newPart)
	}

	return newParts, nil
}

func zipUnarchive(part *message.Part) ([]*message.Part, error) {
	buf := bytes.NewReader(part.Get())
	zr, err := zip.NewReader(buf, int64(buf.Len()))
	if err != nil {
		return nil, err
	}

	var newParts []*message.Part

	// Iterate through the files in the archive.
	for _, f := range zr.File {
		fr, err := f.Open()
		if err != nil {
			return nil, err
		}

		newPartBuf := bytes.Buffer{}
		if _, err = newPartBuf.ReadFrom(fr); err != nil {
			return nil, err
		}

		newPart := part.Copy()
		newPart.Set(newPartBuf.Bytes())
		newPart.MetaSet("archive_filename", f.Name)
		newParts = append(newParts, newPart)
	}

	return newParts, nil
}

func binaryUnarchive(part *message.Part) ([]*message.Part, error) {
	msg, err := message.FromBytes(part.Get())
	if err != nil {
		return nil, err
	}
	parts := make([]*message.Part, msg.Len())
	_ = msg.Iter(func(i int, p *message.Part) error {
		newPart := part.Copy()
		newPart.Set(p.Get())
		parts[i] = newPart
		return nil
	})

	return parts, nil
}

func linesUnarchive(part *message.Part) ([]*message.Part, error) {
	lines := bytes.Split(part.Get(), []byte("\n"))
	parts := make([]*message.Part, len(lines))
	for i, l := range lines {
		newPart := part.Copy()
		newPart.Set(l)
		parts[i] = newPart
	}
	return parts, nil
}

func jsonDocumentsUnarchive(part *message.Part) ([]*message.Part, error) {
	var parts []*message.Part
	dec := json.NewDecoder(bytes.NewReader(part.Get()))
	for {
		var m interface{}
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		newPart := part.Copy()
		newPart.SetJSON(m)
		parts = append(parts, newPart)
	}
	return parts, nil
}

func jsonArrayUnarchive(part *message.Part) ([]*message.Part, error) {
	jDoc, err := part.JSON()
	if err != nil {
		return nil, fmt.Errorf("failed to parse message into JSON array: %v", err)
	}

	jArray, ok := jDoc.([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to parse message into JSON array: invalid type '%T'", jDoc)
	}

	parts := make([]*message.Part, len(jArray))
	for i, ele := range jArray {
		newPart := part.Copy()
		newPart.SetJSON(ele)
		parts[i] = newPart
	}
	return parts, nil
}

func jsonMapUnarchive(part *message.Part) ([]*message.Part, error) {
	jDoc, err := part.JSON()
	if err != nil {
		return nil, fmt.Errorf("failed to parse message into JSON map: %v", err)
	}

	jMap, ok := jDoc.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to parse message into JSON map: invalid type '%T'", jDoc)
	}

	parts := make([]*message.Part, len(jMap))
	i := 0
	for key, ele := range jMap {
		newPart := part.Copy()
		newPart.SetJSON(ele)
		newPart.MetaSet("archive_key", key)
		parts[i] = newPart
		i++
	}
	return parts, nil
}

func csvUnarchive(part *message.Part) ([]*message.Part, error) {
	buf := bytes.NewReader(part.Get())

	scanner := csv.NewReader(buf)
	scanner.ReuseRecord = true

	var newParts []*message.Part

	var headers []string

	var err error

	for {
		var records []string
		records, err = scanner.Read()
		if err != nil {
			break
		}

		if headers == nil {
			headers = make([]string, len(records))
			copy(headers, records)
			continue
		}

		if len(records) < len(headers) {
			err = errors.New("row has too few values")
			break
		}

		if len(records) > len(headers) {
			err = errors.New("row has too many values")
			break
		}

		obj := make(map[string]interface{}, len(records))
		for i, r := range records {
			obj[headers[i]] = r
		}

		newPart := part.Copy()
		newPart.SetJSON(obj)
		newParts = append(newParts, newPart)
	}

	if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to parse message as csv: %v", err)
	}

	return newParts, nil
}

func strToUnarchiver(str string) (unarchiveFunc, error) {
	switch str {
	case "tar":
		return tarUnarchive, nil
	case "zip":
		return zipUnarchive, nil
	case "binary":
		return binaryUnarchive, nil
	case "lines":
		return linesUnarchive, nil
	case "json_documents":
		return jsonDocumentsUnarchive, nil
	case "json_array":
		return jsonArrayUnarchive, nil
	case "json_map":
		return jsonMapUnarchive, nil
	case "csv":
		return csvUnarchive, nil
	}
	return nil, fmt.Errorf("archive format not recognised: %v", str)
}

//------------------------------------------------------------------------------

type unarchiveProc struct {
	unarchive unarchiveFunc
	log       log.Modular
}

func newUnarchive(conf UnarchiveConfig, mgr interop.Manager) (*unarchiveProc, error) {
	dcor, err := strToUnarchiver(conf.Format)
	if err != nil {
		return nil, err
	}
	return &unarchiveProc{
		unarchive: dcor,
		log:       mgr.Logger(),
	}, nil
}

func (d *unarchiveProc) Process(ctx context.Context, msg *message.Part) ([]*message.Part, error) {
	newParts, err := d.unarchive(msg)
	if err != nil {
		d.log.Errorf("Failed to unarchive message part: %v\n", err)
		return nil, err
	}
	return newParts, nil
}

func (d *unarchiveProc) Close(context.Context) error {
	return nil
}
