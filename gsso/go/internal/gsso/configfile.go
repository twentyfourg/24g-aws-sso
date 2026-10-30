package gsso

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// This file is a line-oriented INI reader/writer that preserves comments and
// formatting. The stdlib has nothing equivalent: every Go INI library
// reformats on write, and the managed marker and account name here are
// comments, which those libraries drop.

var sectionRE = regexp.MustCompile(`^\s*\[(.+?)\]\s*$`)

// Block is one section of an INI file: an optional header ("profile foo")
// and every line belonging to it, including the header line itself.
type Block struct {
	Header *string
	Lines  []string
}

// Body returns the lines after the header, or every line for the preamble
// block (Header == nil).
func (b *Block) Body() []string {
	if b.Header == nil {
		return b.Lines
	}
	if len(b.Lines) <= 1 {
		return nil
	}
	return b.Lines[1:]
}

// HasComment reports whether any body line starts with prefix.
func (b *Block) HasComment(prefix string) bool {
	for _, line := range b.Body() {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return true
		}
	}
	return false
}

// CommentValue returns the text after prefix on the first matching comment
// line, or "" if none matches.
func (b *Block) CommentValue(prefix string) (string, bool) {
	for _, line := range b.Body() {
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, prefix) {
			return strings.TrimSpace(stripped[len(prefix):]), true
		}
	}
	return "", false
}

// Get returns the value of key, skipping comment lines.
func (b *Block) Get(key string) (string, bool) {
	for _, line := range b.Body() {
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "#") || strings.HasPrefix(stripped, ";") {
			continue
		}
		name, value, found := strings.Cut(stripped, "=")
		if found && strings.TrimSpace(name) == key {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}

// splitKeepEnds splits text into lines, keeping the trailing newline on each
// line.
func splitKeepEnds(text string) []string {
	if text == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, text[start:i+1])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

// ConfigFile is a full INI file: an ordered list of blocks (the unheaded
// preamble, if any, followed by one block per [section]).
type ConfigFile struct {
	Path   string
	Blocks []*Block
}

// LoadConfigFile reads path. A missing file yields an empty ConfigFile, not
// an error, since callers create sections.
func LoadConfigFile(path string) (*ConfigFile, error) {
	cf := &ConfigFile{Path: path}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cf, nil
	}
	if err != nil {
		return nil, err
	}
	current := &Block{}
	cf.Blocks = append(cf.Blocks, current)
	for _, line := range splitKeepEnds(string(data)) {
		if m := sectionRE.FindStringSubmatch(line); m != nil {
			header := strings.TrimSpace(m[1])
			current = &Block{Header: &header, Lines: []string{line}}
			cf.Blocks = append(cf.Blocks, current)
		} else {
			current.Lines = append(current.Lines, line)
		}
	}
	return cf, nil
}

// Sections returns every block with a header, in file order.
func (c *ConfigFile) Sections() []*Block {
	var out []*Block
	for _, b := range c.Blocks {
		if b.Header != nil {
			out = append(out, b)
		}
	}
	return out
}

// Section returns the block for an exact header match, or nil.
func (c *ConfigFile) Section(header string) *Block {
	for _, b := range c.Sections() {
		if *b.Header == header {
			return b
		}
	}
	return nil
}

// AppendSection appends a rendered section, inserting a blank line before it
// if the previous block does not already end in one.
func (c *ConfigFile) AppendSection(lines []string) (*Block, error) {
	if len(c.Blocks) > 0 {
		previous := c.Blocks[len(c.Blocks)-1]
		if n := len(previous.Lines); n > 0 && strings.TrimSpace(previous.Lines[n-1]) != "" {
			previous.Lines = append(previous.Lines, "\n")
		}
	}
	if len(lines) == 0 {
		return nil, NewError("not a section header: (empty)")
	}
	m := sectionRE.FindStringSubmatch(lines[0])
	if m == nil {
		return nil, NewError("not a section header: %q", lines[0])
	}
	header := strings.TrimSpace(m[1])
	block := &Block{Header: &header, Lines: append([]string(nil), lines...)}
	c.Blocks = append(c.Blocks, block)
	return block, nil
}

// RemoveSection deletes the first block with an exact header match.
func (c *ConfigFile) RemoveSection(header string) bool {
	for i, b := range c.Blocks {
		if b.Header != nil && *b.Header == header {
			c.Blocks = append(c.Blocks[:i], c.Blocks[i+1:]...)
			return true
		}
	}
	return false
}

// Render concatenates every line in every block back into file text.
func (c *ConfigFile) Render() string {
	var sb strings.Builder
	for _, b := range c.Blocks {
		for _, line := range b.Lines {
			sb.WriteString(line)
		}
	}
	return sb.String()
}

// Save atomically writes the rendered file back to Path.
func (c *ConfigFile) Save() error {
	return AtomicWrite(c.Path, c.Render())
}

// AtomicWrite writes text to path via a temp file + rename, so a crash never
// leaves a half-written config. An existing file's permissions are kept; a
// new file gets 0600 since AWS config can hold no secrets here but
// conventionally stays user-only.
func AtomicWrite(path string, text string) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, ".gsso-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			os.Remove(tmpPath)
		}
	}()
	if _, err = tmp.WriteString(text); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmpPath, mode); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
