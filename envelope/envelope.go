// The infractructure to deal with echomail message envelope and header.
// Currently rather messy and redundant.
package envelope

import (
	"bufio"
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/pathologize"
)

// Envelope format identifier string.
const MAGIC = "ECHO 1.0"

// Standardized message datetime format. Always assume this is UTC time.
const DateFormat = "2006-01-02 15:04:05"

// Base structure for the message envelope.
type Envelope map[string][]byte

// A more advanced message structure produced by parsing an envelope and header of the message inside.
type EchomailMessage struct {
	MsgId       string             // Derived by hashing the message file.
	Sender      string             // Sender is always there on a valid message.
	Group       string             // Group is always there on a valid message.
	Date        *time.Time         // Message date, if any.
	Header      map[string]string  // Pre-parsed header.
	Message     *[]byte            // The message itself.
	Attachments map[string]*[]byte // File attachments.
}

func getHeader(b []byte) ([]byte, error) {
	const beginsWith = "```Echomail"
	const endsWith = "```"

	if !bytes.HasPrefix(b, []byte(beginsWith)) {
		return nil, fmt.Errorf("message must start with header")
	}

	end := bytes.Index(b[len(beginsWith):], []byte(endsWith))
	if end == -1 {
		return nil, fmt.Errorf("header never ended")
	}

	end += len(beginsWith)

	blockContent := b[len(beginsWith):end]
	return bytes.TrimSpace(blockContent), nil
}

func parseHeader(data []byte) (map[string]string, error) {
	s := string(data)

	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")

	fields := make(map[string]string, len(lines))

	for _, line := range lines {

		if len(line) > 1024 {
			return nil, fmt.Errorf("header line too long")
		}

		line = strings.TrimSpace(line)
		if line == "" {
			return nil, fmt.Errorf("empty line in header")
		}

		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("missing header field separator")
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return nil, fmt.Errorf("empty header field key")
		}

		fields[key] = value
	}

	return fields, nil
}

// Retrieve and parse a message header into a map.
func MessageHeader(msg []byte) (map[string]string, error) {
	hdr, err := getHeader(msg)
	if err != nil {
		return make(map[string]string), err
	}
	fields, err := parseHeader(hdr)
	if err != nil {
		return fields, err
	}
	return fields, nil
}

// Verify an envelope for validity, parsing the message therein.
func (e *Envelope) Verify() error {
	// The envelope must contain an . file with a non-zero length:
	index, ok := (*e)["."]
	if !ok {
		return fmt.Errorf("message file missing")
	}
	if len(index) == 0 {
		return fmt.Errorf("empty message")
	}

	// Message validity checking:
	if !utf8.Valid(index) {
		return fmt.Errorf("message must be valid utf-8")
	}

	fields, err := MessageHeader(index)
	if err != nil {
		return err
	}

	sender, ok := fields["Sender"]
	if !ok {
		return fmt.Errorf("sender missing")
	}
	if len(sender) != 52 || strings.ToUpper(sender) != sender {
		// TODO: I should be able to do a better validity check here
		return fmt.Errorf("sender ID looks bogus")
	}
	group, ok := fields["Group"]
	if !ok || len(group) == 0 {
		return fmt.Errorf("group missing")
	}

	msgDate, ok := fields["Date"]
	if ok {
		_, err := time.ParseInLocation(DateFormat, msgDate, time.UTC)
		if err != nil {
			return fmt.Errorf("datetime parse error: %w", err)
		}
	}

	// Attachment filenames must be filenames.
	for fn := range *e {
		if fn != "." && !pathologize.IsClean(fn) {
			return fmt.Errorf("illegal filename %s", fn)
		}
	}

	return nil
}

// Write an envelope into a byte array.
func (e *Envelope) Write() ([]byte, error) {

	err := e.Verify()
	if err != nil {
		return []byte{}, err
	}

	var header bytes.Buffer
	var body bytes.Buffer

	filenames := make([]string, 0, len(*e))
	for name := range *e {
		filenames = append(filenames, strings.TrimSpace(name))
	}
	sort.Strings(filenames)

	header.WriteString(MAGIC + "\n")

	for _, name := range filenames {
		content := (*e)[name]
		header.WriteString(fmt.Sprintf("%d %s\n", len(content), name))
		body.Write(content)
	}

	header.WriteString("end\n")

	return append(header.Bytes(), body.Bytes()...), nil

}

// Read an envelope from a byte array.
func Read(source []byte) (Envelope, error) {
	envelope := Envelope{}

	reader := bufio.NewReader(bytes.NewReader(source))

	// Read the header line by line until "end"
	var lines []string
	var totalHeaderLen int
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		tLine := strings.TrimRight(line, "\n")

		lines = append(lines, tLine)
		totalHeaderLen += len(line)

		if tLine == "end" {
			break
		}
	}

	if len(lines) == 0 || lines[0] != MAGIC {
		return nil, fmt.Errorf("missing header")
	}

	type containedFile struct {
		name   string
		length int
	}

	var entries []containedFile
	var usedNames []string
	for _, line := range lines[1 : len(lines)-1] {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid file entry: %s", line)
		}
		fn := strings.TrimSpace(parts[1])
		if slices.Contains(usedNames, fn) {
			return nil, fmt.Errorf("duplicate name in envelope: %s", fn)
		}
		length, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid length in entry: %s", line)
		}
		entries = append(entries, containedFile{fn, length})
		usedNames = append(usedNames, fn)
	}

	body := source[totalHeaderLen:]

	offset := 0
	for _, entry := range entries {
		if offset+entry.length > len(body) {
			return nil, fmt.Errorf("not enough data for file: %s", entry.name)
		}
		envelope[entry.name] = body[offset : offset+entry.length]
		offset += entry.length
	}

	err := envelope.Verify()
	if err != nil {
		return Envelope{}, fmt.Errorf("decoding error: %w", err)
	}
	return envelope, nil

}

func computeMsgId(body []byte) string {
	hash := sha512.Sum512_256(body)
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// Parse an envelope, producing a more wieldy message struct.
func (e *Envelope) Parse() (EchomailMessage, error) {
	msg := EchomailMessage{}
	err := e.Verify()
	if err != nil {
		return msg, err
	}
	msg.MsgId = computeMsgId((*e)["."])
	body := (*e)["."]
	msg.Message = &body

	msg.Attachments = make(map[string]*[]byte, len(*e)-1)
	for fn, fd := range *e {
		if fn != "." {
			msg.Attachments[fn] = &fd
		}
	}

	hdr, _ := getHeader(body)
	msg.Header, _ = parseHeader(hdr)
	if dateString, ok := msg.Header["Date"]; ok {
		date, _ := time.ParseInLocation(DateFormat, dateString, time.UTC)
		msg.Date = &date
	}
	msg.Group = msg.Header["Group"]
	msg.Sender = msg.Header["Sender"]

	return msg, nil
}

// Generates a directory name from a group name.
// It is based on a hash to allow using any symbols as part of the group name.
func GroupDir(groupName string) string {
	hash := sha512.Sum512_256([]byte(strings.TrimSpace(groupName)))
	return base64.RawURLEncoding.EncodeToString(hash[:]) + ".group"
}
